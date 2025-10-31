package game

import (
	"connect4-backend/bot"
	"connect4-backend/database"
	"connect4-backend/kafka"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	games         map[string]*Game
	waitingPlayer *Player
	mutex         sync.RWMutex
	db            *database.DB
	kafka         *kafka.Producer
	bot           *bot.Bot
	onGameUpdate  func(gameID string, game *Game)
	leaderboard   map[string]*PlayerStats
	playerWins    map[string]int // Track consecutive wins for difficulty scaling
}

type PlayerStats struct {
	Username     string  `json:"username"`
	Wins         int     `json:"wins"`
	GamesPlayed  int     `json:"gamesPlayed"`
	WinRate      float64 `json:"winRate"`
	BestTime     float64 `json:"bestTime,omitempty"`
	TotalTime    float64 `json:"totalTime"`
}

type LeaderboardEntry struct {
	Username   string `json:"username"`
	Wins       int    `json:"wins"`
	GamesPlayed int   `json:"gamesPlayed"`
	WinRate    float64 `json:"winRate"`
}

func NewManager(db *database.DB, kafkaProducer *kafka.Producer) *Manager {
	manager := &Manager{
		games:       make(map[string]*Game),
		db:          db,
		kafka:       kafkaProducer,
		bot:         bot.NewBot(),
		leaderboard: make(map[string]*PlayerStats),
		playerWins:  make(map[string]int),
	}
	
	// Start cleanup routine for old games
	go manager.cleanupOldGames()
	
	return manager
}

func (m *Manager) SetGameUpdateCallback(callback func(gameID string, game *Game)) {
	m.onGameUpdate = callback
}

func (m *Manager) FindOrCreateGame(username string) (*Game, *Player, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Validate username
	if len(strings.TrimSpace(username)) == 0 {
		return nil, nil, false
	}
	
	username = strings.TrimSpace(username)
	if len(username) > 20 {
		username = username[:20]
	}

	player := &Player{
		ID:       username,
		Username: username,
		IsBot:    false,
	}

	// Check if there's a waiting player (different from current player)
	if m.waitingPlayer != nil && m.waitingPlayer.Username != username {
		// Find the waiting game
		var waitingGame *Game
		for _, game := range m.games {
			if game.Status == "waiting" && game.Player1.Username == m.waitingPlayer.Username {
				waitingGame = game
				break
			}
		}

		if waitingGame != nil {
			// Match found! Add player 2 to the waiting game
			waitingGame.AddPlayer2(player)
			m.waitingPlayer = nil
			
			log.Printf("Matched players: %s vs %s in game %s", 
				waitingGame.Player1.Username, player.Username, waitingGame.ID)
			
			// Send game start event to Kafka
			m.sendKafkaEvent("game_started", map[string]interface{}{
				"gameId":  waitingGame.ID,
				"player1": waitingGame.Player1.Username,
				"player2": waitingGame.Player2.Username,
				"isBot":   false,
			})
			
			// Notify WebSocket clients that game started
			if m.onGameUpdate != nil {
				m.onGameUpdate(waitingGame.ID, waitingGame)
			}
			
			return waitingGame, player, false
		}
	}

	// Check if this player is already waiting (reconnection case)
	if m.waitingPlayer != nil && m.waitingPlayer.Username == username {
		// Find their existing waiting game
		for _, game := range m.games {
			if game.Status == "waiting" && game.Player1.Username == username {
				return game, player, true
			}
		}
		// If we can't find their game, clear the waiting player
		m.waitingPlayer = nil
	}

	// Create new game and wait for opponent
	game := NewGame(player)
	m.games[game.ID] = game
	m.waitingPlayer = player

	log.Printf("Player %s created new game %s and is waiting for opponent", username, game.ID)

	// Start timeout for bot opponent
	go m.startBotTimeout(game.ID, username)

	return game, player, true
}

func (m *Manager) startBotTimeout(gameID, username string) {
	time.Sleep(10 * time.Second)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	game, exists := m.games[gameID]
	if !exists || game.Status != "waiting" {
		return
	}

	// Add bot as player 2
	botPlayer := &Player{
		ID:       "bot",
		Username: "Smart Bot",
		IsBot:    true,
	}

	game.AddPlayer2(botPlayer)
	m.waitingPlayer = nil

	// Send game start event to Kafka
	m.sendKafkaEvent("game_started", map[string]interface{}{
		"gameId":  game.ID,
		"player1": game.Player1.Username,
		"player2": "Bot Luffy",
		"isBot":   true,
	})

	// Notify WebSocket clients
	if m.onGameUpdate != nil {
		m.onGameUpdate(gameID, game)
	}

	log.Printf("Bot joined game %s with player %s", gameID, username)
}

