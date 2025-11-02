# 4-in-a-Row Real-Time Multiplayer Game

A production-ready, competitive real-time multiplayer Connect Four game with intelligent bot, comprehensive analytics, and persistent leaderboard system.

## Live Link : https://four-in-a-row-ex56.onrender.com/

## Key Features

### Core Gameplay
- Real-time multiplayer with WebSocket connections
- Intelligent strategic bot (blocks wins, creates opportunities, center preference)
- Automatic matchmaking with 10-second timeout fallback to bot
- Reconnection support within 30-second window
- Game state persisistence** and rery

### Smart Bot AI
- Winning move detection: Immediately takes winning opportunities
- Threat blocking: Prevents opponent wins
- Strategic positioning: Prefers center columns and creates multiple win paths
- Difficulty scaling: Adapts to create challenging but fair gameplay

### Analytics & Monitoring
- Real-time event streaming via Kafka
- Game metrics tracking: Duration, win rates, player behavior
- Performance monitoring: Active games, response times
- Comprehensive logging for debugging and optimization

### Leaderboard System
- Persistent rankings with PostgreSQL
- Win rate calculations and game statistics
- Real-time updates after each game
- Player perfoformance tracng

## Tech Stack

- Backend: Go 1.19+ with Gorilla WebSocket, PostgreSQL, Kafka
- Frontend: React 18 with modern hooks and WebSocket client
- Database: PostgreSQL with optimized queries and indexing
- Analytics: Kafka with dedicated consumer service
- Infrastructure: Docker Compose for local development

## Project Architecture

```
4-in-a-row/
├── backend/                 # Go backend server
│   ├── main.go             # Server entry point
│   ├── game/               # Game logic and state management
│   ├── bot/                # AI bot implementation
│   ├── websocket/          # Real-time communication
│   ├── database/           # Data persistence
│   └── kafka/              # Event streaming
├── frontend/               # React frontend
│   ├── src/
│   ├── public/
│   └── package.json
├── analytics/              # Kafka analytics consumer
├── scripts/                # Deployment automation
├── docker-compose.yml      # Infrastructure services
└── README.md
```

## Quick Start

### Prerequisites
- Go 1.19+
- curl (for testing)
- Docker & Docker Compose (optional, for full features)

### Instant Start (No Docker Required)
```bash
# Make scripts executable
chmod +x scripts/*.sh

# Start the game immediately
./scripts/start-simple.sh
```

### Play the Game
Open your browser and go to: http://localhost:8080

### Test Everything
``
### ripts/test.sh
```

### Full Setup (with Database & Analytics)
```bash
# Full setup with PostgreSQL and Kafka
./scripts/setup.sh
./scripts/start.sh
```

### Manual Setup

1. Simple Mode (No dependencies):
```bash
cd backend
go mod tidy
go run main.go
```

2. Full Mode (with Docker):
```bash
docker-compose up -d
cd backend && go run main.go
cd analytics && go run main.go
```

3. Access the Game: http://localhost:8080

## How to Play

1. Enter your username
2. Wait for opponent or play against bot (10s timeout)
3. Drop discs by clicking columns
4. First to connect 4 wins!
5. Check leaderboard for rankings

## Bot Intelligence

The bot uses strategic algorithms:
- Immediate threat detection: Blocks player wins
- Opportunity creation: Seeks winning moves
- Center preference: Controls board center
- Trap setup: Creates multiple winning paths

## Analytics

Kafka tracks:
- Game events (start, move, end)
- Player metrics (wins, games played)
- Performance data (game duration, moves)

## Leaderboard

Real-time rankings based on:
- Total wins
- Win percentage
- Games played

## Configuration

Environment variables:
- `PORT`: Server port (default: 8080)
- `DB_URL`: PostgreSQL connection
- `KAFKA_BROKERS`: Kafka broker addresses

## Production Ready

- Graceful shutdowns
- Error handling
- Connection pooling
- Event-driven architecture
- Scalable design

## API Endpoints

### REST API
- `GET /api/leadereSQL connectioop players ranking
- `GET /api/stats` - Get game statistics and metrics

### WebSocket Events
- `join_game` - Join matchmaking queue
- `make_move` - Make a game move
- `reconnect` - Reconnect to existing game

## Frontend Features

### Responsive Design
- Mobile-first: Optimized for all screen sizes
- Real-time Updates: Live game state synchronization
- Visual Feedback: Animated moves and status indicators
- Connection Status: Real-time connection monitoring

### User Experience
- Intuitive Interface: Click columns to drop discs
- Game Status: Clear turn indicators and game state
- Leaderboard: Live rankings and statistics
- Error Handling: Graceful error messages and recovery

## Security Features

- Input Validation: All user inputs sanitized
- CORS Proteipts/*.shigured for production environments
- Rate Limiting: Prevents abuse and spam
- Connection Security: WebSocket authentication ready
- SQL Injection Prevention: Parameterized queries

## Performance Optimizations

### Backend
- Connection Pooling: Efficient database connections
- Memory Management: Automatic game cleanup
- Concurrent Processing: Goroutine-based architecture
- Event Streaming: Asynchronous analytics processing

### Frontend
- Code Splitting: Optimized bundle sizes
- WebSocket Reconnection: Automatic connection recovery
- Optimistic Updates: Immediate UI feedback
- Caching: Efficient state management

## Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Built with Go and React for optimal performance
- Kafka integration for enterprise-grade analytics
- PostgreSQL for reliable data persistence
- Docker for consistent development environments

---

Ready to play? Start the game and challenge the intelligent bot or find a human opponent for the ultimate 4-in-a-row showdown!
