#!/bin/bash

# Development script for running the separated frontend

set -e

echo "🚀 Starting AI CV Evaluator with separated frontend..."

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    echo "❌ Please run this script from the project root directory"
    exit 1
fi

# Check if .env file exists
if [ ! -f ".env" ]; then
    echo "❌ .env file not found. Please create one based on .env.example"
    exit 1
fi

# Frontend is now always separated by default

echo "📦 Installing frontend dependencies..."
make frontend-install

echo "🔧 Starting backend services..."
docker-compose up -d db redis qdrant tika otel-collector jaeger prometheus grafana

echo "⏳ Waiting for services to be ready..."
sleep 10

echo "🚀 Starting backend API server..."
go run cmd/server/main.go &
BACKEND_PID=$!

echo "⏳ Waiting for backend to start..."
sleep 5

echo "🎨 Starting frontend development server..."
make frontend-dev &
FRONTEND_PID=$!

echo ""
echo "✅ Development environment is ready!"
echo ""
echo "🌐 Frontend: http://localhost:3001"
echo "🔧 Backend API: http://localhost:8080"
echo "📊 Grafana: http://localhost:3000"
echo "📈 Prometheus: http://localhost:9090"
echo "🔍 Jaeger: http://localhost:16686"
echo ""
echo "Press Ctrl+C to stop all services"

# Function to cleanup on exit
cleanup() {
    echo ""
    echo "🛑 Stopping services..."
    kill $BACKEND_PID 2>/dev/null || true
    kill $FRONTEND_PID 2>/dev/null || true
    docker-compose down
    echo "✅ All services stopped"
}

# Set trap to cleanup on script exit
trap cleanup EXIT

# Wait for user to stop
wait
