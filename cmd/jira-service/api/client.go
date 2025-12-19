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

// NewClient creates an authenticated Jira client
func NewClient(creds WorkspaceCredentials, timeout time.Duration) *Client {
	return &Client{
		creds: creds,
		httpClient: &http.Client{
			Timeout: timeout,
		},
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
	url := fmt.Sprintf("%s/rest/api/3/search", c.creds.Site)

	payload := map[string]interface{}{
		"jql":        jql,
		"maxResults": limit,
		"startAt":    0,
	}

	if len(fields) > 0 {
		payload["fields"] = fields
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

