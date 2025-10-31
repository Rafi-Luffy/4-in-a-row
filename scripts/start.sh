#!/bin/bash

set -e  # Exit on any error

echo "Starting 4-in-a-Row Game Server..."

# Function to check if port is in use
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "WARNING: Port $port is already in use. Attempting to free it..."
        lsof -ti:$port | xargs kill -9 2>/dev/null || true
        sleep 2
    fi
}

# Function to wait for service
wait_for_service() {
    local url=$1
    local name=$2
    local max_attempts=30
    
    echo "⏳ Waiting for $name to be ready..."
    for i in $(seq 1 $max_attempts); do
        if curl -s "$url" >/dev/null 2>&1; then
            echo "$name is ready!"
            return 0
        fi
        sleep 1
    done
    echo "ERROR: $name failed to start after $max_attempts seconds"
    return 1
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
    
    # Stop Docker services
    docker-compose down 2>/dev/null || true
    
    echo "All services stopped"
    exit 0
}

# Set up signal handlers
trap cleanup INT TERM

# Check and free ports
check_port 8080
check_port 5432
check_port 9092

# Start infrastructure services
echo "📦 Starting infrastructure services..."
docker-compose down 2>/dev/null || true
docker-compose up -d

# Wait for PostgreSQL
echo "⏳ Waiting for PostgreSQL..."
for i in {1..30}; do
    if docker-compose exec -T postgres pg_isready -U gameuser -d connect4 &>/dev/null; then
        echo "PostgreSQL is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: PostgreSQL failed to start"
        docker-compose logs postgres
        exit 1
    fi
    sleep 1
done

# Wait for Kafka
echo "⏳ Waiting for Kafka..."
sleep 10

# Start backend server
echo "🔧 Starting backend server..."
cd backend
go run main.go &
BACKEND_PID=$!
cd ..

# Wait for backend to be ready
sleep 3
if ! wait_for_service "http://localhost:8080/api/stats" "Backend Server"; then
    echo "ERROR: Backend server failed to start"
    cleanup
    exit 1
fi

# Start analytics consumer
echo "Starting analytics consumer..."
cd analytics
go run main.go &
ANALYTICS_PID=$!
cd ..

# Give analytics time to connect
sleep 2

echo ""
echo "All services are running successfully!"
echo ""
echo "Game Server: http://localhost:8080"
echo "API Endpoints:"
echo "   - Leaderboard: http://localhost:8080/api/leaderboard"
echo "   - Statistics:  http://localhost:8080/api/stats"
echo ""
echo "🔧 Infrastructure:"
echo "   - PostgreSQL: localhost:5432"
echo "   - Kafka:      localhost:9092"
echo ""
echo "📱 Open your browser and go to: http://localhost:8080"
echo ""
echo "Press Ctrl+C to stop all services"

# Keep script running and wait for interrupt
while true; do
    # Check if backend is still running
    if ! kill -0 $BACKEND_PID 2>/dev/null; then
        echo "ERROR: Backend server stopped unexpectedly"
        cleanup
        exit 1
    fi
    
    # Check if analytics is still running
    if ! kill -0 $ANALYTICS_PID 2>/dev/null; then
        echo "WARNING: Analytics consumer stopped, restarting..."
        cd analytics
        go run main.go &
        ANALYTICS_PID=$!
        cd ..
    fi
    
    sleep 5
done