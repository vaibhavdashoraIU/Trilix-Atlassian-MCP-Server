package handlers

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
	"github.com/providentiaww/trilix-atlassian-mcp/pkg/mcp"
)

// JiraHandler handles Jira-related MCP tool calls
type JiraHandler struct {
	callService func(models.JiraRequest) (*models.JiraResponse, error)
}

// NewJiraHandler creates a new Jira handler
func NewJiraHandler(callService func(models.JiraRequest) (*models.JiraResponse, error)) *JiraHandler {
	return &JiraHandler{
		callService: callService,
	}
}

// ListTools returns the list of Jira tools
func (h *JiraHandler) ListTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "jira_list_projects",
			Description: "List all accessible Jira projects in a workspace",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
				},
				"required": []string{"workspace_id"},
			},
		},
		{
			Name:        "jira_list_issues",
			Description: "Search for Jira issues using JQL. Supports querying multiple workspaces - specify workspace_id to search a specific organization.",
			InputType:   "object",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID to query (e.g., 'workspace-1', 'providentia'). Use list_workspaces to see available workspaces.",
					},
					"jql": map[string]interface{}{
						"type":        "string",
						"description": "JQL query string",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results",
						"default":     50,
					},
					"fields": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Fields to return",
					},
				},
				"required": []string{"workspace_id", "jql"},
			},
		},
		{
			Name:        "jira_get_issue",
			Description: "Get a specific issue by key from a workspace. You can query different workspaces in the same chat by specifying different workspace_id values.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID to query (e.g., 'workspace-1', 'providentia'). Use list_workspaces to see available workspaces.",
					},
					"issue_key": map[string]interface{}{
						"type":        "string",
						"description": "Issue key (e.g., PROJ-123)",
					},
					"expand": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Fields to expand",
					},
				},
				"required": []string{"workspace_id", "issue_key"},
			},
		},
		{
			Name:        "jira_create_issue",
			Description: "Create a new issue",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"project_key": map[string]interface{}{
						"type":        "string",
						"description": "Project key",
					},
					"issue_type": map[string]interface{}{
						"type":        "string",
						"description": "Issue type (e.g., Bug, Story, Task)",
					},
					"summary": map[string]interface{}{
						"type":        "string",
						"description": "Issue summary",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Issue description",
					},
					"additional_fields": map[string]interface{}{
						"type":        "object",
						"description": "Additional fields to set",
					},
				},
				"required": []string{"workspace_id", "project_key", "issue_type", "summary"},
			},
		},
		{
			Name:        "jira_update_issue",
			Description: "Update an existing issue",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"issue_key": map[string]interface{}{
						"type":        "string",
						"description": "Issue key",
					},
					"fields": map[string]interface{}{
						"type":        "object",
						"description": "Fields to update",
					},
				},
				"required": []string{"workspace_id", "issue_key", "fields"},
			},
		},
		{
			Name:        "jira_add_comment",
			Description: "Add a comment to an issue",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"issue_key": map[string]interface{}{
						"type":        "string",
						"description": "Issue key",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Comment body",
					},
				},
				"required": []string{"workspace_id", "issue_key", "body"},
			},
		},
		{
			Name:        "jira_transition_issue",
			Description: "Transition an issue to a different status",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"issue_key": map[string]interface{}{
						"type":        "string",
						"description": "Issue key",
					},
					"transition_id": map[string]interface{}{
						"type":        "string",
						"description": "Transition ID",
					},
				},
				"required": []string{"workspace_id", "issue_key", "transition_id"},
			},
		},
	}
}

// HandleTool handles a Jira tool call
func (h *JiraHandler) HandleTool(call mcp.ToolCall, userID string) (mcp.ToolResult, error) {
	workspaceID, ok := call.Arguments["workspace_id"].(string)
	if !ok {
		return mcp.ToolResult{
			Content: []mcp.ContentBlock{
				{Type: "text", Text: "Error: workspace_id is required"},
			},
			IsError: true,
		}, fmt.Errorf("workspace_id is required")
	}

	req := models.JiraRequest{
		Action:      getJiraActionFromToolName(call.Name),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Params:      call.Arguments,
		RequestID:   fmt.Sprintf("req_%d", atomic.AddInt64(&requestIDCounter, 1)),
	}

	resp, err := h.callService(req)
	if err != nil {
		return mcp.ToolResult{
			Content: []mcp.ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, err
	}

	if !resp.Success {
		errorMsg := "Unknown error"
		if resp.Error != nil {
			errorMsg = resp.Error.Message
		}
		return mcp.ToolResult{
			Content: []mcp.ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Error: %s", errorMsg)},
			},
			IsError: true,
		}, fmt.Errorf(errorMsg)
	}

	// Convert response to JSON string
	resultJSON, _ := json.MarshalIndent(resp.Data, "", "  ")

	return mcp.ToolResult{
		Content: []mcp.ContentBlock{
			{Type: "text", Text: string(resultJSON)},
		},
	}, nil
}

func getJiraActionFromToolName(toolName string) string {
	switch toolName {
	case "jira_list_projects":
		return "list_projects"
	case "jira_list_issues":
		return "list_issues"
	case "jira_get_issue":
		return "get_issue"
	case "jira_create_issue":
		return "create_issue"
	case "jira_update_issue":
		return "update_issue"
	case "jira_add_comment":
		return "add_comment"
	case "jira_transition_issue":
		return "transition_issue"
	default:
		return ""
	}
}

