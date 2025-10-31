package main

import (
	"connect4-backend/database"
	"connect4-backend/game"
	"connect4-backend/kafka"
	"connect4-backend/websocket"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	log.Println("Starting 4-in-a-Row Game Server...")

	// Initialize database with retry logic
	var db *database.DB
	var err error
	
	db, err = database.Initialize()
	if err != nil {
		log.Printf("Warning: Database unavailable: %v", err)
		log.Println("Continuing without database (leaderboard will be disabled)")
		db = nil
	} else if db != nil {
		defer db.Close()
		log.Println("Database connected successfully")
	} else {
		log.Println("No database configured (running in simple mode)")
	}

	// Initialize Kafka with retry logic
	var kafkaProducer *kafka.Producer
	
	kafkaProducer, err = kafka.NewProducer()
	if err != nil {
		log.Printf("Warning: Kafka unavailable: %v", err)
		log.Println("Continuing without analytics")
		kafkaProducer = nil
	} else {
		log.Println("Kafka connected successfully")
	}

	// Initialize game manager
	gameManager := game.NewManager(db, kafkaProducer)
	log.Println("Game manager initialized")

	// Initialize WebSocket hub
	hub := websocket.NewHub(gameManager)
	go hub.Run()
	log.Println("WebSocket hub started")

	// Setup routes
	router := mux.NewRouter()
	
	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"services": map[string]bool{
				"database": db != nil,
				"kafka":    kafkaProducer != nil,
				"websocket": true,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	}).Methods("GET")
	
	// CORS middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
			
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	})

	// Logging middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
		})
	})

	// WebSocket endpoint
	router.HandleFunc("/ws", hub.HandleWebSocket).Methods("GET")
	
	// API endpoints
	router.HandleFunc("/api/leaderboard", gameManager.GetLeaderboard).Methods("GET")
	router.HandleFunc("/api/stats", gameManager.GetStats).Methods("GET")

	// Serve the game HTML file - try multiple paths
	gamePaths := []string{
		"../frontend/public/game.html",
		"frontend/public/game.html", 
		"./frontend/public/game.html",
	}
	
	var gameHTML string
	for _, path := range gamePaths {
		if _, err := os.Stat(path); err == nil {
			content, err := os.ReadFile(path)
			if err == nil {
				gameHTML = string(content)
				break
			}
		}
	}
	
	if gameHTML != "" {
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(gameHTML))
		}).Methods("GET")
	} else {
		// Embedded complete game HTML
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>4-in-a-Row Game</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        :root {
            --bg-primary: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            --bg-card: rgba(255, 255, 255, 0.95);
            --text-primary: #333;
            --board-bg: #2c3e50;
        }
        [data-theme="dark"] {
            --bg-primary: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
            --bg-card: rgba(0, 0, 0, 0.4);
            --text-primary: #ffffff;
            --board-bg: #0f1419;
        }
        body {
            font-family: Arial, sans-serif;
            background: var(--bg-primary);
            min-height: 100vh;
            color: var(--text-primary);
            display: flex;
            flex-direction: column;
            align-items: center;
            padding: 20px;
        }
        .header { text-align: center; margin-bottom: 30px; }
        .header h1 { font-size: 2.5rem; color: white; text-shadow: 2px 2px 4px rgba(0,0,0,0.3); }
        .login-form {
            background: var(--bg-card);
            padding: 30px;
            border-radius: 15px;
            text-align: center;
            max-width: 400px;
            width: 100%;
            margin-bottom: 30px;
        }
        .login-form input {
            width: 100%;
            padding: 12px;
            font-size: 16px;
            border: 1px solid #ddd;
            border-radius: 8px;
            margin-bottom: 15px;
            background: rgba(255, 255, 255, 0.95);
            color: #333;
        }
        .login-form button {
            width: 100%;
            padding: 12px;
            font-size: 16px;
            background: #3498db;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
        }
        .game-container { display: flex; gap: 30px; flex-wrap: wrap; justify-content: center; }
        .game-board { background: var(--bg-card); padding: 20px; border-radius: 15px; }
        .board {
            display: grid;
            grid-template-columns: repeat(7, 60px);
            grid-template-rows: repeat(6, 60px);
            gap: 8px;
            background: var(--board-bg);
            padding: 15px;
            border-radius: 10px;
            margin-bottom: 20px;
        }
        .cell {
            width: 60px;
            height: 60px;
            border-radius: 50%;
            border: 2px solid #34495e;
            cursor: pointer;
            transition: all 0.3s;
            background: rgba(255, 255, 255, 0.1);
        }
        .cell:hover { transform: scale(1.05); }
        .cell.player1 { background: #e74c3c; border-color: #c0392b; }
        .cell.player2 { background: #f39c12; border-color: #e67e22; }
        .game-info { background: var(--bg-card); padding: 20px; border-radius: 15px; min-width: 300px; color: var(--text-primary); }
        .player-info { display: flex; justify-content: space-between; padding: 10px; margin: 10px 0; background: rgba(0,0,0,0.1); border-radius: 8px; }
        .status { text-align: center; padding: 15px; margin: 15px 0; border-radius: 8px; font-weight: bold; }
        .status.waiting { background: #fff3cd; color: #856404; }
        .status.playing { background: #d4edda; color: #155724; }
        .status.finished { background: #f8d7da; color: #721c24; }
        .theme-toggle {
            position: fixed;
            top: 20px;
            left: 20px;
            padding: 10px 15px;
            border: none;
            border-radius: 25px;
            background: rgba(255, 255, 255, 0.9);
            color: #333;
            cursor: pointer;
        }
        .connection-status {
            position: fixed;
            top: 10px;
            right: 10px;
            padding: 5px 10px;
            border-radius: 4px;
            font-size: 12px;
        }
        .connection-status.connected { background: #27ae60; color: white; }
        .connection-status.disconnected { background: #e74c3c; color: white; }
        .error { background: #f8d7da; border: 1px solid #f5c6cb; color: #721c24; padding: 10px; border-radius: 4px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="connection-status" id="connectionStatus">Connecting...</div>
    <button class="theme-toggle" id="themeToggle" onclick="toggleTheme()">Dark Mode</button>
    
    <div class="header">
        <h1>4-in-a-Row</h1>
        <p>Connect Four Game</p>
    </div>

    <div id="loginForm" class="login-form">
        <h2>Join Game</h2>
        <input type="text" id="usernameInput" placeholder="Enter your username" maxlength="20">
        <input type="text" id="gameIdInput" placeholder="Game ID (optional)" maxlength="36" style="margin-top: 10px;">
        <button id="joinButton" onclick="joinGame()">Start Playing</button>
        <div id="loginError" class="error" style="display: none;"></div>
    </div>

    <div id="gameContainer" class="game-container" style="display: none;">
        <div class="game-board">
            <div id="gameBoard" class="board"></div>
            <button onclick="resetGame()" style="width: 100%; padding: 10px; background: #6c757d; color: white; border: none; border-radius: 4px; cursor: pointer;">New Game</button>
        </div>

        <div class="game-info">
            <h3>Game Info</h3>
            <div id="gameId" style="font-size: 12px; opacity: 0.7; margin-bottom: 15px;"></div>
            
            <div id="player1Info" class="player-info">
                <span id="player1Name">Player 1</span>
                <span>Player 1</span>
            </div>

            <div id="player2Info" class="player-info">
                <span id="player2Name">Waiting...</span>
                <span>Player 2</span>
            </div>

            <div id="gameStatus" class="status waiting">Waiting for opponent...</div>
            <div id="gameError" class="error" style="display: none;"></div>
        </div>
    </div>

    <script>
        let ws = null;
        let connected = false;
        let game = null;
        let player = null;
        let username = '';

        function connectWebSocket() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws';
            
            ws = new WebSocket(wsUrl);
            
            ws.onopen = function() {
                console.log('WebSocket connected');
                connected = true;
                updateConnectionStatus(true);
            };

            ws.onclose = function() {
                console.log('WebSocket disconnected');
                connected = false;
                updateConnectionStatus(false);
                setTimeout(connectWebSocket, 3000);
            };

            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
                updateConnectionStatus(false);
            };

            ws.onmessage = function(event) {
                const message = JSON.parse(event.data);
                handleMessage(message);
            };
        }

        function updateConnectionStatus(isConnected) {
            const status = document.getElementById('connectionStatus');
            if (isConnected) {
                status.textContent = 'Connected';
                status.className = 'connection-status connected';
            } else {
                status.textContent = 'Disconnected';
                status.className = 'connection-status disconnected';
            }
        }

        function handleMessage(message) {
            console.log('Received:', message);
            
            switch (message.type) {
                case 'game_joined':
                    game = message.data.game;
                    player = message.data.player;
                    showGame();
                    updateGameDisplay();
                    if (message.data.isWaiting) {
                        showStatus('Waiting for opponent... (10s timeout)', 'waiting');
                    }
                    break;
                    
                case 'game_started':
                    game = message.data;
                    updateGameDisplay();
                    showStatus('Game started!', 'playing');
                    break;
                    
                case 'game_updated':
                    game = message.data;
                    updateGameDisplay();
                    if (game.status === 'playing') {
                        showStatus('Game started!', 'playing');
                    }
                    break;
                    
                case 'move_made':
                    game = message.data.game;
                    updateGameDisplay();
                    break;
                    
                case 'error':
                    showError(message.data.message);
                    break;
            }
        }

        function joinGame() {
            const usernameInput = document.getElementById('usernameInput');
            const gameIdInput = document.getElementById('gameIdInput');
            username = usernameInput.value.trim();
            const gameId = gameIdInput.value.trim();
            
            if (!username) {
                showLoginError('Please enter a username');
                return;
            }

            if (!connected) {
                showLoginError('Not connected to server');
                return;
            }

            const data = { username: username };
            if (gameId) {
                data.gameId = gameId;
            }

            ws.send(JSON.stringify({
                type: 'join_game',
                data: data
            }));
        }

        function makeMove(column) {
            if (!game || game.status !== 'playing' || !connected) return;
            
            const playerNum = player.username === game.player1.username ? 1 : 2;
            if (game.currentTurn !== playerNum) return;

            ws.send(JSON.stringify({
                type: 'make_move',
                data: { column: column }
            }));
        }

        function showGame() {
            document.getElementById('loginForm').style.display = 'none';
            document.getElementById('gameContainer').style.display = 'flex';
            createBoard();
        }

        function createBoard() {
            const board = document.getElementById('gameBoard');
            board.innerHTML = '';
            
            for (let row = 0; row < 6; row++) {
                for (let col = 0; col < 7; col++) {
                    const cell = document.createElement('div');
                    cell.className = 'cell empty';
                    cell.onclick = () => makeMove(col);
                    board.appendChild(cell);
                }
            }
        }

        function updateGameDisplay() {
            if (!game) return;

            document.getElementById('gameId').textContent = 'Game ID: ' + game.id;
            document.getElementById('player1Name').textContent = game.player1.username + (player && player.username === game.player1.username ? ' (You)' : '');
            
            if (game.player2) {
                document.getElementById('player2Name').textContent = game.player2.username + (player && player.username === game.player2.username ? ' (You)' : '') + (game.player2.isBot ? ' (Bot)' : '');
            } else {
                document.getElementById('player2Name').textContent = 'Waiting...';
            }

            const cells = document.querySelectorAll('.cell');
            for (let row = 0; row < 6; row++) {
                for (let col = 0; col < 7; col++) {
                    const cell = cells[row * 7 + col];
                    const value = game.board[row][col];
                    
                    cell.className = 'cell';
                    if (value === 0) {
                        cell.className += ' empty';
                    } else if (value === 1) {
                        cell.className += ' player1';
                    } else if (value === 2) {
                        cell.className += ' player2';
                    }
                }
            }

            updateGameStatus();
        }

        function updateGameStatus() {
            if (!game) return;
            
            let statusText = '';
            let statusClass = '';
            
            if (game.status === 'waiting') {
                statusText = 'Waiting for opponent...';
                statusClass = 'waiting';
            } else if (game.status === 'playing') {
                const currentPlayer = game.currentTurn === 1 ? game.player1 : game.player2;
                const isMyTurn = player && currentPlayer.username === player.username;
                statusText = isMyTurn ? "Your turn!" : currentPlayer.username + "'s turn";
                statusClass = 'playing';
            } else if (game.status === 'finished') {
                if (game.winner === 0) {
                    statusText = "It's a draw!";
                } else {
                    const winner = game.winner === 1 ? game.player1 : game.player2;
                    const isWinner = player && winner.username === player.username;
                    statusText = isWinner ? "You won!" : winner.username + " won!";
                }
                statusClass = 'finished';
            }
            
            showStatus(statusText, statusClass);
        }

        function showStatus(text, className) {
            const status = document.getElementById('gameStatus');
            status.textContent = text;
            status.className = 'status ' + className;
        }

        function showError(message) {
            const error = document.getElementById('gameError');
            error.textContent = message;
            error.style.display = 'block';
            setTimeout(() => {
                error.style.display = 'none';
            }, 5000);
        }

        function showLoginError(message) {
            const error = document.getElementById('loginError');
            error.textContent = message;
            error.style.display = 'block';
            setTimeout(() => {
                error.style.display = 'none';
            }, 5000);
        }

        function resetGame() {
            game = null;
            player = null;
            username = '';
            document.getElementById('loginForm').style.display = 'block';
            document.getElementById('gameContainer').style.display = 'none';
            document.getElementById('usernameInput').value = '';
            document.getElementById('gameIdInput').value = '';
        }

        function toggleTheme() {
            const body = document.body;
            const themeToggle = document.getElementById('themeToggle');
            
            if (body.getAttribute('data-theme') === 'dark') {
                body.removeAttribute('data-theme');
                themeToggle.textContent = 'Dark Mode';
            } else {
                body.setAttribute('data-theme', 'dark');
                themeToggle.textContent = 'Light Mode';
            }
        }

        // Initialize
        connectWebSocket();
    </script>
</body>
</html>`
<html>
<head>
    <title>4-in-a-Row Game</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; }
        .container { max-width: 600px; margin: 0 auto; }
        .status { background: rgba(255,255,255,0.1); padding: 20px; border-radius: 10px; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <h1>4-in-a-Row Game Server</h1>
        <div class="status">
            <h2>Server is Running!</h2>
            <p>WebSocket endpoint: <code>ws://localhost:8080/ws</code></p>
            <p>API endpoints available:</p>
            <ul style="text-align: left;">
                <li><a href="/api/stats" style="color: #fff;">GET /api/stats</a></li>
                <li><a href="/api/leaderboard" style="color: #fff;">GET /api/leaderboard</a></li>
                <li><a href="/health" style="color: #fff;">GET /health</a></li>
            </ul>
        </div>
    </div>
</body>
</html>`
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte(html))
			}).Methods("GET")
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on http://localhost:%s", port)
		log.Printf("Game ready at http://localhost:%s", port)
		log.Printf("API available at http://localhost:%s/api/", port)
		log.Printf("WebSocket at ws://localhost:%s/ws", port)
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Println("Shutdown signal received")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	// Close Kafka producer
	if kafkaProducer != nil {
		kafkaProducer.Close()
	}

	log.Println("✅ Server shutdown complete")
}