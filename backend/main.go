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

	// Serve the game HTML file
	gamePath := "../frontend/public/game.html"
	if _, err := os.Stat(gamePath); err == nil {
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, gamePath)
		}).Methods("GET")
	} else {
		// Fallback to built React app if available
		buildPath := "../frontend/build"
		if _, err := os.Stat(buildPath); err == nil {
			router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(buildPath, "static")))))
			router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				indexPath := filepath.Join(buildPath, "index.html")
				if _, err := os.Stat(indexPath); os.IsNotExist(err) {
					http.NotFound(w, r)
					return
				}
				http.ServeFile(w, r, indexPath)
			})
		} else {
			// Final fallback to simple status page
			router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				html := `<!DOCTYPE html>
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