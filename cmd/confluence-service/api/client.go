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
	Site  string // e.g., "https://eso.atlassian.net/wiki"
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

// NewClient creates an authenticated Confluence client
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

// GetPage fetches a page by ID with body content
func (c *Client) GetPage(pageID string) (*models.ConfluencePage, error) {
	url := fmt.Sprintf("%s/rest/api/content/%s?expand=body.storage,version",
		c.creds.Site, pageID)

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
		return nil, fmt.Errorf("failed to get page %s: %s", pageID, string(body))
	}

	var page models.ConfluencePage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, err
	}

	return &page, nil
}

// GetChildren returns all direct child pages of a parent page
func (c *Client) GetChildren(pageID string) ([]models.ConfluencePage, error) {
	url := fmt.Sprintf("%s/rest/api/content/%s/child/page?expand=version",
		c.creds.Site, pageID)

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
		return nil, fmt.Errorf("failed to get children of %s: %s", pageID, string(body))
	}

	var result struct {
		Results []models.ConfluencePage `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

// CreatePage creates a new page in the specified space
func (c *Client) CreatePage(spaceKey, title, body string, parentID *string) (*models.ConfluencePage, error) {
	url := fmt.Sprintf("%s/rest/api/content", c.creds.Site)

	payload := models.CreatePageRequest{
		Type:  "page",
		Title: title,
		Space: models.SpaceRef{Key: spaceKey},
		Body: models.BodyContent{
			Storage: models.StorageContent{
				Value:          body,
				Representation: "storage",
			},
		},
	}

	if parentID != nil {
		payload.Ancestors = []models.AncestorRef{{ID: *parentID}}
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
		return nil, fmt.Errorf("failed to create page '%s': %s", title, string(body))
	}

	var page models.ConfluencePage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, err
	}

	return &page, nil
}

// SearchPages searches for pages using CQL
func (c *Client) SearchPages(cql string, limit int) (*models.SearchResults, error) {
	url := fmt.Sprintf("%s/rest/api/content/search?cql=%s&limit=%d",
		c.creds.Site, cql, limit)

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
		return nil, fmt.Errorf("failed to search: %s", string(body))
	}

	var results models.SearchResults
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return &results, nil
}

// ListSpaces lists all spaces in the workspace
func (c *Client) ListSpaces(limit int) ([]models.ConfluenceSpace, error) {
	url := fmt.Sprintf("%s/rest/api/space?limit=%d", c.creds.Site, limit)

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
		return nil, fmt.Errorf("failed to list spaces: %s", string(body))
	}

	var result struct {
		Results []models.ConfluenceSpace `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

// GetSpace gets details about a specific space
func (c *Client) GetSpace(spaceKey string) (*models.ConfluenceSpace, error) {
	url := fmt.Sprintf("%s/rest/api/space/%s", c.creds.Site, spaceKey)

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
		return nil, fmt.Errorf("failed to get space %s: %s", spaceKey, string(body))
	}

	var space models.ConfluenceSpace
	if err := json.NewDecoder(resp.Body).Decode(&space); err != nil {
		return nil, err
	}

	return &space, nil
}

