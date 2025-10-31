#!/bin/bash

set -e

echo "Deploying 4-in-a-Row Game to Production..."

# Build backend
echo "🔧 Building backend..."
cd backend
go build -o ../bin/connect4-backend main.go
cd ..

# Build analytics
echo "Building analytics..."
cd analytics
go build -o ../bin/connect4-analytics main.go
cd ..

# Create bin directory if it doesn't exist
mkdir -p bin

echo "Build complete!"
echo ""
echo "Deployment files:"
echo "   - Backend binary: bin/connect4-backend"
echo "   - Analytics binary: bin/connect4-analytics"
echo "   - Frontend: frontend/public/game.html"
echo "   - Docker config: docker-compose.yml"
echo ""
echo "🌐 To deploy to production:"
echo "   1. Copy all files to your server"
echo "   2. Run: docker-compose up -d"
echo "   3. Run: ./bin/connect4-backend"
echo "   4. Run: ./bin/connect4-analytics"
echo ""
echo "🔧 Environment variables for production:"
echo "   export PORT=8080"
echo "   export DATABASE_URL='postgres://user:pass@host:5432/dbname'"
echo "   export KAFKA_BROKERS='broker1:9092,broker2:9092'"