package websocket

import (
	"connect4-backend/game"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

type Hub struct {
	clients     map[*Client]bool
	gameClients map[string][]*Client
	register    chan *Client
	unregister  chan *Client
	broadcast   chan []byte
	gameManager *game.Manager
	mutex       sync.RWMutex
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	username string
	gameID   string
}

type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func NewHub(gameManager *game.Manager) *Hub {
	hub := &Hub{
		clients:     make(map[*Client]bool),
		gameClients: make(map[string][]*Client),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcast:   make(chan []byte, 256),
		gameManager: gameManager,
	}
	
	// Set callback for game updates
	gameManager.SetGameUpdateCallback(hub.onGameUpdate)
	
	return hub
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			log.Printf("Client connected: %s", client.username)

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				
				// Remove from game clients
				if client.gameID != "" {
					h.removeClientFromGame(client)
				}
			}
			h.mutex.Unlock()
			log.Printf("Client disconnected: %s", client.username)

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			log.Printf("JSON unmarshal error: %v", err)
			continue
		}

		c.handleMessage(msg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg Message) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Error handling message: %v", r)
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Invalid message format"},
			})
		}
	}()

	switch msg.Type {
	case "join_game":
		data, ok := msg.Data.(map[string]interface{})
		if !ok {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Invalid data format"},
			})
			return
		}
		
		usernameInterface, exists := data["username"]
		if !exists || usernameInterface == nil {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Username is required"},
			})
			return
		}
		
		username, ok := usernameInterface.(string)
		if !ok || len(strings.TrimSpace(username)) == 0 {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Valid username is required"},
			})
			return
		}
		
		c.username = strings.TrimSpace(username)
		
		// Check if gameId is provided for joining specific game
		if gameIDInterface, exists := data["gameId"]; exists && gameIDInterface != nil {
			gameID, ok := gameIDInterface.(string)
			if ok && strings.TrimSpace(gameID) != "" {
				c.joinSpecificGame(c.username, strings.TrimSpace(gameID))
				return
			}
		}
		
		c.joinGame(c.username)

	case "make_move":
		data, ok := msg.Data.(map[string]interface{})
		if !ok {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Invalid data format"},
			})
			return
		}
		
		columnInterface, exists := data["column"]
		if !exists {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Column is required"},
			})
			return
		}
		
		columnFloat, ok := columnInterface.(float64)
		if !ok {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Invalid column format"},
			})
			return
		}
		
		column := int(columnFloat)
		if column < 0 || column >= 7 {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Column must be between 0 and 6"},
			})
			return
		}
		
		c.makeMove(column)

	case "reconnect":
		data, ok := msg.Data.(map[string]interface{})
		if !ok {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Invalid data format"},
			})
			return
		}
		
		gameIDInterface, exists := data["gameId"]
		if !exists || gameIDInterface == nil {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Game ID is required"},
			})
			return
		}
		
		usernameInterface, exists := data["username"]
		if !exists || usernameInterface == nil {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Username is required"},
			})
			return
		}
		
		gameID, ok := gameIDInterface.(string)
		if !ok || strings.TrimSpace(gameID) == "" {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Valid game ID is required"},
			})
			return
		}
		
		username, ok := usernameInterface.(string)
		if !ok || strings.TrimSpace(username) == "" {
			c.sendMessage(Message{
				Type: "error",
				Data: map[string]string{"message": "Valid username is required"},
			})
			return
		}
		
		c.username = strings.TrimSpace(username)
		c.reconnectToGame(strings.TrimSpace(gameID), c.username)
		
	default:
		c.sendMessage(Message{
			Type: "error",
			Data: map[string]string{"message": "Unknown message type"},
		})
	}
}

func (c *Client) joinGame(username string) {
	gameObj, player, isWaiting := c.hub.gameManager.FindOrCreateGame(username)
	c.gameID = gameObj.ID

	c.hub.mutex.Lock()
	c.hub.gameClients[gameObj.ID] = append(c.hub.gameClients[gameObj.ID], c)
	c.hub.mutex.Unlock()

	response := Message{
		Type: "game_joined",
		Data: map[string]interface{}{
			"game":      gameObj,
			"player":    player,
			"isWaiting": isWaiting,
		},
	}

	c.sendMessage(response)

	// Always broadcast game state to all clients
	messageType := "game_updated"
	if gameObj.Status == "playing" {
		messageType = "game_started"
	}
	c.broadcastToGame(gameObj.ID, Message{
		Type: messageType,
		Data: gameObj,
	})
}

