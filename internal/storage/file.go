package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
)

// CredentialStoreInterface defines the interface for credential storage
type CredentialStoreInterface interface {
	GetCredentials(userID, workspaceID string) (*models.WorkspaceCredentials, error)
	SaveCredentials(cred *models.AtlassianCredential) error
	DeleteCredentials(userID, workspaceID string) error
	ListWorkspaces(userID string) ([]models.AtlassianCredential, error)
	Ping() error
	Close() error
}

// WorkspaceConfig represents the structure of workspaces.json
type WorkspaceConfig struct {
	ID       string `json:"id,omitempty"` // Added for UUID support
	Name     string `json:"name"`
	BaseURL  string `json:"baseUrl"`
	Email    string `json:"email"`
	APIToken string `json:"apiToken"`
}

// FileCredentialStore handles storage and retrieval of Atlassian credentials from a JSON file
// Supports multiple workspaces simultaneously
type FileCredentialStore struct {
	filePath    string
	workspaces  map[string]WorkspaceConfig // Indexed by workspace ID (or name if ID missing)
	lastModTime time.Time                  // Track file modification time
	mu          sync.RWMutex               // Thread safety
}

// NewFileCredentialStore creates a new file-based credential store
func NewFileCredentialStore(filePath string) (*FileCredentialStore, error) {
	store := &FileCredentialStore{
		filePath:   filePath,
		workspaces: make(map[string]WorkspaceConfig),
	}

	// Load workspaces from file
	if err := store.loadWorkspaces(); err != nil {
		return nil, fmt.Errorf("failed to load workspaces: %w", err)
	}

	return store, nil
}

// loadWorkspaces reads and parses the workspaces.json file
func (s *FileCredentialStore) loadWorkspaces() error {
	// Resolve absolute path
	absPath, err := filepath.Abs(s.filePath)
	if err != nil {
		return err
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		s.workspaces = make(map[string]WorkspaceConfig)
		return nil
	}

	// Read file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read workspaces file: %w", err)
	}

	// Parse JSON
	var workspaces []WorkspaceConfig
	if len(data) > 0 {
		if err := json.Unmarshal(data, &workspaces); err != nil {
			return fmt.Errorf("failed to parse workspaces JSON: %w", err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.workspaces = make(map[string]WorkspaceConfig)
	// Index by ID if present, else Name
	for _, ws := range workspaces {
		id := ws.ID
		if id == "" {
			id = ws.Name
		}
		s.workspaces[id] = ws
	}

	// Update last modification time
	if stat, err := os.Stat(absPath); err == nil {
		s.lastModTime = stat.ModTime()
	}

	return nil
}

// saveToFile writes the current workspaces to the JSON file
func (s *FileCredentialStore) saveToFile() error {
	s.mu.RLock()
	var list []WorkspaceConfig
	for _, ws := range s.workspaces {
		list = append(list, ws)
	}
	s.mu.RUnlock()

	// Sort by Name for stable output
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(s.filePath)
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(absPath, data, 0644)
}

// GetCredentials retrieves credentials for a user/workspace
func (s *FileCredentialStore) GetCredentials(userID, workspaceID string) (*models.WorkspaceCredentials, error) {
	s.checkAndReload()
	
	s.mu.RLock()
	ws, exists := s.workspaces[workspaceID]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrNotFound
	}

	return &models.WorkspaceCredentials{
		Site:  ws.BaseURL,
		Email: ws.Email,
		Token: ws.APIToken,
	}, nil
}

// SaveCredentials saves credentials to the file
func (s *FileCredentialStore) SaveCredentials(cred *models.AtlassianCredential) error {
	s.checkAndReload()

	s.mu.Lock()
	// Use generated ID as key
	id := cred.WorkspaceID
	if id == "" {
		id = cred.WorkspaceName // Fallback, though WorkspaceID should be set by handler
	}

	s.workspaces[id] = WorkspaceConfig{
		ID:       id,
		Name:     cred.WorkspaceName,
		BaseURL:  cred.AtlassianURL,
		Email:    cred.Email,
		APIToken: cred.APIToken,
	}
	s.mu.Unlock()

	return s.saveToFile()
}

// DeleteCredentials removes credentials from the file
func (s *FileCredentialStore) DeleteCredentials(userID, workspaceID string) error {
	s.checkAndReload()

	s.mu.Lock()
	if _, exists := s.workspaces[workspaceID]; !exists {
		s.mu.Unlock()
		return ErrNotFound
	}
	delete(s.workspaces, workspaceID)
	s.mu.Unlock()

	return s.saveToFile()
}

// ListWorkspaces returns all workspaces from the file
func (s *FileCredentialStore) ListWorkspaces(userID string) ([]models.AtlassianCredential, error) {
	s.checkAndReload()
	
	s.mu.RLock()
	defer s.mu.RUnlock()

	var credentials []models.AtlassianCredential
	for id, ws := range s.workspaces {
		credentials = append(credentials, models.AtlassianCredential{
			UserID:        userID,
			WorkspaceID:   id, // This is either UUID or Name
			WorkspaceName: ws.Name,
			AtlassianURL:  ws.BaseURL,
			Email:         ws.Email,
			APIToken:      ws.APIToken,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		})
	}
	return credentials, nil
}

// Ping is a no-op for file-based storage
func (s *FileCredentialStore) Ping() error {
	return nil
}

// Close is a no-op for file-based storage
func (s *FileCredentialStore) Close() error {
	return nil
}

// checkAndReload checks if the file has been modified and reloads if necessary
func (s *FileCredentialStore) checkAndReload() {
	absPath, err := filepath.Abs(s.filePath)
	if err != nil {
		return
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return
	}
	
	s.mu.RLock()
	lastMod := s.lastModTime
	s.mu.RUnlock()

	if stat.ModTime().After(lastMod) {
		s.loadWorkspaces()
	}
}

// NewCredentialStoreFromEnv creates a credential store based on environment variables
// If WORKSPACES_FILE is set, uses file-based storage
// Otherwise, uses PostgreSQL storage (requires DATABASE_URL and API_KEY_ENCRYPTION_KEY)
func NewCredentialStoreFromEnv() (CredentialStoreInterface, error) {
	workspacesFile := os.Getenv("WORKSPACES_FILE")
	if workspacesFile != "" {
		// Use file-based storage
		return NewFileCredentialStore(workspacesFile)
	}

	// Use PostgreSQL storage
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("either WORKSPACES_FILE or DATABASE_URL must be set")
	}

	encryptionKey := os.Getenv("API_KEY_ENCRYPTION_KEY")
	if encryptionKey == "" {
		return nil, fmt.Errorf("API_KEY_ENCRYPTION_KEY is required when using database storage")
	}

	return NewCredentialStore(databaseURL, encryptionKey)
}

