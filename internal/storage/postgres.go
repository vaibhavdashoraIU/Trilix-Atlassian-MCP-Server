package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/providentiaww/trilix-atlassian-mcp/internal/crypto"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
	_ "github.com/lib/pq"
)

// CredentialStore handles storage and retrieval of Atlassian credentials
type CredentialStore struct {
	db *sql.DB
	encryptionKey string
}

// NewCredentialStore creates a new credential store
func NewCredentialStore(connectionString, encryptionKey string) (*CredentialStore, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres: %v", err)
	}

	// Set connection pool limits for cloud stability
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %v", err)
	}

	fmt.Println("✅ Successfully connected to PostgreSQL/Supabase")

	store := &CredentialStore{
		db:            db,
		encryptionKey: encryptionKey,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %v", err)
	}

	return store, nil
}

// initSchema creates the necessary database tables
func (s *CredentialStore) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS atlassian_credentials (
		user_id VARCHAR(255) NOT NULL,
		workspace_id VARCHAR(255) NOT NULL,
		workspace_name VARCHAR(255) NOT NULL,
		atlassian_url VARCHAR(500) NOT NULL,
		email VARCHAR(255) NOT NULL,
		api_token_encrypted TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		PRIMARY KEY (user_id, workspace_id)
	);

	CREATE INDEX IF NOT EXISTS idx_user_id ON atlassian_credentials(user_id);
	`

	_, err := s.db.Exec(query)
	return err
}

// GetCredentials retrieves and decrypts credentials for a user/workspace
func (s *CredentialStore) GetCredentials(userID, workspaceID string) (*models.WorkspaceCredentials, error) {
	var encryptedToken, atlassianURL, email string

	query := `
		SELECT atlassian_url, email, api_token_encrypted
		FROM atlassian_credentials
		WHERE user_id = $1 AND workspace_id = $2
	`

	err := s.db.QueryRow(query, userID, workspaceID).Scan(&atlassianURL, &email, &encryptedToken)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Decrypt token
	token, err := crypto.Decrypt(encryptedToken, s.encryptionKey)
	if err != nil {
		return nil, err
	}

	return &models.WorkspaceCredentials{
		Site:  atlassianURL,
		Email: email,
		Token: token,
	}, nil
}

// SaveCredentials encrypts and stores credentials
func (s *CredentialStore) SaveCredentials(cred *models.AtlassianCredential) error {
	// Encrypt token
	encryptedToken, err := crypto.Encrypt(cred.APIToken, s.encryptionKey)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO atlassian_credentials 
			(user_id, workspace_id, workspace_name, atlassian_url, email, api_token_encrypted, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, workspace_id)
		DO UPDATE SET
			workspace_name = EXCLUDED.workspace_name,
			atlassian_url = EXCLUDED.atlassian_url,
			email = EXCLUDED.email,
			api_token_encrypted = EXCLUDED.api_token_encrypted,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	if cred.CreatedAt.IsZero() {
		cred.CreatedAt = now
	}
	cred.UpdatedAt = now

	_, err = s.db.Exec(query,
		cred.UserID,
		cred.WorkspaceID,
		cred.WorkspaceName,
		cred.AtlassianURL,
		cred.Email,
		encryptedToken,
		cred.CreatedAt,
		cred.UpdatedAt,
	)

	return err
}

// DeleteCredentials removes credentials for a user/workspace
func (s *CredentialStore) DeleteCredentials(userID, workspaceID string) error {
	query := `
		DELETE FROM atlassian_credentials
		WHERE user_id = $1 AND workspace_id = $2
	`

	_, err := s.db.Exec(query, userID, workspaceID)
	return err
}

// ListWorkspaces returns all workspaces for a user
func (s *CredentialStore) ListWorkspaces(userID string) ([]models.AtlassianCredential, error) {
	query := `
		SELECT user_id, workspace_id, workspace_name, atlassian_url, email, created_at, updated_at
		FROM atlassian_credentials
		WHERE user_id = $1
		ORDER BY workspace_name
	`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var credentials []models.AtlassianCredential
	for rows.Next() {
		var cred models.AtlassianCredential
		err := rows.Scan(
			&cred.UserID,
			&cred.WorkspaceID,
			&cred.WorkspaceName,
			&cred.AtlassianURL,
			&cred.Email,
			&cred.CreatedAt,
			&cred.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, cred)
	}

	return credentials, rows.Err()
}

// Ping tests the database connection
func (s *CredentialStore) Ping() error {
	return s.db.Ping()
}

// Close closes the database connection
func (s *CredentialStore) Close() error {
	return s.db.Close()
}

var ErrNotFound = &NotFoundError{}

type NotFoundError struct{}

func (e *NotFoundError) Error() string {
	return "credentials not found"
}

