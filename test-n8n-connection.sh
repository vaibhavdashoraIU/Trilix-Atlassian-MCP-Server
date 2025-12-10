#!/bin/bash

echo "Testing MCP Server for n8n Cloud..."
echo ""

# Test 1: Check if server is running
echo "1. Testing local server..."
response=$(curl -s -X POST http://localhost:3001/message \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}')

if echo "$response" | grep -q "trilix-atlassian-mcp-server"; then
    echo "✅ Local server is working"
else
    echo "❌ Local server is NOT responding"
    exit 1
fi

echo ""
echo "2. Checking if ngrok is installed..."
if command -v ngrok &> /dev/null; then
    echo "✅ ngrok is installed"
else
    echo "❌ ngrok is NOT installed"
    echo ""
    echo "Install ngrok:"
    echo "  brew install ngrok"
    echo ""
    echo "Or download from: https://ngrok.com/download"
    exit 1
fi

echo ""
echo "3. Checking if ngrok is running..."
if lsof -i :4040 &> /dev/null; then
    echo "✅ ngrok appears to be running"
    echo ""
    echo "Get your public URL:"
    echo "  curl -s http://localhost:4040/api/tunnels | python3 -c \"import sys, json; print(json.load(sys.stdin)['tunnels'][0]['public_url'])\""
else
    echo "❌ ngrok is NOT running"
    echo ""
    echo "Start ngrok in a new terminal:"
    echo "  ngrok http 3001"
fi

echo ""
echo "=========================================="
echo "For n8n Cloud, use:"
echo "  Transport: HTTP"
echo "  Endpoint: https://YOUR-NGROK-URL.ngrok.io"
echo "=========================================="
