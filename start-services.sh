#!/bin/zsh
# Start all Trilix Atlassian MCP Server services (macOS/zsh)

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
GO_BIN="$(command -v go)"

if [[ -z "$GO_BIN" ]]; then
  echo "Error: Go not found in PATH. Please install Go." >&2
  exit 1
fi

if [[ ! -f "$PROJECT_ROOT/.env" ]]; then
  echo "Warning: .env file not found. Services may fail to start." >&2
fi

# Start Confluence Service
echo "Starting Confluence Service..."
(cd "$PROJECT_ROOT/cmd/confluence-service" && nohup "$GO_BIN" run main.go > "$PROJECT_ROOT/confluence-service.log" 2>&1 &)
sleep 1

# Start Jira Service
echo "Starting Jira Service..."
(cd "$PROJECT_ROOT/cmd/jira-service" && nohup "$GO_BIN" run main.go > "$PROJECT_ROOT/jira-service.log" 2>&1 &)
sleep 1

# Start MCP Server
echo "Starting MCP Server..."
(cd "$PROJECT_ROOT/cmd/mcp-server" && nohup "$GO_BIN" run main.go > "$PROJECT_ROOT/mcp-server.log" 2>&1 &)
sleep 2

echo ""
echo "All services started!"
echo "Logs:"
echo "  $PROJECT_ROOT/confluence-service.log"
echo "  $PROJECT_ROOT/jira-service.log"
echo "  $PROJECT_ROOT/mcp-server.log"
echo ""
echo "To stop services, use: pkill -f 'go run main.go' or kill the relevant PIDs."