func (m *Manager) MakeMove(gameID string, column int, playerUsername string) (*Move, *Game, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	game, exists := m.games[gameID]
	if !exists {
		return nil, nil, ErrGameNotFound
	}

	// Determine player number
	var playerNum int
	if game.Player1.Username == playerUsername {
		playerNum = PLAYER1
	} else if game.Player2 != nil && game.Player2.Username == playerUsername {
		playerNum = PLAYER2
	} else {
		return nil, nil, ErrPlayerNotFound
	}

	move, err := game.MakeMove(column, playerNum)
	if err != nil {
		return nil, nil, err
	}

	// Send move event to Kafka
	m.sendKafkaEvent("move_made", map[string]interface{}{
		"gameId":   gameID,
		"player":   playerUsername,
		"column":   column,
		"row":      move.Row,
		"isBot":    false,
	})

	// If game finished, save to database
	if game.Status == "finished" {
		m.saveGameResult(game)
		
		m.sendKafkaEvent("game_finished", map[string]interface{}{
			"gameId":   gameID,
			"winner":   game.Winner,
			"duration": time.Since(game.CreatedAt).Seconds(),
		})
	}

	return move, game, nil
}

func (m *Manager) MakeBotMove(gameID string) (*Move, *Game, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	game, exists := m.games[gameID]
	if !exists {
		return nil, nil, ErrGameNotFound
	}

	if !game.IsBot || game.CurrentTurn != PLAYER2 || game.Status != "playing" {
		return nil, game, nil
	}

	// Get player's consecutive wins for difficulty scaling
	playerWins := m.playerWins[game.Player1.Username]
	
	// Get bot move with difficulty scaling
	column := m.bot.GetBestMoveWithDifficulty(game.Board, PLAYER2, playerWins)
	
	move, err := game.MakeMove(column, PLAYER2)
	if err != nil {
		return nil, nil, err
	}

	// Send bot move event to Kafka
	m.sendKafkaEvent("move_made", map[string]interface{}{
		"gameId":   gameID,
		"player":   "Smart Bot",
		"column":   column,
		"row":      move.Row,
		"isBot":    true,
	})

	// If game finished, save to database
	if game.Status == "finished" {
		m.saveGameResult(game)
		
		m.sendKafkaEvent("game_finished", map[string]interface{}{
			"gameId":   gameID,
			"winner":   game.Winner,
			"duration": time.Since(game.CreatedAt).Seconds(),
		})
	}

	return move, game, nil
}

func (m *Manager) JoinSpecificGame(username, gameID string) (*Game, *Player, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	player := &Player{
		ID:       username,
		Username: username,
		IsBot:    false,
	}

	game, exists := m.games[gameID]
	if !exists {
		return nil, nil, ErrGameNotFound
	}

	// Check if game is waiting for a player
	if game.Status != "waiting" {
		return nil, nil, ErrGameNotActive
	}

	// Check if player is already in this game
	if game.Player1.Username == username {
		return game, player, nil
	}

	// Check if game already has 2 players
	if game.Player2 != nil {
		return nil, nil, ErrGameFull
	}

	// Add player 2 to the game
	game.AddPlayer2(player)
	
	// Clear waiting player if this was the waiting game
	if m.waitingPlayer != nil && m.waitingPlayer.Username == game.Player1.Username {
		m.waitingPlayer = nil
	}

	log.Printf("Player %s joined specific game %s with %s", username, gameID, game.Player1.Username)

	// Send game start event to Kafka
	m.sendKafkaEvent("game_started", map[string]interface{}{
		"gameId":  game.ID,
		"player1": game.Player1.Username,
		"player2": game.Player2.Username,
		"isBot":   false,
	})

	// Notify WebSocket clients that game started
	if m.onGameUpdate != nil {
		m.onGameUpdate(game.ID, game)
	}

	return game, player, nil
}

func (m *Manager) GetGame(gameID string) (*Game, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	game, exists := m.games[gameID]
	return game, exists
}

