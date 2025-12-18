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
	
	// Helper function to try an endpoint
	tryEndpoint := func(version string) error {
		apiURL := fmt.Sprintf("%s/rest/api/%s/myself", siteURL, version)
		
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		
		req.SetBasicAuth(email, apiToken)
		req.Header.Set("Accept", "application/json")
		
		resp, err := v.client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to connect to Atlassian: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("not found") // specific error to trigger fallback
		}
		
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("invalid credentials: authentication failed")
		}
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
		
		// Verify the email matches
		if accountEmail, ok := result["emailAddress"].(string); ok {
			// Basic check - some instances might return email differently or not at all depending on privacy settings
			// We warn but don't strictly fail if it's just missing, but if it mismatches we should know
			if accountEmail != "" && !strings.EqualFold(accountEmail, email) {
				return fmt.Errorf("email mismatch: expected %s, got %s", email, accountEmail)
			}
		}
		
		return nil
	}

	// Try v3 first
	err := tryEndpoint("3")
	if err != nil {
		if err.Error() == "not found" {
			// Fallback to v2
			err = tryEndpoint("2")
			if err != nil {
				if err.Error() == "not found" {
					// Fallback to Confluence (for Confluence-only sites)
					// Try checking current user in Confluence
					confluenceURL := fmt.Sprintf("%s/wiki/rest/api/user/current", siteURL)
					req, cErr := http.NewRequest("GET", confluenceURL, nil)
					if cErr != nil {
						return fmt.Errorf("failed to create Confluence request: %w", cErr)
					}
					req.SetBasicAuth(email, apiToken)
					req.Header.Set("Accept", "application/json")
					
					resp, cErr := v.client.Do(req)
					if cErr != nil {
						return fmt.Errorf("failed to connect to Confluence: %w", cErr)
					}
					defer resp.Body.Close()
					
					if resp.StatusCode == http.StatusOK {
						// Success! It's a Confluence instance.
						return nil
					} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
                         return fmt.Errorf("invalid credentials (checked Jira v3/v2 and Confluence)")
                    }
                    
					// If Confluence also 404s (or other error), return the original "not found"
					return fmt.Errorf("API endpoint not found (tried Jira v3, Jira v2, and Confluence). Check your Site URL.")
				}
				return err
			}
			return nil
		}
		return err
	}
	
	return nil
}

// ValidateConfluenceAccess checks if the token has access to Confluence
func (v *Validator) ValidateConfluenceAccess(siteURL, email, apiToken string) error {
	// Normalize site URL
	siteURL = strings.TrimSuffix(siteURL, "/")
	
	tryEndpoint := func(basePath string) error {
		apiURL := fmt.Sprintf("%s%s", siteURL, basePath)
		
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		
		req.SetBasicAuth(email, apiToken)
		req.Header.Set("Accept", "application/json")
		
		resp, err := v.client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to connect to Confluence: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("not found")
		}
		
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("no Confluence access")
		}
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		
		return nil
	}

	// Try cloud/v3 path
	err := tryEndpoint("/wiki/rest/api/space")
	if err != nil {
		if err.Error() == "not found" {
			return err
		}
		return err
	}
	
	return nil
}

// ValidateJiraAccess checks if the token has access to Jira
func (v *Validator) ValidateJiraAccess(siteURL, email, apiToken string) error {
	// Normalize site URL
	siteURL = strings.TrimSuffix(siteURL, "/")
	
	tryEndpoint := func(version string) error {
		apiURL := fmt.Sprintf("%s/rest/api/%s/project", siteURL, version)
		
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		
		req.SetBasicAuth(email, apiToken)
		req.Header.Set("Accept", "application/json")
		
		resp, err := v.client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to connect to Jira: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("not found")
		}
		
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("no Jira access")
		}
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		
		return nil
	}
	
	// Try v3 first
	err := tryEndpoint("3")
	if err != nil {
		if err.Error() == "not found" {
			// Fallback to v2
			err = tryEndpoint("2")
			if err != nil {
				if err.Error() == "not found" {
					return fmt.Errorf("Jira API endpoint not found (tried v3 and v2)")
				}
				return err
			}
			return nil
		}
		return err
	}
	
	return nil
}
