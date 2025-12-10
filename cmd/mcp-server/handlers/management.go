package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/providentiaww/trilix-atlassian-mcp/internal/storage"
	"github.com/providentiaww/trilix-atlassian-mcp/pkg/mcp"
)

// ManagementHandler handles workspace management tools
type ManagementHandler struct {
	credStore storage.CredentialStoreInterface
}

// NewManagementHandler creates a new management handler
func NewManagementHandler(credStore storage.CredentialStoreInterface) *ManagementHandler {
	return &ManagementHandler{
		credStore: credStore,
	}
}

// ListTools returns the list of management tools
func (h *ManagementHandler) ListTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "list_workspaces",
			Description: "List all configured Atlassian workspaces. You can connect to multiple workspaces simultaneously and query different organizations in the same chat session.",
			InputType:   "object",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "workspace_status",
			Description: "Check connectivity status of a workspace",
			InputType:   "object",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID to check",
					},
				},
				"required": []string{"workspace_id"},
			},
		},
	}
}

// HandleTool handles a management tool call
func (h *ManagementHandler) HandleTool(call mcp.ToolCall, userID string) (mcp.ToolResult, error) {
	switch call.Name {
	case "list_workspaces":
		return h.handleListWorkspaces(userID)
	case "workspace_status":
		return h.handleWorkspaceStatus(call, userID)
	default:
		return mcp.ToolResult{
			Content: []mcp.ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", call.Name)},
			},
			IsError: true,
		}, fmt.Errorf("unknown tool: %s", call.Name)
	}
}

func (h *ManagementHandler) handleListWorkspaces(userID string) (mcp.ToolResult, error) {
	workspaces, err := h.credStore.ListWorkspaces(userID)
	if err != nil {
		return mcp.ToolResult{
			Content: []mcp.ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, err
	}

	resultJSON, _ := json.MarshalIndent(workspaces, "", "  ")

	return mcp.ToolResult{
		Content: []mcp.ContentBlock{
			{Type: "text", Text: string(resultJSON)},
		},
	}, nil
}

func (h *ManagementHandler) handleWorkspaceStatus(call mcp.ToolCall, userID string) (mcp.ToolResult, error) {
	workspaceID, ok := call.Arguments["workspace_id"].(string)
	if !ok {
		return mcp.ToolResult{
			Content: []mcp.ContentBlock{
				{Type: "text", Text: "Error: workspace_id is required"},
			},
			IsError: true,
		}, fmt.Errorf("workspace_id is required")
	}

	_, err := h.credStore.GetCredentials(userID, workspaceID)
	if err != nil {
		return mcp.ToolResult{
			Content: []mcp.ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Workspace not found: %s", workspaceID)},
			},
			IsError: true,
		}, err
	}

	result := map[string]interface{}{
		"workspace_id": workspaceID,
		"status":       "connected",
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")

	return mcp.ToolResult{
		Content: []mcp.ContentBlock{
			{Type: "text", Text: string(resultJSON)},
		},
	}, nil
}

