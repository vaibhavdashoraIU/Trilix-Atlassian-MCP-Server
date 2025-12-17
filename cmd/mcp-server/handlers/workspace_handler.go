package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/providentiaww/trilix-atlassian-mcp/cmd/mcp-server/auth"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/atlassian"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/storage"
)

// WorkspaceHandler handles workspace management HTTP endpoints
type WorkspaceHandler struct {
	credStore storage.CredentialStoreInterface
	validator *atlassian.Validator
}

// NewWorkspaceHandler creates a new workspace handler
func NewWorkspaceHandler(credStore storage.CredentialStoreInterface) *WorkspaceHandler {
	return &WorkspaceHandler{
		credStore: credStore,
		validator: atlassian.NewValidator(),
	}
}

// CreateWorkspaceRequest represents the request to create a workspace
type CreateWorkspaceRequest struct {
	WorkspaceName string `json:"workspaceName"`
	SiteURL       string `json:"siteUrl"`
	Email         string `json:"email"`
	APIToken      string `json:"apiToken"`
}

// WorkspaceResponse represents a workspace without sensitive data
type WorkspaceResponse struct {
	WorkspaceID   string    `json:"workspaceId"`
	WorkspaceName string    `json:"workspaceName"`
	SiteURL       string    `json:"siteUrl"`
	Email         string    `json:"email"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// HandleCreateWorkspace handles POST /api/workspaces
func (h *WorkspaceHandler) HandleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	// Extract user from context
	userCtx, ok := auth.ExtractUserFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var req CreateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.SiteURL == "" || req.Email == "" || req.APIToken == "" {
		http.Error(w, "Missing required fields: siteUrl, email, apiToken", http.StatusBadRequest)
		return
	}

	// Default workspace name if not provided
	if req.WorkspaceName == "" {
		req.WorkspaceName = req.SiteURL
	}

	// Validate Atlassian token
	if err := h.validator.ValidateToken(req.SiteURL, req.Email, req.APIToken); err != nil {
		http.Error(w, fmt.Sprintf("Invalid Atlassian credentials: %v", err), http.StatusUnauthorized)
		return
	}

	// Generate workspace ID
	workspaceID := uuid.New().String()

	// Create credential object
	cred := &models.AtlassianCredential{
		UserID:        userCtx.UserID,
		WorkspaceID:   workspaceID,
		WorkspaceName: req.WorkspaceName,
		AtlassianURL:  req.SiteURL,
		Email:         req.Email,
		APIToken:      req.APIToken,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Save credentials
	if err := h.credStore.SaveCredentials(cred); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save credentials: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response (without token)
	response := WorkspaceResponse{
		WorkspaceID:   workspaceID,
		WorkspaceName: req.WorkspaceName,
		SiteURL:       req.SiteURL,
		Email:         req.Email,
		CreatedAt:     cred.CreatedAt,
		UpdatedAt:     cred.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// HandleListWorkspaces handles GET /api/workspaces
func (h *WorkspaceHandler) HandleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	// Extract user from context
	userCtx, ok := auth.ExtractUserFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get workspaces
	workspaces, err := h.credStore.ListWorkspaces(userCtx.UserID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list workspaces: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to response format (without tokens)
	var responses []WorkspaceResponse
	for _, ws := range workspaces {
		responses = append(responses, WorkspaceResponse{
			WorkspaceID:   ws.WorkspaceID,
			WorkspaceName: ws.WorkspaceName,
			SiteURL:       ws.AtlassianURL,
			Email:         ws.Email,
			CreatedAt:     ws.CreatedAt,
			UpdatedAt:     ws.UpdatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

// HandleDeleteWorkspace handles DELETE /api/workspaces/:id
func (h *WorkspaceHandler) HandleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	// Extract user from context
	userCtx, ok := auth.ExtractUserFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract workspace ID from URL path
	// Expected format: /api/workspaces/{id}
	workspaceID := r.URL.Path[len("/api/workspaces/"):]
	if workspaceID == "" {
		http.Error(w, "Missing workspace ID", http.StatusBadRequest)
		return
	}

	// Delete credentials
	if err := h.credStore.DeleteCredentials(userCtx.UserID, workspaceID); err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to delete workspace: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleWorkspaceStatus handles GET /api/workspaces/:id/status
func (h *WorkspaceHandler) HandleWorkspaceStatus(w http.ResponseWriter, r *http.Request) {
	// Extract user from context
	userCtx, ok := auth.ExtractUserFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract workspace ID from URL path
	// Expected format: /api/workspaces/{id}/status
	path := r.URL.Path[len("/api/workspaces/"):]
	workspaceID := path[:len(path)-len("/status")]
	if workspaceID == "" {
		http.Error(w, "Missing workspace ID", http.StatusBadRequest)
		return
	}

	// Get credentials
	creds, err := h.credStore.GetCredentials(userCtx.UserID, workspaceID)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to get workspace: %v", err), http.StatusInternalServerError)
		return
	}

	// Test connection
	err = h.validator.ValidateToken(creds.Site, creds.Email, creds.Token)
	
	status := map[string]interface{}{
		"workspaceId": workspaceID,
		"connected":   err == nil,
	}
	
	if err != nil {
		status["error"] = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