func (c *Client) joinSpecificGame(username, gameID string) {
	gameObj, player, err := c.hub.gameManager.JoinSpecificGame(username, gameID)
	if err != nil {
		c.sendMessage(Message{
			Type: "error",
			Data: map[string]string{"message": err.Error()},
		})
		return
	}

	c.gameID = gameObj.ID

	c.hub.mutex.Lock()
	c.hub.gameClients[gameObj.ID] = append(c.hub.gameClients[gameObj.ID], c)
	c.hub.mutex.Unlock()

	response := Message{
		Type: "game_joined",
		Data: map[string]interface{}{
			"game":      gameObj,
			"player":    player,
			"isWaiting": gameObj.Status == "waiting",
		},
	}

	c.sendMessage(response)

	// Always broadcast the updated game state to all clients in this game
	messageType := "game_updated"
	if gameObj.Status == "playing" {
		messageType = "game_started"
	}
	
	c.broadcastToGame(gameObj.ID, Message{
		Type: messageType,
		Data: gameObj,
	})
}

func (c *Client) makeMove(column int) {
	if c.gameID == "" {
		return
	}

	move, gameObj, err := c.hub.gameManager.MakeMove(c.gameID, column, c.username)
	if err != nil {
		c.sendMessage(Message{
			Type: "error",
			Data: map[string]string{"message": err.Error()},
		})
		return
	}

	// Broadcast move to all game clients
	c.broadcastToGame(c.gameID, Message{
		Type: "move_made",
		Data: map[string]interface{}{
			"move": move,
			"game": gameObj,
		},
	})

	// If it's bot's turn, make bot move
	if gameObj.IsBot && gameObj.CurrentTurn == game.PLAYER2 && gameObj.Status == "playing" {
		go func() {
			time.Sleep(500 * time.Millisecond) // Small delay for better UX
			
			botMove, updatedGame, err := c.hub.gameManager.MakeBotMove(c.gameID)
			if err != nil {
				log.Printf("Bot move error: %v", err)
				return
			}

			if botMove != nil {
				c.broadcastToGame(c.gameID, Message{
					Type: "move_made",
					Data: map[string]interface{}{
						"move": botMove,
						"game": updatedGame,
					},
				})
			}
		}()
	}
}

func (c *Client) reconnectToGame(gameID, username string) {
	gameObj, exists := c.hub.gameManager.GetGame(gameID)
	if !exists {
		c.sendMessage(Message{
			Type: "error",
			Data: map[string]string{"message": "Game not found"},
		})
		return
	}

	// Verify player belongs to this game
	if gameObj.Player1.Username != username && 
		(gameObj.Player2 == nil || gameObj.Player2.Username != username) {
		c.sendMessage(Message{
			Type: "error",
			Data: map[string]string{"message": "Not authorized for this game"},
		})
		return
	}

	c.gameID = gameID
	c.username = username

	c.hub.mutex.Lock()
	c.hub.gameClients[gameID] = append(c.hub.gameClients[gameID], c)
	c.hub.mutex.Unlock()

	c.sendMessage(Message{
		Type: "game_reconnected",
		Data: gameObj,
	})
}

func (c *Client) sendMessage(msg Message) {
	data, _ := json.Marshal(msg)
	select {
	case c.send <- data:
	default:
		close(c.send)
	}
}

func (c *Client) broadcastToGame(gameID string, msg Message) {
	c.hub.mutex.RLock()
	clients := c.hub.gameClients[gameID]
	c.hub.mutex.RUnlock()

	data, _ := json.Marshal(msg)
	for _, client := range clients {
		select {
		case client.send <- data:
		default:
			close(client.send)
		}
	}
}

func (h *Hub) removeClientFromGame(client *Client) {
	if clients, exists := h.gameClients[client.gameID]; exists {
		for i, c := range clients {
			if c == client {
				h.gameClients[client.gameID] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
	}
}

func (h *Hub) onGameUpdate(gameID string, gameObj *game.Game) {
	h.mutex.RLock()
	clients := h.gameClients[gameID]
	h.mutex.RUnlock()

	messageType := "game_updated"
	if gameObj.Status == "playing" {
		messageType = "game_started"
	}

	msg := Message{
		Type: messageType,
		Data: gameObj,
	}
	
	data, _ := json.Marshal(msg)
	for _, client := range clients {
		select {
		case client.send <- data:
		default:
			close(client.send)
		}
	}
}