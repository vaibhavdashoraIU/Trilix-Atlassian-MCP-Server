package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
)

// WorkspaceCredentials holds connection info for one Atlassian instance
type WorkspaceCredentials struct {
	Site  string // e.g., "https://eso.atlassian.net"
	Email string // e.g., "service@eso.com"
	Token string // Atlassian API token
}

// Client wraps HTTP client with Atlassian auth
type Client struct {
	creds      WorkspaceCredentials
	httpClient *http.Client
}

// Shared HTTP client with connection pooling
var sharedHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	},
}

// NewClient creates an authenticated Jira client
func NewClient(creds WorkspaceCredentials, timeout time.Duration) *Client {
	// Use a dedicated client if a specific timeout is requested, 
	// otherwise use the shared one.
	client := sharedHTTPClient
	if timeout > 0 && timeout != sharedHTTPClient.Timeout {
		client = &http.Client{
			Timeout:   timeout,
			Transport: sharedHTTPClient.Transport,
		}
	}

	return &Client{
		creds:      creds,
		httpClient: client,
	}
}

// authHeader returns the Basic auth header value
func (c *Client) authHeader() string {
	credentials := fmt.Sprintf("%s:%s", c.creds.Email, c.creds.Token)
	encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
	return "Basic " + encoded
}

// SearchIssues searches for issues using JQL
func (c *Client) SearchIssues(jql string, fields []string, limit int) (*models.SearchResponse, error) {
	url := fmt.Sprintf("%s/rest/api/3/search/jql", c.creds.Site)

	payload := map[string]interface{}{
		"jql":        jql,
		"maxResults": limit,
	}

	if len(fields) > 0 {
		payload["fields"] = fields
	} else {
		// If no fields requested, ensure we get at least the essentials.
		payload["fields"] = []string{"key", "summary", "status", "issuetype", "assignee", "updated"}
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search issues: %s", string(body))
	}

	var searchResp models.SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	return &searchResp, nil
}

// GetIssue gets a specific issue by key or ID
func (c *Client) GetIssue(issueKey string, expand []string) (*models.JiraIssue, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.creds.Site, issueKey)

	if len(expand) > 0 {
		expandStr := ""
		for i, e := range expand {
			if i > 0 {
				expandStr += ","
			}
			expandStr += e
		}
		url += "?expand=" + expandStr
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get issue %s: %s", issueKey, string(body))
	}

	var issue models.JiraIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

// CreateIssue creates a new issue
func (c *Client) CreateIssue(projectKey, issueType, summary, description string, additionalFields map[string]interface{}) (*models.JiraIssue, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue", c.creds.Site)

	fields := map[string]interface{}{
		"project": map[string]string{
			"key": projectKey,
		},
		"issuetype": map[string]string{
			"name": issueType,
		},
		"summary":     summary,
		"description": description,
	}

	// Merge additional fields
	for k, v := range additionalFields {
		fields[k] = v
	}

	payload := models.CreateIssueRequest{
		Fields: fields,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create issue: %s", string(body))
	}

	var issue models.JiraIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

// UpdateIssue updates an existing issue
func (c *Client) UpdateIssue(issueKey string, fields map[string]interface{}) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.creds.Site, issueKey)

	payload := models.UpdateIssueRequest{
		Fields: fields,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update issue %s: %s", issueKey, string(body))
	}

	return nil
}

// AddComment adds a comment to an issue
func (c *Client) AddComment(issueKey, body string) (*models.Comment, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/comment", c.creds.Site, issueKey)

	payload := map[string]interface{}{
		"body": body,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to add comment: %s", string(body))
	}

	var comment models.Comment
	if err := json.NewDecoder(resp.Body).Decode(&comment); err != nil {
		return nil, err
	}

	return &comment, nil
}

// TransitionIssue transitions an issue to a different status
func (c *Client) TransitionIssue(issueKey, transitionID string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.creds.Site, issueKey)

	payload := map[string]interface{}{
		"transition": map[string]string{
			"id": transitionID,
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to transition issue %s: %s", issueKey, string(body))
	}

	return nil
}

// ListProjects returns a list of visible projects
func (c *Client) ListProjects() ([]models.ProjectRef, error) {
	url := fmt.Sprintf("%s/rest/api/3/project", c.creds.Site)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list projects: %s", string(body))
	}

	var projects []models.ProjectRef
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, err
	}

	return projects, nil
}

// GetAgileBoards lists all agile boards
func (c *Client) GetAgileBoards(projectKey, boardType string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board", c.creds.Site)
	
	// Add query parameters
	params := ""
	if projectKey != "" {
		params += "?projectKeyOrId=" + projectKey
	}
	if boardType != "" {
		if params == "" {
			params += "?type=" + boardType
		} else {
			params += "&type=" + boardType
		}
	}
	url += params

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get boards: %s", string(body))
	}

	var result struct {
		Values []map[string]interface{} `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Values, nil
}

// GetBoardIssues gets issues on a board
func (c *Client) GetBoardIssues(boardID string, limit int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%s/issue?maxResults=%d", 
		c.creds.Site, boardID, limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get board issues: %s", string(body))
	}

	var result struct {
		Issues []map[string]interface{} `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Issues, nil
}

