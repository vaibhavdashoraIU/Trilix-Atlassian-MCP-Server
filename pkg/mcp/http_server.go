package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// HTTPServer wraps MCP server with HTTP endpoints
type HTTPServer struct {
	server  *Server
	handler func(ToolCall) (ToolResult, error)
}

// NewHTTPServer creates a new HTTP server
func NewHTTPServer(server *Server, handler func(ToolCall) (ToolResult, error)) *HTTPServer {
	return &HTTPServer{
		server:  server,
		handler: handler,
	}
}

// StartHTTP starts the HTTP server
func (h *HTTPServer) StartHTTP(port int) error {
	http.HandleFunc("/health", h.handleHealth)
	http.HandleFunc("/tools", h.handleListTools)
	http.HandleFunc("/tools/call", h.handleToolCall)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("MCP HTTP Server listening on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}

func (h *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *HTTPServer) handleListTools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": h.server.tools,
	})
}

func (h *HTTPServer) handleToolCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	toolCall := ToolCall{
		Name:      req.Name,
		Arguments: req.Arguments,
	}

	result, err := h.handler(toolCall)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
