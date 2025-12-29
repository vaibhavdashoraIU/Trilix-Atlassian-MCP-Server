#!/bin/bash

echo "🛑 Stopping Trilix Services..."

# Kill process on port 3000
echo "🧹 Cleaning up port 3000..."
lsof -ti:3000 | xargs kill -9 2>/dev/null

# Make sure the binary is dead
pkill -f "mcp-server" 2>/dev/null


# Kill the microservices (go run processes fallback)
pkill -f "go run main.go" 2>/dev/null
pkill -f "confluence-service" 2>/dev/null
pkill -f "jira-service" 2>/dev/null

echo "✅ All services stopped."
