#!/bin/zsh
# Stop all Trilix Atlassian MCP Server services (macOS/zsh)
# chmod +x stop-services.sh ./stop-services.sh

echo "Stopping Trilix Atlassian MCP Server services..."

# Find all Go processes running main.go in this project
PIDS=($(pgrep -f "go run main.go"))

if [[ ${#PIDS[@]} -eq 0 ]]; then
  echo "No running services found."
  exit 0
fi

echo "Found ${#PIDS[@]} service(s) to stop..."
for PID in $PIDS; do
  echo "Stopping process $PID..."
  kill $PID
  sleep 1
  if kill -0 $PID 2>/dev/null; then
    echo "Process $PID did not stop, forcing..."
    kill -9 $PID
  fi
done

echo ""
echo "All services stopped."
