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
		{
			Name:        "jira_get_agile_boards",
			Description: "List all agile boards in a workspace",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"project_key": map[string]interface{}{
						"type":        "string",
						"description": "Optional project key to filter boards",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Board type filter (scrum, kanban, simple)",
					},
				},
				"required": []string{"workspace_id"},
			},
		},
		{
			Name:        "jira_get_board_issues",
			Description: "Get issues on a specific agile board",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"board_id": map[string]interface{}{
						"type":        "string",
						"description": "Board ID",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results",
						"default":     50,
					},
				},
				"required": []string{"workspace_id", "board_id"},
			},
		},
		{
			Name:        "jira_get_sprints_from_board",
			Description: "List sprints for a specific board",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"board_id": map[string]interface{}{
						"type":        "string",
						"description": "Board ID",
					},
					"state": map[string]interface{}{
						"type":        "string",
						"description": "Sprint state filter (active, future, closed)",
					},
				},
				"required": []string{"workspace_id", "board_id"},
			},
		},
		{
			Name:        "jira_get_sprint_issues",
			Description: "Get issues in a specific sprint",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"sprint_id": map[string]interface{}{
						"type":        "string",
						"description": "Sprint ID",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results",
						"default":     50,
					},
				},
				"required": []string{"workspace_id", "sprint_id"},
			},
		},
		{
			Name:        "jira_create_sprint",
			Description: "Create a new sprint on a board",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"board_id": map[string]interface{}{
						"type":        "string",
						"description": "Board ID",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Sprint name",
					},
					"start_date": map[string]interface{}{
						"type":        "string",
						"description": "Start date (ISO 8601 format)",
					},
					"end_date": map[string]interface{}{
						"type":        "string",
						"description": "End date (ISO 8601 format)",
					},
				},
				"required": []string{"workspace_id", "board_id", "name"},
			},
		},
		{
			Name:        "jira_update_sprint",
			Description: "Update an existing sprint",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"sprint_id": map[string]interface{}{
						"type":        "string",
						"description": "Sprint ID",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Sprint name",
					},
					"state": map[string]interface{}{
						"type":        "string",
						"description": "Sprint state (active, closed)",
					},
					"start_date": map[string]interface{}{
						"type":        "string",
						"description": "Start date (ISO 8601 format)",
					},
					"end_date": map[string]interface{}{
						"type":        "string",
						"description": "End date (ISO 8601 format)",
					},
				},
				"required": []string{"workspace_id", "sprint_id"},
			},
		},
		{
			Name:        "jira_get_worklog",
			Description: "Get worklog (time tracking) entries for an issue",
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
				},
				"required": []string{"workspace_id", "issue_key"},
			},
		},
		{
			Name:        "jira_add_worklog",
			Description: "Add a worklog (time tracking) entry to an issue",
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
					"time_spent": map[string]interface{}{
						"type":        "string",
						"description": "Time spent (e.g., '3h 30m', '2d', '1w 2d 3h')",
					},
					"comment": map[string]interface{}{
						"type":        "string",
						"description": "Optional comment for the worklog entry",
					},
					"started": map[string]interface{}{
						"type":        "string",
						"description": "Optional start time (ISO 8601 format)",
					},
				},
				"required": []string{"workspace_id", "issue_key", "time_spent"},
			},
		},
		{
			Name:        "jira_get_transitions",
			Description: "Get available transitions for an issue",
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
				},
				"required": []string{"workspace_id", "issue_key"},
			},
		},
		{
			Name:        "jira_delete_issue",
			Description: "Delete an issue",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"issue_key": map[string]interface{}{
						"type":        "string",
						"description": "Issue key to delete",
					},
				},
				"required": []string{"workspace_id", "issue_key"},
			},
		},
		{
			Name:        "jira_get_project_issues",
			Description: "Get all issues in a project",
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
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results",
						"default":     100,
					},
				},
				"required": []string{"workspace_id", "project_key"},
			},
		},
		{
			Name:        "jira_get_project_versions",
			Description: "List versions for a project",
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
				},
				"required": []string{"workspace_id", "project_key"},
			},
		},
		{
			Name:        "jira_search_users",
			Description: "Search for Jira users by name or email",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "User name or email to search for",
					},
				},
				"required": []string{"workspace_id", "query"},
			},
		},
		{
			Name:        "jira_get_user_profile",
			Description: "Get detailed profile information about a Jira user",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"account_id": map[string]interface{}{
						"type":        "string",
						"description": "User account ID",
					},
				},
				"required": []string{"workspace_id", "account_id"},
			},
		},
		{
			Name:        "jira_search_fields",
			Description: "List all available fields in the Jira workspace",
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
			Name:        "jira_create_issue_link",
			Description: "Create a link between two Jira issues (e.g., 'Blocks', 'Relates to')",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Link type name (e.g., 'Blocks', 'Relates', 'Duplicate')",
					},
					"inward_key": map[string]interface{}{
						"type":        "string",
						"description": "Key of the inward issue",
					},
					"outward_key": map[string]interface{}{
						"type":        "string",
						"description": "Key of the outward issue",
					},
				},
				"required": []string{"workspace_id", "type", "inward_key", "outward_key"},
			},
		},
		{
			Name:        "jira_remove_issue_link",
			Description: "Remove an existing link between Jira issues",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"link_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the issue link to remove",
					},
				},
				"required": []string{"workspace_id", "link_id"},
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
	case "jira_get_agile_boards":
		return "get_agile_boards"
	case "jira_get_board_issues":
		return "get_board_issues"
	case "jira_get_sprints_from_board":
		return "get_sprints_from_board"
	case "jira_get_sprint_issues":
		return "get_sprint_issues"
	case "jira_create_sprint":
		return "create_sprint"
	case "jira_update_sprint":
		return "update_sprint"
	case "jira_get_worklog":
		return "get_worklog"
	case "jira_add_worklog":
		return "add_worklog"
	case "jira_get_transitions":
		return "get_transitions"
	case "jira_delete_issue":
		return "delete_issue"
	case "jira_get_project_issues":
		return "get_project_issues"
	case "jira_get_project_versions":
		return "get_project_versions"
	case "jira_search_users":
		return "search_users"
	case "jira_get_user_profile":
		return "get_user_profile"
	case "jira_search_fields":
		return "search_fields"
	case "jira_create_issue_link":
		return "create_issue_link"
	case "jira_remove_issue_link":
		return "remove_issue_link"
	default:
		return ""
	}
}

