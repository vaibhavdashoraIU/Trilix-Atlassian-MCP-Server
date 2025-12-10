package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// SSEServer implements MCP protocol over Server-Sent Events
type SSEServer struct {
	server  *Server
	handler func(ToolCall) (ToolResult, error)
	mu      sync.Mutex
}

// NewSSEServer creates a new SSE-based MCP server
func NewSSEServer(server *Server, handler func(ToolCall) (ToolResult, error)) *SSEServer {
	return &SSEServer{
		server:  server,
		handler: handler,
	}
}

// StartSSE starts the SSE server
func (s *SSEServer) StartSSE(port int) error {
	http.HandleFunc("/", s.handleMessage)        // Root endpoint for n8n
	http.HandleFunc("/sse", s.handleSSE)
	http.HandleFunc("/message", s.handleMessage)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("MCP SSE Server listening on %s\n", addr)
	return http.ListenAndServe(addr, s.corsMiddleware(http.DefaultServeMux))
}

func (s *SSEServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func (s *SSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial connection message
	fmt.Fprintf(w, "event: endpoint\ndata: /message\n\n")
	flusher.Flush()

	// Keep connection alive
	<-r.Context().Done()
}

func (s *SSEServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var request map[string]interface{}
	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	method, _ := request["method"].(string)
	var response map[string]interface{}

	switch method {
	case "initialize":
		response = s.handleInitialize(request)
	case "tools/list":
		response = s.handleListTools()
	case "tools/call":
		response = s.handleToolCall(request)
	default:
		response = map[string]interface{}{
			"jsonrpc": "2.0",
			"error": map[string]interface{}{
				"code":    -32601,
				"message": fmt.Sprintf("Method not found: %s", method),
			},
		}
	}

	response["jsonrpc"] = "2.0"
	if id, ok := request["id"]; ok {
		response["id"] = id
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *SSEServer) handleInitialize(request map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "trilix-atlassian-mcp-server",
				"version": "1.0.0",
			},
		},
	}
}

func (s *SSEServer) handleListTools() map[string]interface{} {
	return map[string]interface{}{
		"result": map[string]interface{}{
			"tools": s.server.tools,
		},
	}
}

func (s *SSEServer) handleToolCall(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{
			"error": map[string]interface{}{
				"code":    -32602,
				"message": "Invalid params",
			},
		}
	}

	name, _ := params["name"].(string)
	arguments, _ := params["arguments"].(map[string]interface{})

	toolCall := ToolCall{
		Name:      name,
		Arguments: arguments,
	}

	result, err := s.handler(toolCall)
	if err != nil {
		return map[string]interface{}{
			"error": map[string]interface{}{
				"code":    -32000,
				"message": err.Error(),
			},
		}
	}

	return map[string]interface{}{
		"result": result,
	}
}
