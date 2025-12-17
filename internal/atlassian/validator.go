package atlassian

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Validator handles Atlassian API token validation
type Validator struct {
	client *http.Client
}

// NewValidator creates a new Atlassian validator
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ValidateToken validates an Atlassian API token by calling the /myself endpoint
func (v *Validator) ValidateToken(siteURL, email, apiToken string) error {
	// Normalize site URL
	siteURL = strings.TrimSuffix(siteURL, "/")
	
	// Construct API endpoint
	apiURL := fmt.Sprintf("%s/rest/api/3/myself", siteURL)
	
	// Create request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set basic auth
	req.SetBasicAuth(email, apiToken)
	req.Header.Set("Accept", "application/json")
	
	// Execute request
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Atlassian: %w", err)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("invalid credentials: authentication failed")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// Parse response to verify it's valid
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Verify the email matches
	if accountEmail, ok := result["emailAddress"].(string); ok {
		if accountEmail != email {
			return fmt.Errorf("email mismatch: expected %s, got %s", email, accountEmail)
		}
	}
	
	return nil
}

// ValidateConfluenceAccess checks if the token has access to Confluence
func (v *Validator) ValidateConfluenceAccess(siteURL, email, apiToken string) error {
	// Normalize site URL
	siteURL = strings.TrimSuffix(siteURL, "/")
	
	// Construct Confluence API endpoint
	apiURL := fmt.Sprintf("%s/wiki/rest/api/space", siteURL)
	
	// Create request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set basic auth
	req.SetBasicAuth(email, apiToken)
	req.Header.Set("Accept", "application/json")
	
	// Execute request
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Confluence: %w", err)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("no Confluence access")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	return nil
}

// ValidateJiraAccess checks if the token has access to Jira
func (v *Validator) ValidateJiraAccess(siteURL, email, apiToken string) error {
	// Normalize site URL
	siteURL = strings.TrimSuffix(siteURL, "/")
	
	// Construct Jira API endpoint
	apiURL := fmt.Sprintf("%s/rest/api/3/project", siteURL)
	
	// Create request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set basic auth
	req.SetBasicAuth(email, apiToken)
	req.Header.Set("Accept", "application/json")
	
	// Execute request
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Jira: %w", err)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("no Jira access")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	return nil
}
