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
	managementHandler *ManagementHandler
}

// NewRestToolHandler creates a new REST tool handler
func NewRestToolHandler(confluenceHandler *ConfluenceHandler, jiraHandler *JiraHandler, managementHandler *ManagementHandler) *RestToolHandler {
	return &RestToolHandler{
		confluenceHandler: confluenceHandler,
		jiraHandler:       jiraHandler,
		managementHandler: managementHandler,
	}
}

// HandleToolRequest generic handler for tool execution
func (h *RestToolHandler) HandleToolRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Incoming REST request: %s %s from %s\n", r.Method, r.URL.Path, r.RemoteAddr)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract tool name from URL path
	// Expected format: /api/tools/{tool_name}
	
	trimmedPath := strings.TrimSpace(r.URL.Path)
	trimmedPath = strings.Trim(trimmedPath, "/")
	parts := strings.Split(trimmedPath, "/")
	
	toolName := ""
	if len(parts) >= 3 {
		toolName = parts[2]
	} else {
		fmt.Printf("Malformed REST path: %s\n", r.URL.Path)
		http.Error(w, "Invalid path format. Expected /api/tools/{tool_name}", http.StatusBadRequest)
		return
	}

	// Extract user ID
	userID := ""
	if userCtx, ok := auth.ExtractUserFromContext(r.Context()); ok {
		userID = userCtx.UserID
	}

	// Route to correct handler
	var result mcp.ToolResult
	var err error

	fmt.Printf("REST Tool Call: %s for user %s\n", toolName, userID)

	// Parse body as arguments
	var arguments map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&arguments); err != nil {
		fmt.Printf("Error decoding request body: %v\n", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Trusted Service Override: Extract user_id from arguments if authenticated via Service Token
	if isService, ok := r.Context().Value("IsServiceCall").(bool); ok && isService {
		if injectedUser, ok := arguments["user_id"].(string); ok && injectedUser != "" {
			fmt.Printf("🔒 Service Override: Using user_id=%s from input\n", injectedUser)
			userID = injectedUser
			// Clean up arguments to avoid passing user_id to the actual tool if not needed
			// But for now, keeping it is harmless as tools ignore unknown args
		}
	}

	call := mcp.ToolCall{
		Name:      toolName,
		Arguments: arguments,
	}

	if toolName == "list_workspaces" || toolName == "workspace_status" {
		result, err = h.managementHandler.HandleTool(call, userID)
	} else if strings.HasPrefix(toolName, "confluence_") {
		result, err = h.confluenceHandler.HandleTool(call, userID)
	} else if strings.HasPrefix(toolName, "jira_") {
		result, err = h.jiraHandler.HandleTool(call, userID)
	} else {
		fmt.Printf("Unknown REST tool: %s\n", toolName)
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
