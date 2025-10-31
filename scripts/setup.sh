#!/bin/bash

set -e  # Exit on any error

echo "Setting up 4-in-a-Row Game..."

# Check prerequisites
check_prerequisites() {
    echo "🔍 Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        echo "ERROR: Docker is not installed. Please install Docker first."
        echo "   Visit: https://docs.docker.com/get-docker/"
        exit 1
    fi

    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        echo "ERROR: Docker Compose is not installed. Please install Docker Compose first."
        echo "   Visit: https://docs.docker.com/compose/install/"
        exit 1
    fi

    if ! command -v go &> /dev/null; then
        echo "ERROR: Go is not installed. Please install Go 1.19+ first."
        echo "   Visit: https://golang.org/doc/install"
        exit 1
    fi

    if ! command -v node &> /dev/null; then
        echo "ERROR: Node.js is not installed. Please install Node.js 16+ first."
        echo "   Visit: https://nodejs.org/"
        exit 1
    fi

    echo "All prerequisites satisfied!"
}

# Start infrastructure services
start_infrastructure() {
    echo "📦 Starting PostgreSQL and Kafka..."
    
    # Stop any existing containers
    docker-compose down 2>/dev/null || true
    
    # Start services
    docker-compose up -d
    
    echo "⏳ Waiting for services to be ready..."
    
    # Wait for PostgreSQL
    echo "   Waiting for PostgreSQL..."
    for i in {1..30}; do
        if docker-compose exec -T postgres pg_isready -U gameuser -d connect4 &>/dev/null; then
            echo "   PostgreSQL is ready!"
            break
        fi
        if [ $i -eq 30 ]; then
            echo "   ERROR: PostgreSQL failed to start after 30 seconds"
            docker-compose logs postgres
            exit 1
        fi
        sleep 1
    done
    
    # Wait for Kafka
    echo "   Waiting for Kafka..."
    sleep 10  # Kafka needs more time to start
    echo "   Kafka should be ready!"
}

# Setup backend
setup_backend() {
    echo "🔧 Setting up Go backend..."
    cd backend
    
    # Clean and download dependencies
    go clean -modcache 2>/dev/null || true
    go mod tidy
    
    # Test compilation
    if ! go build -o /tmp/connect4-backend main.go; then
        echo "ERROR: Backend compilation failed!"
        exit 1
    fi
    
    rm -f /tmp/connect4-backend
    echo "   Backend setup complete!"
    cd ..
}

# Setup frontend
setup_frontend() {
    echo "🎨 Setting up React frontend..."
    cd frontend
    
    # Clean install
    rm -rf node_modules package-lock.json 2>/dev/null || true
    
    if ! npm install; then
        echo "ERROR: Frontend npm install failed!"
        exit 1
    fi
    
    # Build for production
    if ! npm run build; then
        echo "ERROR: Frontend build failed!"
        exit 1
    fi
    
    echo "   Frontend setup complete!"
    cd ..
}

# Setup analytics
setup_analytics() {
    echo "Setting up analytics consumer..."
    cd analytics
    
    go mod tidy
    
    # Test compilation
    if ! go build -o /tmp/connect4-analytics main.go; then
        echo "ERROR: Analytics compilation failed!"
        exit 1
    fi
    
    rm -f /tmp/connect4-analytics
    echo "   Analytics setup complete!"
    cd ..
}

# Main setup process
main() {
    check_prerequisites
    start_infrastructure
    setup_backend
    setup_frontend
    setup_analytics
    
    echo ""
    echo "Setup complete! Everything is ready to go!"
    echo ""
    echo "To start the game:"
    echo "   ./scripts/start.sh"
    echo ""
    echo "Or start services manually:"
    echo "   1. Backend:   cd backend && go run main.go"
    echo "   2. Analytics: cd analytics && go run main.go"
    echo "   3. Browser:   http://localhost:8080"
    echo ""
    echo "🔧 Services running:"
    echo "   - Game Server: http://localhost:8080"
    echo "   - PostgreSQL:  localhost:5432"
    echo "   - Kafka:       localhost:9092"
    echo ""
    echo "Ready for deployment!"
}

# Run main function
main "$@"