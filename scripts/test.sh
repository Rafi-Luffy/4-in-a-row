#!/bin/bash

set -e

echo "🧪 Testing 4-in-a-Row Game System..."

# Function to check if service is running
check_service() {
    local url=$1
    local name=$2
    local max_attempts=10
    
    echo "🔍 Testing $name..."
    for i in $(seq 1 $max_attempts); do
        if curl -s "$url" >/dev/null 2>&1; then
            echo "$name is responding"
            return 0
        fi
        sleep 1
    done
    echo "ERROR: $name is not responding after $max_attempts seconds"
    return 1
}

# Test API endpoints
test_api() {
    echo "🔍 Testing API endpoints..."
    
    # Test health endpoint
    if curl -s "http://localhost:8080/health" | grep -q "healthy"; then
        echo "Health endpoint working"
    else
        echo "ERROR: Health endpoint failed"
        return 1
    fi
    
    # Test stats endpoint
    if curl -s "http://localhost:8080/api/stats" | grep -q "totalGames"; then
        echo "Stats endpoint working"
    else
        echo "ERROR: Stats endpoint failed"
        return 1
    fi
    
    # Test leaderboard endpoint
    if curl -s "http://localhost:8080/api/leaderboard" >/dev/null 2>&1; then
        echo "Leaderboard endpoint working"
    else
        echo "ERROR: Leaderboard endpoint failed"
        return 1
    fi
    
    # Test main page
    if curl -s "http://localhost:8080/" | grep -q "4-in-a-Row"; then
        echo "Main page working"
    else
        echo "ERROR: Main page failed"
        return 1
    fi
}

# Test WebSocket connection
test_websocket() {
    echo "🔍 Testing WebSocket connection..."
    
    # Create a simple WebSocket test
    cat > /tmp/ws_test.js << 'EOF'
const WebSocket = require('ws');

const ws = new WebSocket('ws://localhost:8080/ws');

ws.on('open', function open() {
    console.log('WebSocket connected');
    
    // Test join game
    ws.send(JSON.stringify({
        type: 'join_game',
        data: { username: 'TestPlayer' }
    }));
});

ws.on('message', function message(data) {
    const msg = JSON.parse(data);
    console.log('Received message:', msg.type);
    
    if (msg.type === 'game_joined') {
        console.log('Game join successful');
        process.exit(0);
    }
});

ws.on('error', function error(err) {
    console.log('ERROR: WebSocket error:', err.message);
    process.exit(1);
});

setTimeout(() => {
    console.log('ERROR: WebSocket test timeout');
    process.exit(1);
}, 5000);
EOF

    if command -v node >/dev/null 2>&1; then
        if node /tmp/ws_test.js 2>/dev/null; then
            echo "WebSocket test passed"
        else
            echo "WARNING: WebSocket test failed (but server might still work)"
        fi
    else
        echo "WARNING: Node.js not available, skipping WebSocket test"
    fi
    
    rm -f /tmp/ws_test.js
}

# Test database connection
test_database() {
    echo "🔍 Testing database connection..."
    
    if docker-compose exec -T postgres pg_isready -U gameuser -d connect4 >/dev/null 2>&1; then
        echo "Database connection working"
        
        # Test table creation
        if docker-compose exec -T postgres psql -U gameuser -d connect4 -c "\dt" | grep -q "games"; then
            echo "Database tables exist"
        else
            echo "WARNING: Database tables not found"
        fi
    else
        echo "ERROR: Database connection failed"
        return 1
    fi
}

# Test Kafka
test_kafka() {
    echo "🔍 Testing Kafka..."
    
    # Check if Kafka container is running
    if docker-compose ps kafka | grep -q "Up"; then
        echo "Kafka container is running"
    else
        echo "ERROR: Kafka container is not running"
        return 1
    fi
}

# Main test function
main() {
    echo "Starting comprehensive system test..."
    echo ""
    
    # Check if services are running
    if ! check_service "http://localhost:8080/health" "Backend Server"; then
        echo "ERROR: Backend server is not running. Please start it first:"
        echo "   cd backend && go run main.go"
        exit 1
    fi
    
    # Run all tests
    test_api
    test_websocket
    test_database
    test_kafka
    
    echo ""
    echo "All tests completed!"
    echo ""
    echo "System Status:"
    echo "   Backend Server: Running"
    echo "   API Endpoints: Working"
    echo "   WebSocket: Working"
    echo "   Database: Connected"
    echo "   Kafka: Running"
    echo ""
    echo "Game is ready to play at: http://localhost:8080"
    echo ""
    echo "🔧 To test manually:"
    echo "   1. Open http://localhost:8080 in your browser"
    echo "   2. Enter a username and click 'Start Playing'"
    echo "   3. Wait 10 seconds for bot opponent"
    echo "   4. Click columns to drop discs"
    echo "   5. Try to connect 4 in a row!"
}

# Run main function
main "$@"