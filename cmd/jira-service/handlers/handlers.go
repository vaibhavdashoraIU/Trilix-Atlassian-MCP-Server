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
	case "get_agile_boards":
		response = s.handleGetAgileBoards(client, req)
	case "get_board_issues":
		response = s.handleGetBoardIssues(client, req)
	case "get_sprints_from_board":
		response = s.handleGetSprintsFromBoard(client, req)
	case "get_sprint_issues":
		response = s.handleGetSprintIssues(client, req)
	case "create_sprint":
		response = s.handleCreateSprint(client, req)
	case "update_sprint":
		response = s.handleUpdateSprint(client, req)
	case "get_worklog":
		response = s.handleGetWorklog(client, req)
	case "add_worklog":
		response = s.handleAddWorklog(client, req)
	case "get_transitions":
		response = s.handleGetTransitions(client, req)
	case "delete_issue":
		response = s.handleDeleteIssue(client, req)
	case "get_project_issues":
		response = s.handleGetProjectIssues(client, req)
	case "get_project_versions":
		response = s.handleGetProjectVersions(client, req)
	case "search_users":
		response = s.handleSearchUsers(client, req)
	case "get_user_profile":
		response = s.handleGetUserProfile(client, req)
	case "search_fields":
		response = s.handleSearchFields(client, req)
	case "create_issue_link":
		response = s.handleCreateIssueLink(client, req)
	case "remove_issue_link":
		response = s.handleRemoveIssueLink(client, req)
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

func (s *Service) handleGetAgileBoards(client *api.Client, req models.JiraRequest) map[string]interface{} {
	projectKey, _ := req.Params["project_key"].(string)
	boardType, _ := req.Params["type"].(string)

	boards, err := client.GetAgileBoards(projectKey, boardType)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(boards, req.RequestID)
}

func (s *Service) handleGetBoardIssues(client *api.Client, req models.JiraRequest) map[string]interface{} {
	boardID, ok := req.Params["board_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing board_id", req.RequestID)
	}

	limit := 50
	if l, ok := req.Params["limit"].(float64); ok {
		limit = int(l)
	}

	issues, err := client.GetBoardIssues(boardID, limit)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(issues, req.RequestID)
}

func (s *Service) handleGetSprintsFromBoard(client *api.Client, req models.JiraRequest) map[string]interface{} {
	boardID, ok := req.Params["board_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing board_id", req.RequestID)
	}

	state, _ := req.Params["state"].(string)

	sprints, err := client.GetSprintsFromBoard(boardID, state)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(sprints, req.RequestID)
}

func (s *Service) handleGetSprintIssues(client *api.Client, req models.JiraRequest) map[string]interface{} {
	sprintID, ok := req.Params["sprint_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing sprint_id", req.RequestID)
	}

	limit := 50
	if l, ok := req.Params["limit"].(float64); ok {
		limit = int(l)
	}

	issues, err := client.GetSprintIssues(sprintID, limit)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(issues, req.RequestID)
}

func (s *Service) handleCreateSprint(client *api.Client, req models.JiraRequest) map[string]interface{} {
	boardID, ok := req.Params["board_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing board_id", req.RequestID)
	}

	name, ok := req.Params["name"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing name", req.RequestID)
	}

	startDate, _ := req.Params["start_date"].(string)
	endDate, _ := req.Params["end_date"].(string)

	sprint, err := client.CreateSprint(boardID, name, startDate, endDate)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(sprint, req.RequestID)
}

func (s *Service) handleUpdateSprint(client *api.Client, req models.JiraRequest) map[string]interface{} {
	sprintID, ok := req.Params["sprint_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing sprint_id", req.RequestID)
	}

	name, _ := req.Params["name"].(string)
	state, _ := req.Params["state"].(string)
	startDate, _ := req.Params["start_date"].(string)
	endDate, _ := req.Params["end_date"].(string)

	sprint, err := client.UpdateSprint(sprintID, name, state, startDate, endDate)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(sprint, req.RequestID)
}

