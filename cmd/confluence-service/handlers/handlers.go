package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/providentiaww/trilix-atlassian-mcp/cmd/confluence-service/api"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/cache"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/storage"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Service handles Confluence service requests
type Service struct {
	credStore  storage.CredentialStoreInterface
	apiTimeout time.Duration
	cache      *cache.SimpleCache
}

// NewService creates a new Confluence service
func NewService(credStore storage.CredentialStoreInterface, timeout time.Duration) *Service {
	return &Service{
		credStore:  credStore,
		apiTimeout: timeout,
		cache:      cache.NewSimpleCache(),
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
	case "update_page":
		response = s.handleUpdatePage(client, req)
	case "delete_page":
		response = s.handleDeletePage(client, req)
	case "search":
		response = s.handleSearch(client, req)
	case "list_spaces":
		response = s.handleListSpaces(client, req)
	case "get_space":
		response = s.handleGetSpace(client, req)
	case "copy_page":
		response = s.handleCopyPage(req)
	case "get_page_children":
		response = s.handleGetPageChildren(client, req)
	case "add_comment":
		response = s.handleAddComment(client, req)
	case "get_comments":
		response = s.handleGetComments(client, req)
	case "add_label":
		response = s.handleAddLabel(client, req)
	case "get_labels":
		response = s.handleGetLabels(client, req)
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
	// Check cache first
	cacheKey := fmt.Sprintf("spaces:%s:%s", req.UserID, req.WorkspaceID)
	if cached, found := s.cache.Get(cacheKey); found {
		if cachedData, ok := cached.(map[string]interface{}); ok {
			return cachedData
		}
	}

	// Cache miss - fetch from API
	limit := 50
	if l, ok := req.Params["limit"].(float64); ok {
		limit = int(l)
	}

	spaces, err := client.ListSpaces(limit)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	response := models.SuccessResponse(spaces, req.RequestID)

	// Cache for 2 minutes
	s.cache.Set(cacheKey, response, 2*time.Minute)

	return response
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

func (s *Service) handleUpdatePage(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	pageID, ok := req.Params["page_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing page_id", req.RequestID)
	}

	body, ok := req.Params["body"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing body", req.RequestID)
	}

	// Get current page to retrieve version
	currentPage, err := client.GetPage(pageID)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	// Use provided title or keep existing
	title := currentPage.Title
	if t, ok := req.Params["title"].(string); ok && t != "" {
		title = t
	}

	updatedPage, err := client.UpdatePage(pageID, title, body, currentPage.Version.Number+1)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(updatedPage, req.RequestID)
}

func (s *Service) handleDeletePage(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	pageID, ok := req.Params["page_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing page_id", req.RequestID)
	}

	err := client.DeletePage(pageID)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Page %s deleted successfully", pageID),
	}, req.RequestID)
}

func (s *Service) handleGetPageChildren(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	pageID, ok := req.Params["page_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing page_id", req.RequestID)
	}

	limit := 25
	if l, ok := req.Params["limit"].(float64); ok {
		limit = int(l)
	}

	children, err := client.GetPageChildren(pageID, limit)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(children, req.RequestID)
}

func (s *Service) handleAddComment(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	pageID, ok := req.Params["page_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing page_id", req.RequestID)
	}

	body, ok := req.Params["body"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing body", req.RequestID)
	}

	comment, err := client.AddComment(pageID, body)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(comment, req.RequestID)
}

func (s *Service) handleGetComments(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	pageID, ok := req.Params["page_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing page_id", req.RequestID)
	}

	limit := 25
	if l, ok := req.Params["limit"].(float64); ok {
		limit = int(l)
	}

	comments, err := client.GetComments(pageID, limit)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(comments, req.RequestID)
}

func (s *Service) handleAddLabel(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	pageID, ok := req.Params["page_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing page_id", req.RequestID)
	}

	label, ok := req.Params["label"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing label", req.RequestID)
	}

	result, err := client.AddLabel(pageID, label)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(result, req.RequestID)
}

func (s *Service) handleGetLabels(client *api.Client, req models.ConfluenceRequest) map[string]interface{} {
	pageID, ok := req.Params["page_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing page_id", req.RequestID)
	}

	labels, err := client.GetLabels(pageID)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(labels, req.RequestID)
}