// GetSprintsFromBoard lists sprints for a board
func (c *Client) GetSprintsFromBoard(boardID, state string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%s/sprint", c.creds.Site, boardID)
	
	if state != "" {
		url += "?state=" + state
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get sprints: %s", string(body))
	}

	var result struct {
		Values []map[string]interface{} `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Values, nil
}

// GetSprintIssues gets issues in a sprint
func (c *Client) GetSprintIssues(sprintID string, limit int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%s/issue?maxResults=%d", 
		c.creds.Site, sprintID, limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get sprint issues: %s", string(body))
	}

	var result struct {
		Issues []map[string]interface{} `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Issues, nil
}

// CreateSprint creates a new sprint
func (c *Client) CreateSprint(boardID, name, startDate, endDate string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint", c.creds.Site)

	payload := map[string]interface{}{
		"name":          name,
		"originBoardId": boardID,
	}
	
	if startDate != "" {
		payload["startDate"] = startDate
	}
	if endDate != "" {
		payload["endDate"] = endDate
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create sprint: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// UpdateSprint updates an existing sprint
func (c *Client) UpdateSprint(sprintID, name, state, startDate, endDate string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%s", c.creds.Site, sprintID)

	payload := make(map[string]interface{})
	
	if name != "" {
		payload["name"] = name
	}
	if state != "" {
		payload["state"] = state
	}
	if startDate != "" {
		payload["startDate"] = startDate
	}
	if endDate != "" {
		payload["endDate"] = endDate
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to update sprint: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetWorklog gets worklog entries for an issue
func (c *Client) GetWorklog(issueKey string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/worklog", c.creds.Site, issueKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get worklog: %s", string(body))
	}

	var result struct {
		Worklogs []map[string]interface{} `json:"worklogs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Worklogs, nil
}

// AddWorklog adds a worklog entry to an issue
func (c *Client) AddWorklog(issueKey, timeSpent, comment, started string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/worklog", c.creds.Site, issueKey)

	payload := map[string]interface{}{
		"timeSpent": timeSpent,
	}
	
	if comment != "" {
		payload["comment"] = comment
	}
	if started != "" {
		payload["started"] = started
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to add worklog: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetTransitions gets available transitions for an issue
func (c *Client) GetTransitions(issueKey string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.creds.Site, issueKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get transitions: %s", string(body))
	}

	var result struct {
		Transitions []map[string]interface{} `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Transitions, nil
}

// DeleteIssue deletes an issue
func (c *Client) DeleteIssue(issueKey string) error {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s", c.creds.Site, issueKey)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete issue: %s", string(body))
	}

	return nil
}

// GetProjectIssues gets all issues in a project
func (c *Client) GetProjectIssues(projectKey string, limit int) (*models.SearchResponse, error) {
	jql := fmt.Sprintf("project=%s ORDER BY created DESC", projectKey)
	return c.SearchIssues(jql, nil, limit)
}

// GetProjectVersions lists versions for a project
func (c *Client) GetProjectVersions(projectKey string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/api/2/project/%s/versions", c.creds.Site, projectKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get project versions: %s", string(body))
	}

	var versions []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, err
	}

	return versions, nil
}

// SearchUsers searches for Jira users
func (c *Client) SearchUsers(query string) ([]models.User, error) {
	url := fmt.Sprintf("%s/rest/api/3/user/search?query=%s", c.creds.Site, query)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search users: %s", string(body))
	}

	var users []models.User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}

	return users, nil
}

// GetUserProfile gets a specific user's detailed profile
func (c *Client) GetUserProfile(accountID string) (*models.User, error) {
	url := fmt.Sprintf("%s/rest/api/3/user?accountId=%s", c.creds.Site, accountID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user profile: %s", string(body))
	}

	var user models.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

// SearchFields lists all available fields in Jira
func (c *Client) SearchFields() ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/api/3/field", c.creds.Site)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search fields: %s", string(body))
	}

	var fields []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&fields); err != nil {
		return nil, err
	}

	return fields, nil
}

// CreateIssueLink creates a link between two issues
func (c *Client) CreateIssueLink(type_name, inward_key, outward_key string) error {
	url := fmt.Sprintf("%s/rest/api/3/issueLink", c.creds.Site)

	payload := map[string]interface{}{
		"type": map[string]string{
			"name": type_name,
		},
		"inwardIssue": map[string]string{
			"key": inward_key,
		},
		"outwardIssue": map[string]string{
			"key": outward_key,
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create issue link: %s", string(body))
	}

	return nil
}

// RemoveIssueLink removes a link between issues
func (c *Client) RemoveIssueLink(linkID string) error {
	url := fmt.Sprintf("%s/rest/api/3/issueLink/%s", c.creds.Site, linkID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove issue link: %s", string(body))
	}

	return nil
}

