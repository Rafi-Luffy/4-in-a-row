package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

type GameEvent struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp int64                  `json:"timestamp"`
}

type Analytics struct {
	reader *kafka.Reader
}

func NewAnalytics() *Analytics {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     strings.Split(brokers, ","),
		Topic:       "game-events",
		GroupID:     "analytics-consumer",
		StartOffset: kafka.LastOffset,
	})

	return &Analytics{reader: reader}
}

func (a *Analytics) Start() {
	log.Println("Analytics consumer started...")
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-c
		log.Println("Shutting down analytics consumer...")
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			message, err := a.reader.ReadMessage(ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				log.Printf("Error reading message: %v", err)
				continue
			}

			a.processMessage(message.Value)
		}
	}
}

func (a *Analytics) processMessage(messageBytes []byte) {
	var event GameEvent
	if err := json.Unmarshal(messageBytes, &event); err != nil {
		log.Printf("Error unmarshaling event: %v", err)
		return
	}

	switch event.Type {
	case "game_started":
		a.handleGameStarted(event)
	case "move_made":
		a.handleMoveMade(event)
	case "game_finished":
		a.handleGameFinished(event)
	default:
		log.Printf("Unknown event type: %s", event.Type)
	}
}

func (a *Analytics) handleGameStarted(event GameEvent) {
	gameID := event.Data["gameId"].(string)
	player1 := event.Data["player1"].(string)
	player2 := event.Data["player2"].(string)
	isBot := event.Data["isBot"].(bool)

	log.Printf("GAME STARTED: %s | %s vs %s | Bot: %v", 
		gameID, player1, player2, isBot)

	// Here you could store to database, send to monitoring systems, etc.
	a.trackMetric("game_started", map[string]interface{}{
		"game_id": gameID,
		"is_bot":  isBot,
		"players": []string{player1, player2},
	})
}

func (a *Analytics) handleMoveMade(event GameEvent) {
	gameID := event.Data["gameId"].(string)
	player := event.Data["player"].(string)
	column := int(event.Data["column"].(float64))
	isBot := event.Data["isBot"].(bool)

	log.Printf("MOVE MADE: %s | %s -> Column %d | Bot: %v", 
		gameID, player, column, isBot)

	a.trackMetric("move_made", map[string]interface{}{
		"game_id": gameID,
		"player":  player,
		"column":  column,
		"is_bot":  isBot,
	})
}

func (a *Analytics) handleGameFinished(event GameEvent) {
	gameID := event.Data["gameId"].(string)
	winner := int(event.Data["winner"].(float64))
	duration := event.Data["duration"].(float64)

	var result string
	if winner == 0 {
		result = "DRAW"
	} else {
		result = "WIN"
	}

	log.Printf("GAME FINISHED: %s | Result: %s | Duration: %.1fs", 
		gameID, result, duration)

	a.trackMetric("game_finished", map[string]interface{}{
		"game_id":  gameID,
		"winner":   winner,
		"duration": duration,
		"result":   result,
	})

	// Calculate and log performance metrics
	a.calculateGameMetrics(duration, winner)
}

func (a *Analytics) trackMetric(eventType string, data map[string]interface{}) {
	// In a real production environment, you would:
	// 1. Store metrics in a time-series database (InfluxDB, Prometheus)
	// 2. Send to monitoring services (DataDog, New Relic)
	// 3. Update dashboards and alerts
	// 4. Calculate real-time analytics

	log.Printf("📈 METRIC: %s | Data: %+v", eventType, data)
}

func (a *Analytics) calculateGameMetrics(duration float64, winner int) {
	// Example analytics calculations
	if duration < 30 {
		log.Printf("QUICK GAME: Duration %.1fs (under 30s)", duration)
	} else if duration > 300 {
		log.Printf("LONG GAME: Duration %.1fs (over 5 minutes)", duration)
	}

	if winner == 0 {
		log.Printf("DRAW GAME: No winner after %.1fs", duration)
	}

	// You could track:
	// - Average game duration by hour/day
	// - Win rates by player
	// - Most popular columns
	// - Bot vs human performance
	// - Peak playing times
	// - Player retention metrics
}

func (a *Analytics) Close() error {
	return a.reader.Close()
}

func main() {
	analytics := NewAnalytics()
	defer analytics.Close()

	log.Println("Starting 4-in-a-Row Analytics Consumer")
	log.Println("Tracking game events and calculating metrics...")
	
	analytics.Start()
}