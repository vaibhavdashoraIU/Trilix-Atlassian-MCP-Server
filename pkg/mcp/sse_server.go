package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/providentiaww/trilix-atlassian-mcp/cmd/mcp-server/auth"
)

// SSEServer implements MCP protocol over Server-Sent Events
type SSEServer struct {
	server  *Server
	handler func(ToolCall, string) (ToolResult, error) // Updated to accept userID
	mu      sync.Mutex
}

// NewSSEServer creates a new SSE-based MCP server
func NewSSEServer(server *Server, handler func(ToolCall, string) (ToolResult, error)) *SSEServer {
	return &SSEServer{
		server:  server,
		handler: handler,
	}
}

// HandleSSE handles SSE connection establishment
func (s *SSEServer) HandleSSE(w http.ResponseWriter, r *http.Request) {
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

	// Keep connection alive until client disconnects
	<-r.Context().Done()
}

// HandleMessage handles MCP protocol messages
func (s *SSEServer) HandleMessage(w http.ResponseWriter, r *http.Request) {
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
		response = s.handleToolCall(request, r)
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

func (s *SSEServer) handleToolCall(request map[string]interface{}, r *http.Request) map[string]interface{} {
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

	// Extract userID from request context (set by auth middleware)
	userID := ""
	if userCtx, ok := auth.ExtractUserFromContext(r.Context()); ok {
		userID = userCtx.UserID
	}

	result, err := s.handler(toolCall, userID)
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

// StreamEvent represents an SSE event
type StreamEvent struct {
	Type    string      `json:"type"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SendSSEEvent sends an SSE event to the client
func SendSSEEvent(w http.ResponseWriter, event StreamEvent) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
	return nil
}
