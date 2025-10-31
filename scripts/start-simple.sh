#!/bin/bash

set -e

echo "Starting 4-in-a-Row Game (Simple Mode - No Docker Required)..."

# Function to check if port is in use
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "WARNING: Port $port is already in use. Attempting to free it..."
        lsof -ti:$port | xargs kill -9 2>/dev/null || true
        sleep 2
    fi
}

# Cleanup function
cleanup() {
    echo ""
    echo "Stopping all services..."
    
    # Kill background processes
    if [ ! -z "$BACKEND_PID" ]; then
        kill $BACKEND_PID 2>/dev/null || true
    fi
    if [ ! -z "$ANALYTICS_PID" ]; then
        kill $ANALYTICS_PID 2>/dev/null || true
    fi
    
    echo "All services stopped"
    exit 0
}

# Set up signal handlers
trap cleanup INT TERM

# Check and free ports
check_port 8080

echo "🔧 Building and starting backend server..."
cd backend
go mod tidy

# Start backend server (without database and Kafka for simplicity)
export DATABASE_URL=""
export KAFKA_BROKERS=""
go run main.go &
BACKEND_PID=$!
cd ..

# Wait for backend to be ready
echo "⏳ Waiting for backend to start..."
sleep 3

# Check if backend is running
if curl -s "http://localhost:8080/health" >/dev/null 2>&1; then
    echo "Backend server is running!"
else
    echo "ERROR: Backend server failed to start"
    cleanup
    exit 1
fi

echo ""
echo "Game is running successfully!"
echo ""
echo "Play the game: http://localhost:8080"
echo "API Health: http://localhost:8080/health"
echo "Statistics: http://localhost:8080/api/stats"
echo "Leaderboard: http://localhost:8080/api/leaderboard"
echo ""
echo "Note: Running in simple mode (no database/analytics)"
echo "   - Leaderboard will be empty"
echo "   - No game analytics"
echo "   - Games are not persisted"
echo ""
echo "📱 Open your browser and go to: http://localhost:8080"
echo ""
echo "Press Ctrl+C to stop the server"

# Keep script running and wait for interrupt
while true; do
    # Check if backend is still running
    if ! kill -0 $BACKEND_PID 2>/dev/null; then
        echo "ERROR: Backend server stopped unexpectedly"
        cleanup
        exit 1
    fi
    
    sleep 5
done