#!/bin/bash

echo "🚀 Starting GoTask Demo System..."

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go first."
    exit 1
fi

# 检查 Node.js 是否安装
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js first."
    exit 1
fi

echo "✅ Dependencies check passed"

# 构建前端
echo "📦 Building frontend..."
cd web
npm install
npm run build
cd ..

# 启动后端服务器
echo "🔧 Starting backend server..."
cd examples/server
echo "🌟 Starting server on http://localhost:8080"
echo "💡 Press Ctrl+C to stop the server"
echo ""
echo "📊 API endpoints:"
echo "  GET  http://localhost:8080/api/tasks/tree"
echo "  GET  http://localhost:8080/api/tasks"
echo "  POST http://localhost:8080/api/tasks"
echo "  GET  http://localhost:8080/api/tasks/{id}"
echo "  POST http://localhost:8080/api/tasks/{id}/stop"
echo "  GET  http://localhost:8080/api/tasks/history"
echo "  GET  http://localhost:8080/api/tasks/stats"
echo ""
echo "🌐 Frontend: http://localhost:8080"
echo ""

go run main.go