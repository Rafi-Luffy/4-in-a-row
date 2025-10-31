package database

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
}

func Initialize() (*DB, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// If no DATABASE_URL is set, return nil to indicate no database
		return nil, nil
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection with timeout
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	// Create tables
	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	log.Println("✅ Database initialized successfully")
	return &DB{db}, nil
}

func createTables(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS games (
		id VARCHAR(255) PRIMARY KEY,
		player1 VARCHAR(255) NOT NULL,
		player2 VARCHAR(255) NOT NULL,
		winner VARCHAR(255),
		duration FLOAT,
		is_bot BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_games_winner ON games(winner);
	CREATE INDEX IF NOT EXISTS idx_games_created_at ON games(created_at);
	CREATE INDEX IF NOT EXISTS idx_games_player1 ON games(player1);
	CREATE INDEX IF NOT EXISTS idx_games_player2 ON games(player2);
	`

	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Failed to create tables: %v", err)
		return err
	}

	log.Println("✅ Database tables created/verified")
	return nil
}

func (db *DB) IsHealthy() bool {
	if db == nil || db.DB == nil {
		return false
	}
	return db.Ping() == nil
}