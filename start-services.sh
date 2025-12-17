#!/bin/bash

# Configuration
PORT=3000

echo "🚀 Starting Trilix Services..."

# Kill any existing process on the target port
echo "🧹 Cleaning up port $PORT..."
lsof -ti:$PORT | xargs kill -9 2>/dev/null

# Clean up Python server if running
pkill -f "python3 -m http.server" 2>/dev/null

# Start Confluence Service (Microservice)
echo "Starting Confluence Service..."
(cd cmd/confluence-service && go run main.go > ../../confluence-service.log 2>&1 &)

# Start Jira Service (Microservice)
echo "Starting Jira Service..."
(cd cmd/jira-service && go run main.go > ../../jira-service.log 2>&1 &)

sleep 2

# Build the server
echo "🔨 Building Go Server..."
cd cmd/mcp-server
go build -o mcp-server

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    echo "🌐 Starting server on http://localhost:$PORT"
    echo "   - Frontend: http://localhost:$PORT/trilix-workspaces.html"
    echo "   - Test Client: http://localhost:$PORT/docs/test-client.html"
    echo "   - API: http://localhost:$PORT/api/workspaces"
    echo "   - SSE: http://localhost:$PORT/sse"
    echo ""
    echo "Logs:"
    ./mcp-server
else
    echo "❌ Build failed!"
    exit 1
fi