func (m *Manager) saveGameResult(game *Game) {
	duration := time.Since(game.CreatedAt).Seconds()
	
	// Update in-memory leaderboard
	m.updateLeaderboard(game, duration)
	
	// Also save to database if available
	if m.db == nil {
		return
	}

	var winner string
	if game.Winner == PLAYER1 {
		winner = game.Player1.Username
	} else if game.Winner == PLAYER2 {
		winner = game.Player2.Username
	} else {
		winner = "draw"
	}

	_, err := m.db.Exec(`
		INSERT INTO games (id, player1, player2, winner, duration, is_bot, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, game.ID, game.Player1.Username, game.Player2.Username, winner, 
		duration, game.IsBot, game.CreatedAt)

	if err != nil {
		log.Printf("Failed to save game result: %v", err)
	}
}

func (m *Manager) updateLeaderboard(game *Game, duration float64) {
	// Update player 1 stats (always human)
	if stats, exists := m.leaderboard[game.Player1.Username]; exists {
		stats.GamesPlayed++
		stats.TotalTime += duration
		if game.Winner == PLAYER1 {
			stats.Wins++
			// Track consecutive wins for difficulty scaling
			m.playerWins[game.Player1.Username]++
		} else {
			// Reset consecutive wins on loss
			m.playerWins[game.Player1.Username] = 0
		}
		if stats.BestTime == 0 || (game.Winner == PLAYER1 && duration < stats.BestTime) {
			stats.BestTime = duration
		}
		stats.WinRate = float64(stats.Wins) / float64(stats.GamesPlayed) * 100
	} else {
		stats := &PlayerStats{
			Username:    game.Player1.Username,
			GamesPlayed: 1,
			TotalTime:   duration,
		}
		if game.Winner == PLAYER1 {
			stats.Wins = 1
			stats.BestTime = duration
			m.playerWins[game.Player1.Username] = 1
		} else {
			m.playerWins[game.Player1.Username] = 0
		}
		stats.WinRate = float64(stats.Wins) / float64(stats.GamesPlayed) * 100
		m.leaderboard[game.Player1.Username] = stats
	}
	
	// Update player 2 stats (only if human, not bot)
	if game.Player2 != nil && !game.Player2.IsBot {
		if stats, exists := m.leaderboard[game.Player2.Username]; exists {
			stats.GamesPlayed++
			stats.TotalTime += duration
			if game.Winner == PLAYER2 {
				stats.Wins++
				m.playerWins[game.Player2.Username]++
			} else {
				m.playerWins[game.Player2.Username] = 0
			}
			if stats.BestTime == 0 || (game.Winner == PLAYER2 && duration < stats.BestTime) {
				stats.BestTime = duration
			}
			stats.WinRate = float64(stats.Wins) / float64(stats.GamesPlayed) * 100
		} else {
			stats := &PlayerStats{
				Username:    game.Player2.Username,
				GamesPlayed: 1,
				TotalTime:   duration,
			}
			if game.Winner == PLAYER2 {
				stats.Wins = 1
				stats.BestTime = duration
				m.playerWins[game.Player2.Username] = 1
			} else {
				m.playerWins[game.Player2.Username] = 0
			}
			stats.WinRate = float64(stats.Wins) / float64(stats.GamesPlayed) * 100
			m.leaderboard[game.Player2.Username] = stats
		}
	}
}

func (m *Manager) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	// Convert map to slice and sort
	var leaderboard []*PlayerStats
	for _, stats := range m.leaderboard {
		leaderboard = append(leaderboard, stats)
	}
	
	// Sort by best time (ascending) - fastest wins first
	for i := 0; i < len(leaderboard)-1; i++ {
		for j := i + 1; j < len(leaderboard); j++ {
			// Sort by best time (ascending), with 0 times (no wins) at the end
			if (leaderboard[i].BestTime == 0 && leaderboard[j].BestTime > 0) ||
				(leaderboard[i].BestTime > 0 && leaderboard[j].BestTime > 0 && leaderboard[i].BestTime > leaderboard[j].BestTime) {
				leaderboard[i], leaderboard[j] = leaderboard[j], leaderboard[i]
			}
		}
	}
	
	// Limit to top 10
	if len(leaderboard) > 10 {
		leaderboard = leaderboard[:10]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leaderboard)
}

func (m *Manager) GetStats(w http.ResponseWriter, r *http.Request) {
	if m.db == nil {
		// Return basic stats when database is not available
		stats := map[string]interface{}{
			"totalGames":    0,
			"botGames":      0,
			"humanGames":    0,
			"avgDuration":   0,
			"activeGames":   len(m.games),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
		return
	}

	var totalGames, botGames int
	var avgDuration float64

	err := m.db.QueryRow(`
		SELECT 
			COUNT(*) as total_games,
			SUM(CASE WHEN is_bot THEN 1 ELSE 0 END) as bot_games,
			AVG(duration) as avg_duration
		FROM games
	`).Scan(&totalGames, &botGames, &avgDuration)

	if err != nil {
		http.Error(w, "Failed to fetch stats", http.StatusInternalServerError)
		return
	}

	stats := map[string]interface{}{
		"totalGames":    totalGames,
		"botGames":      botGames,
		"humanGames":    totalGames - botGames,
		"avgDuration":   avgDuration,
		"activeGames":   len(m.games),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (m *Manager) cleanupOldGames() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		m.mutex.Lock()
		now := time.Now()
		
		for gameID, game := range m.games {
			// Remove finished games older than 30 minutes
			if game.Status == "finished" && now.Sub(game.LastMove) > 30*time.Minute {
				delete(m.games, gameID)
				log.Printf("Cleaned up finished game: %s", gameID)
			}
			// Remove waiting games older than 15 minutes (abandoned)
			if game.Status == "waiting" && now.Sub(game.CreatedAt) > 15*time.Minute {
				delete(m.games, gameID)
				if m.waitingPlayer != nil && m.waitingPlayer.Username == game.Player1.Username {
					m.waitingPlayer = nil
				}
				log.Printf("Cleaned up abandoned waiting game: %s", gameID)
			}
		}
		
		m.mutex.Unlock()
	}
}

func (m *Manager) sendKafkaEvent(eventType string, data map[string]interface{}) {
	if m.kafka == nil {
		return
	}

	event := map[string]interface{}{
		"type":      eventType,
		"data":      data,
		"timestamp": time.Now().Unix(),
	}

	eventJSON, _ := json.Marshal(event)
	m.kafka.SendMessage("game-events", string(eventJSON))
}