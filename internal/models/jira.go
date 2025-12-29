package models

// JiraRequest represents a request to the Jira service
type JiraRequest struct {
	Action      string         `json:"action"`       // list_issues, get_issue, create_issue, update_issue, add_comment
	WorkspaceID string         `json:"workspace_id"` // User's workspace label
	UserID      string         `json:"user_id"`      // Clerk user ID
	Params      map[string]any `json:"params"`       // Action-specific parameters
	RequestID   string         `json:"request_id"`   // Correlation ID
}

// JiraResponse represents a response from the Jira service
type JiraResponse struct {
	Success   bool       `json:"success"`
	Data      any        `json:"data,omitempty"`
	Error     *ErrorInfo `json:"error,omitempty"`
	RequestID string     `json:"request_id"`
}

// JiraIssue represents a Jira issue
type JiraIssue struct {
	ID     string                 `json:"id"`
	Key    string                 `json:"key"`
	Self   string                 `json:"self"`
	Fields map[string]interface{} `json:"fields"`
}

// IssueFields contains common issue fields
type IssueFields struct {
	Summary     string                 `json:"summary"`
	Description string                 `json:"description,omitempty"`
	Status      IssueStatus            `json:"status"`
	Assignee    *User                  `json:"assignee,omitempty"`
	Reporter    *User                  `json:"reporter,omitempty"`
	Project     ProjectRef              `json:"project"`
	IssueType   IssueType              `json:"issuetype"`
	Created     string                 `json:"created"`
	Updated     string                 `json:"updated"`
	Custom      map[string]interface{} `json:"-"`
}

// IssueStatus represents issue status
type IssueStatus struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
}

// User represents a Jira user
type User struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress,omitempty"`
}

// ProjectRef references a Jira project
type ProjectRef struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// IssueType represents an issue type
type IssueType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SearchResponse represents Jira search results
type SearchResponse struct {
	StartAt       int         `json:"startAt"`
	MaxResults    int         `json:"maxResults"`
	Total         int         `json:"total"`
	NextPageToken string      `json:"nextPageToken,omitempty"`
	Issues        []JiraIssue `json:"issues"`
}

// CreateIssueRequest represents a request to create an issue
type CreateIssueRequest struct {
	Fields map[string]interface{} `json:"fields"`
}

// UpdateIssueRequest represents a request to update an issue
type UpdateIssueRequest struct {
	Fields map[string]interface{} `json:"fields"`
}

// Comment represents a Jira comment
type Comment struct {
	Body    string `json:"body"`
	Created string `json:"created,omitempty"`
	Author  *User  `json:"author,omitempty"`
}