func (s *Service) handleGetWorklog(client *api.Client, req models.JiraRequest) map[string]interface{} {
	issueKey, ok := req.Params["issue_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing issue_key", req.RequestID)
	}

	worklogs, err := client.GetWorklog(issueKey)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(worklogs, req.RequestID)
}

func (s *Service) handleAddWorklog(client *api.Client, req models.JiraRequest) map[string]interface{} {
	issueKey, ok := req.Params["issue_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing issue_key", req.RequestID)
	}

	timeSpent, ok := req.Params["time_spent"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing time_spent", req.RequestID)
	}

	comment, _ := req.Params["comment"].(string)
	started, _ := req.Params["started"].(string)

	worklog, err := client.AddWorklog(issueKey, timeSpent, comment, started)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(worklog, req.RequestID)
}

func (s *Service) handleGetTransitions(client *api.Client, req models.JiraRequest) map[string]interface{} {
	issueKey, ok := req.Params["issue_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing issue_key", req.RequestID)
	}

	transitions, err := client.GetTransitions(issueKey)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(transitions, req.RequestID)
}

func (s *Service) handleDeleteIssue(client *api.Client, req models.JiraRequest) map[string]interface{} {
	issueKey, ok := req.Params["issue_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing issue_key", req.RequestID)
	}

	err := client.DeleteIssue(issueKey)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Issue %s deleted successfully", issueKey),
	}, req.RequestID)
}

func (s *Service) handleGetProjectIssues(client *api.Client, req models.JiraRequest) map[string]interface{} {
	projectKey, ok := req.Params["project_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing project_key", req.RequestID)
	}

	limit := 100
	if l, ok := req.Params["limit"].(float64); ok {
		limit = int(l)
	}

	issues, err := client.GetProjectIssues(projectKey, limit)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(issues, req.RequestID)
}

func (s *Service) handleGetProjectVersions(client *api.Client, req models.JiraRequest) map[string]interface{} {
	projectKey, ok := req.Params["project_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing project_key", req.RequestID)
	}

	versions, err := client.GetProjectVersions(projectKey)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(versions, req.RequestID)
}

func (s *Service) handleSearchUsers(client *api.Client, req models.JiraRequest) map[string]interface{} {
	query, ok := req.Params["query"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing query", req.RequestID)
	}

	users, err := client.SearchUsers(query)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(users, req.RequestID)
}

func (s *Service) handleGetUserProfile(client *api.Client, req models.JiraRequest) map[string]interface{} {
	accountID, ok := req.Params["account_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing account_id", req.RequestID)
	}

	user, err := client.GetUserProfile(accountID)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(user, req.RequestID)
}

func (s *Service) handleSearchFields(client *api.Client, req models.JiraRequest) map[string]interface{} {
	fields, err := client.SearchFields()
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(fields, req.RequestID)
}

func (s *Service) handleCreateIssueLink(client *api.Client, req models.JiraRequest) map[string]interface{} {
	typeName, ok := req.Params["type"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing type", req.RequestID)
	}

	inwardKey, ok := req.Params["inward_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing inward_key", req.RequestID)
	}

	outwardKey, ok := req.Params["outward_key"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing outward_key", req.RequestID)
	}

	err := client.CreateIssueLink(typeName, inwardKey, outwardKey)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(map[string]interface{}{"success": true}, req.RequestID)
}

func (s *Service) handleRemoveIssueLink(client *api.Client, req models.JiraRequest) map[string]interface{} {
	linkID, ok := req.Params["link_id"].(string)
	if !ok {
		return models.ErrorResponse(models.ErrCodeInvalidRequest, "missing link_id", req.RequestID)
	}

	err := client.RemoveIssueLink(linkID)
	if err != nil {
		return models.ErrorResponse(models.ErrCodeAPIError, err.Error(), req.RequestID)
	}

	return models.SuccessResponse(map[string]interface{}{"success": true}, req.RequestID)
}

