#!/bin/bash

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🚀 Starting GoTask Demo System..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go first."
    exit 1
fi

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js first."
    exit 1
fi

echo "✅ Dependencies check passed"

# Build frontend
echo "📦 Building frontend..."
cd "$SCRIPT_DIR/web" || exit
if command -v pnpm &> /dev/null; then
    pnpm install
    pnpm run build
else
    npm install
    npm run build
fi

# Start backend server
echo "🔧 Starting backend server..."
cd "$SCRIPT_DIR/server" || exit
echo "🌟 Starting server on http://localhost:8082"
echo "💡 Press Ctrl+C to stop the server"
echo ""
echo "📊 API endpoints:"
echo "  GET  http://localhost:8082/api/tasks/tree"
echo "  GET  http://localhost:8082/api/tasks"
echo "  POST http://localhost:8082/api/tasks"
echo "  GET  http://localhost:8082/api/tasks/{id}"
echo "  POST http://localhost:8082/api/tasks/{id}/stop"
echo "  GET  http://localhost:8082/api/tasks/history"
echo "  GET  http://localhost:8082/api/tasks/stats"
echo ""
echo "🌐 Frontend: http://localhost:8082"
echo ""

go run main.go
