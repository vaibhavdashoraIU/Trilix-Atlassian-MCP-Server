package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/providentiaww/trilix-atlassian-mcp/cmd/confluence-service/api"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/storage"
	amqp "github.com/rabbitmq/amqp091-go"
	"time"
)

// Service handles Confluence service requests
type Service struct {
	credStore  storage.CredentialStoreInterface
	apiTimeout time.Duration
}

// NewService creates a new Confluence service
func NewService(credStore storage.CredentialStoreInterface, timeout time.Duration) *Service {
	return &Service{
		credStore:  credStore,
		apiTimeout: timeout,
	}
}

// HandleRequest processes incoming RabbitMQ messages
func (s *Service) HandleRequest(d amqp.Delivery) []byte {
	var req models.ConfluenceRequest
	if err := json.Unmarshal(d.Body, &req); err != nil {
		response := models.ErrorResponse(models.ErrCodeInvalidRequest, err.Error(), req.RequestID)
		responseBytes, _ := json.Marshal(response)
		return responseBytes
	}

	// Get credentials for the workspace
	creds, err := s.credStore.GetCredentials(req.UserID, req.WorkspaceID)
	if err != nil {
		response := models.ErrorResponse(models.ErrCodeAuthFailed,
			fmt.Sprintf("workspace not found: %s", req.WorkspaceID), req.RequestID)
		responseBytes, _ := json.Marshal(response)
		return responseBytes
	}

	// Ensure Site URL includes /wiki for Confluence API
	site := creds.Site
	if site != "" && !strings.HasSuffix(site, "/wiki") {
		// Remove trailing slash if present, then add /wiki
		site = strings.TrimSuffix(site, "/")
		site += "/wiki"
	}

	// Create API client
	client := api.NewClient(api.WorkspaceCredentials{
		Site:  site,
		Email: creds.Email,
		Token: creds.Token,
	}, s.apiTimeout)

	// Route to appropriate handler
	var response map[string]interface{}
	switch req.Action {
	case "get_page":
		response = s.handleGetPage(client, req)
	case "create_page":
		response = s.handleCreatePage(client, req)
	case "search":
		response = s.handleSearch(client, req)
	case "list_spaces":
		response = s.handleListSpaces(client, req)
	case "get_space":
		response = s.handleGetSpace(client, req)
	case "copy_page":
		response = s.handleCopyPage(req)
	default:
		response = models.ErrorResponse(models.ErrCodeInvalidRequest,
			fmt.Sprintf("unknown action: %s", req.Action), req.RequestID)
	}

	responseBytes, _ := json.Marshal(response)
	return responseBytes
}

func (s *Service) handleGetPage(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	pageID, ok := req.Params["page_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing page_id", req.RequestID)
	}

	page, err := client.GetPage(pageID)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(page, req.RequestID)
}

func (s *Service) handleCreatePage(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	spaceKey, ok := req.Params["space_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing space_key", req.RequestID)
	}

	title, ok := req.Params["title"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing title", req.RequestID)
	}

	body, ok := req.Params["body"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing body", req.RequestID)
	}

	var parentID *string
	if pid, ok := req.Params["parent_id"].(string); ok && pid != "" {
		parentID = &pid
	}

	page, err := client.CreatePage(spaceKey, title, body, parentID)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(page, req.RequestID)
}

func (s *Service) handleSearch(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	query, ok := req.Params["query"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing query", req.RequestID)
	}

	limit := 10
	if l, ok := req.Params["limit"].(float64); ok {
		limit = int(l)
	}

	results, err := client.SearchPages(query, limit)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(results, req.RequestID)
}

func (s *Service) handleListSpaces(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	limit := 50
	if l, ok := req.Params["limit"].(float64); ok {
		limit = int(l)
	}

	spaces, err := client.ListSpaces(limit)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(spaces, req.RequestID)
}

func (s *Service) handleGetSpace(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	spaceKey, ok := req.Params["space_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing space_key", req.RequestID)
	}

	space, err := client.GetSpace(spaceKey)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(space, req.RequestID)
}

func (s *Service) handleCopyPage(req models.ConfluenceRequest) map[string]interface{} {
	srcWorkspace, ok := req.Params["src_workspace"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing src_workspace", req.RequestID)
	}

	dstWorkspace, ok := req.Params["dst_workspace"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing dst_workspace", req.RequestID)
	}

	srcPageID, ok := req.Params["src_page_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing src_page_id", req.RequestID)
	}

	dstSpaceKey, ok := req.Params["dst_space_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing dst_space_key", req.RequestID)
	}

	var dstParentID *string
	if pid, ok := req.Params["dst_parent_id"].(string); ok && pid != "" {
		dstParentID = &pid
	}

	// Get credentials for both workspaces
	srcCreds, err := s.credStore.GetCredentials(req.UserID, srcWorkspace)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAuthFailed,
			fmt.Sprintf("source workspace not found: %s", srcWorkspace), req.RequestID)
	}

	dstCreds, err := s.credStore.GetCredentials(req.UserID, dstWorkspace)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAuthFailed,
			fmt.Sprintf("destination workspace not found: %s", dstWorkspace), req.RequestID)
	}

	// Create clients for both workspaces
	srcClient := api.NewClient(api.WorkspaceCredentials{
		Site:  srcCreds.Site,
		Email: srcCreds.Email,
		Token: srcCreds.Token,
	}, s.apiTimeout)

	dstClient := api.NewClient(api.WorkspaceCredentials{
		Site:  dstCreds.Site,
		Email: dstCreds.Email,
		Token: dstCreds.Token,
	}, s.apiTimeout)

	// Read from source
	page, err := srcClient.GetPage(srcPageID)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	// Create in destination
	newPage, err := dstClient.CreatePage(dstSpaceKey, page.Title, page.Body.Storage.Value, dstParentID)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(newPage, req.RequestID)
}

