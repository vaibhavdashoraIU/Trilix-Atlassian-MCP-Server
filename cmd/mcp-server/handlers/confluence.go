package handlers

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
	"github.com/providentiaww/trilix-atlassian-mcp/pkg/mcp"
)

var requestIDCounter int64

// ConfluenceHandler handles Confluence-related MCP tool calls
type ConfluenceHandler struct {
	callService func(models.ConfluenceRequest) (*models.ConfluenceResponse, error)
}

// NewConfluenceHandler creates a new Confluence handler
func NewConfluenceHandler(callService func(models.ConfluenceRequest) (*models.ConfluenceResponse, error)) *ConfluenceHandler {
	return &ConfluenceHandler{
		callService: callService,
	}
}

// ListTools returns the list of Confluence tools
func (h *ConfluenceHandler) ListTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "confluence_get_page",
			Description: "Retrieve a Confluence page by ID from a specific workspace. You can query different workspaces in the same chat by specifying different workspace_id values.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID to query (e.g., 'workspace-1', 'providentia'). Use list_workspaces to see available workspaces.",
					},
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "Confluence page ID",
					},
				},
				"required": []string{"workspace_id", "page_id"},
			},
		},
		{
			Name:        "confluence_search",
			Description: "Search for content in Confluence using CQL. Supports querying multiple workspaces - specify workspace_id to search a specific organization.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID to search (e.g., 'workspace-1', 'providentia'). Use list_workspaces to see available workspaces.",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "CQL search query",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results",
						"default":     10,
					},
				},
				"required": []string{"workspace_id", "query"},
			},
		},
		{
			Name:        "confluence_create_page",
			Description: "Create a new page in Confluence",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"space_key": map[string]interface{}{
						"type":        "string",
						"description": "Space key",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Page title",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Page body (storage format)",
					},
					"parent_id": map[string]interface{}{
						"type":        "string",
						"description": "Optional parent page ID",
					},
				},
				"required": []string{"workspace_id", "space_key", "title", "body"},
			},
		},
		{
			Name:        "confluence_copy_page",
			Description: "Copy a page from one workspace to another",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"src_workspace": map[string]interface{}{
						"type":        "string",
						"description": "Source workspace ID",
					},
					"dst_workspace": map[string]interface{}{
						"type":        "string",
						"description": "Destination workspace ID",
					},
					"src_page_id": map[string]interface{}{
						"type":        "string",
						"description": "Source page ID",
					},
					"dst_space_key": map[string]interface{}{
						"type":        "string",
						"description": "Destination space key",
					},
					"dst_parent_id": map[string]interface{}{
						"type":        "string",
						"description": "Optional parent page ID in destination",
					},
				},
				"required": []string{"src_workspace", "dst_workspace", "src_page_id", "dst_space_key"},
			},
		},
		{
			Name:        "confluence_list_spaces",
			Description: "List all spaces in a workspace",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results",
						"default":     50,
					},
				},
				"required": []string{"workspace_id"},
			},
		},
		{
			Name:        "confluence_update_page",
			Description: "Update an existing Confluence page",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "Page ID to update",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "New page title (optional)",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "New page body content (storage format)",
					},
				},
				"required": []string{"workspace_id", "page_id", "body"},
			},
		},
		{
			Name:        "confluence_delete_page",
			Description: "Delete a Confluence page",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "Page ID to delete",
					},
				},
				"required": []string{"workspace_id", "page_id"},
			},
		},
		{
			Name:        "confluence_get_page_children",
			Description: "Get child pages of a Confluence page",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "Parent page ID",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results",
						"default":     25,
					},
				},
				"required": []string{"workspace_id", "page_id"},
			},
		},
		{
			Name:        "confluence_add_comment",
			Description: "Add a comment to a Confluence page",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "Page ID to comment on",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Comment body (storage format)",
					},
				},
				"required": []string{"workspace_id", "page_id", "body"},
			},
		},
		{
			Name:        "confluence_get_comments",
			Description: "Get comments for a Confluence page",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "Page ID",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results",
						"default":     25,
					},
				},
				"required": []string{"workspace_id", "page_id"},
			},
		},
		{
			Name:        "confluence_add_label",
			Description: "Add a label to a Confluence page",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "Page ID",
					},
					"label": map[string]interface{}{
						"type":        "string",
						"description": "Label name to add",
					},
				},
				"required": []string{"workspace_id", "page_id", "label"},
			},
		},
		{
			Name:        "confluence_get_labels",
			Description: "Get labels for a Confluence page",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "Page ID",
					},
				},
				"required": []string{"workspace_id", "page_id"},
			},
		},
		{
			Name:        "confluence_search_user",
			Description: "Search for Confluence users by name or email",
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
			Name:        "confluence_get_space",
			Description: "Get detailed information about a specific Confluence space",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"space_key": map[string]interface{}{
						"type":        "string",
						"description": "Space key",
					},
				},
				"required": []string{"workspace_id", "space_key"},
			},
		},
		{
			Name:        "confluence_get_attachments",
			Description: "Get attachments for a specific Confluence page",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "Workspace ID",
					},
					"page_id": map[string]interface{}{
						"type":        "string",
						"description": "Page ID",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results",
						"default":     25,
					},
				},
				"required": []string{"workspace_id", "page_id"},
			},
		},
	}
}

// HandleTool handles a Confluence tool call
func (h *ConfluenceHandler) HandleTool(call mcp.ToolCall, userID string) (mcp.ToolResult, error) {
	workspaceID, ok := call.Arguments["workspace_id"].(string)
	if !ok {
		return mcp.ToolResult{
			Content: []mcp.ContentBlock{
				{Type: "text", Text: "Error: workspace_id is required"},
			},
			IsError: true,
		}, fmt.Errorf("workspace_id is required")
	}

	req := models.ConfluenceRequest{
		Action:      getActionFromToolName(call.Name),
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

func getActionFromToolName(toolName string) string {
	switch toolName {
	case "confluence_get_page":
		return "get_page"
	case "confluence_search":
		return "search"
	case "confluence_create_page":
		return "create_page"
	case "confluence_copy_page":
		return "copy_page"
	case "confluence_list_spaces":
		return "list_spaces"
	case "confluence_update_page":
		return "update_page"
	case "confluence_delete_page":
		return "delete_page"
	case "confluence_get_page_children":
		return "get_page_children"
	case "confluence_add_comment":
		return "add_comment"
	case "confluence_get_comments":
		return "get_comments"
	case "confluence_add_label":
		return "add_label"
	case "confluence_get_labels":
		return "get_labels"
	case "confluence_search_user":
		return "search_user"
	case "confluence_get_space":
		return "get_space"
	case "confluence_get_attachments":
		return "get_attachments"
	default:
		return ""
	}
}


