package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/providentiaww/trilix-atlassian-mcp/cmd/mcp-server/auth"
	"github.com/providentiaww/trilix-atlassian-mcp/pkg/mcp"
)

// RestToolHandler generic handler for exposing MCP tools via REST
type RestToolHandler struct {
	confluenceHandler *ConfluenceHandler
	jiraHandler       *JiraHandler
}

// NewRestToolHandler creates a new REST tool handler
func NewRestToolHandler(confluenceHandler *ConfluenceHandler, jiraHandler *JiraHandler) *RestToolHandler {
	return &RestToolHandler{
		confluenceHandler: confluenceHandler,
		jiraHandler:       jiraHandler,
	}
}

// HandleToolRequest generic handler for tool execution
func (h *RestToolHandler) HandleToolRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract tool name from URL path
	// Expected format: /api/tools/{tool_name}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) == 0 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	toolName := parts[len(parts)-1]

	// Extract user ID
	userID := ""
	if userCtx, ok := auth.ExtractUserFromContext(r.Context()); ok {
		userID = userCtx.UserID
	} else {
		// Fallback for dev mode
		// http.Error(w, "Unauthorized", http.StatusUnauthorized)
		// return
	}

	// Parse body as arguments
	var arguments map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&arguments); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Construct ToolCall
	call := mcp.ToolCall{
		Name:      toolName,
		Arguments: arguments,
	}

	// Route to correct handler
	var result mcp.ToolResult
	var err error

	if strings.HasPrefix(toolName, "confluence_") {
		result, err = h.confluenceHandler.HandleTool(call, userID)
	} else if strings.HasPrefix(toolName, "jira_") {
		result, err = h.jiraHandler.HandleTool(call, userID)
	} else {
		http.Error(w, fmt.Sprintf("Unknown tool: %s", toolName), http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result.IsError {
		errMsg := "Unknown error"
		if len(result.Content) > 0 {
			errMsg = result.Content[0].Text
		}
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	// Return result
	w.Header().Set("Content-Type", "application/json")
	
	// The tool handlers return JSON strings wrapped in Text content
	// We want to return actual JSON, so we try to parse it first
	// If parsing fails (plain text), we wrap it in a JSON object
	
	var jsonContent interface{}
	if len(result.Content) > 0 {
		textContent := result.Content[0].Text
		err := json.Unmarshal([]byte(textContent), &jsonContent)
		if err == nil {
			json.NewEncoder(w).Encode(jsonContent)
		} else {
			// Not JSON, return as object
			json.NewEncoder(w).Encode(map[string]string{
				"result": textContent,
			})
		}
	} else {
		json.NewEncoder(w).Encode(map[string]string{
			"status": "success",
		})
	}
}
