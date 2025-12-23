package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/providentiaww/trilix-atlassian-mcp/cmd/jira-service/api"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/storage"
	amqp "github.com/rabbitmq/amqp091-go"
	"time"
)

// Service handles Jira service requests
type Service struct {
	credStore  storage.CredentialStoreInterface
	apiTimeout time.Duration
}

// NewService creates a new Jira service
func NewService(credStore storage.CredentialStoreInterface, timeout time.Duration) *Service {
	return &Service{
		credStore:  credStore,
		apiTimeout: timeout,
	}
}

// HandleRequest processes incoming RabbitMQ messages
func (s *Service) HandleRequest(d amqp.Delivery) []byte {
	var req models.JiraRequest
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

	// Create API client
	client := api.NewClient(api.WorkspaceCredentials{
		Site:  creds.Site,
		Email: creds.Email,
		Token: creds.Token,
	}, s.apiTimeout)

	// Route to appropriate handler
	var response map[string]interface{}
	switch req.Action {
	case "list_issues":
		response = s.handleListIssues(client, req)
	case "get_issue":
		response = s.handleGetIssue(client, req)
	case "create_issue":
		response = s.handleCreateIssue(client, req)
	case "update_issue":
		response = s.handleUpdateIssue(client, req)
	case "add_comment":
		response = s.handleAddComment(client, req)
	case "transition_issue":
		response = s.handleTransitionIssue(client, req)
	case "list_projects":
		response = s.handleListProjects(client, req)
	default:
		response = models.ErrorResponse(models.ErrCodeInvalidRequest,
			fmt.Sprintf("unknown action: %s", req.Action), req.RequestID)
	}

	responseBytes, _ := json.Marshal(response)
	return responseBytes
}

func (s *Service) handleListIssues(client *api.Client, req models.JiraRequest) map[string]interface{} {
	jql, ok := req.Params["jql"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing jql", req.RequestID)
	}

	limit := 50
	if l, ok := req.Params["limit"].(float64); ok {
		limit = int(l)
	}

	var fields []string
	if f, ok := req.Params["fields"].([]interface{}); ok {
		fields = make([]string, len(f))
		for i, v := range f {
			fields[i] = v.(string)
		}
	}

	results, err := client.SearchIssues(jql, fields, limit)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(results, req.RequestID)
}

func (s *Service) handleGetIssue(client *api.Client, req models.JiraRequest) map[string]interface{} {
	issueKey, ok := req.Params["issue_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing issue_key", req.RequestID)
	}

	var expand []string
	if e, ok := req.Params["expand"].([]interface{}); ok {
		expand = make([]string, len(e))
		for i, v := range e {
			expand[i] = v.(string)
		}
	}

	issue, err := client.GetIssue(issueKey, expand)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(issue, req.RequestID)
}

func (s *Service) handleCreateIssue(client *api.Client, req models.JiraRequest) map[string]interface{} {
	projectKey, ok := req.Params["project_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing project_key", req.RequestID)
	}

	issueType, ok := req.Params["issue_type"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing issue_type", req.RequestID)
	}

	summary, ok := req.Params["summary"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing summary", req.RequestID)
	}

	description := ""
	if d, ok := req.Params["description"].(string); ok {
		description = d
	}

	// Additional fields
	additionalFields := make(map[string]interface{})
	if af, ok := req.Params["additional_fields"].(map[string]interface{}); ok {
		additionalFields = af
	}

	issue, err := client.CreateIssue(projectKey, issueType, summary, description, additionalFields)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(issue, req.RequestID)
}

func (s *Service) handleUpdateIssue(client *api.Client, req models.JiraRequest) map[string]interface{} {
	issueKey, ok := req.Params["issue_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing issue_key", req.RequestID)
	}

	fields, ok := req.Params["fields"].(map[string]interface{})
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing fields", req.RequestID)
	}

	err := client.UpdateIssue(issueKey, fields)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(map[string]string{"status": "updated"}, req.RequestID)
}

func (s *Service) handleAddComment(client *api.Client, req models.JiraRequest) map[string]interface{} {
	issueKey, ok := req.Params["issue_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing issue_key", req.RequestID)
	}

	body, ok := req.Params["body"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing body", req.RequestID)
	}

	comment, err := client.AddComment(issueKey, body)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(comment, req.RequestID)
}

func (s *Service) handleTransitionIssue(client *api.Client, req models.JiraRequest) map[string]interface{} {
	issueKey, ok := req.Params["issue_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing issue_key", req.RequestID)
	}

	transitionID, ok := req.Params["transition_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing transition_id", req.RequestID)
	}

	err := client.TransitionIssue(issueKey, transitionID)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(map[string]string{"status": "transitioned"}, req.RequestID)
}

func (s *Service) handleListProjects(client *api.Client, req models.JiraRequest) map[string]interface{} {
	projects, err := client.ListProjects()
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(projects, req.RequestID)
}

