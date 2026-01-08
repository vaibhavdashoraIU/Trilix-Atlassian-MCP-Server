package oauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

// Store provides persistence for OAuth data.
type Store struct {
	db    *sql.DB
	redis *redis.Client
}

// NewStoreFromEnv initializes the OAuth store using Postgres and optional Redis.
func NewStoreFromEnv() (*Store, error) {
	connString := os.Getenv("OAUTH_DATABASE_URL")
	if connString == "" {
		connString = os.Getenv("DATABASE_URL")
	}
	if connString == "" {
		return nil, fmt.Errorf("OAUTH_DATABASE_URL or DATABASE_URL is required")
	}

	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres: %w", err)
	}
	db.SetMaxOpenConns(parseEnvInt("OAUTH_DB_MAX_OPEN_CONNS", 5))
	db.SetMaxIdleConns(parseEnvInt("OAUTH_DB_MAX_IDLE_CONNS", 2))
	db.SetConnMaxLifetime(parseEnvDuration("OAUTH_DB_CONN_MAX_LIFETIME", 5*time.Minute))

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	store := &Store{db: db}
	if err := store.initSchema(); err != nil {
		return nil, err
	}

	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_URL: %w", err)
		}
		store.redis = redis.NewClient(opts)
		if err := store.redis.Ping(context.Background()).Err(); err != nil {
			return nil, fmt.Errorf("failed to ping redis: %w", err)
		}
	}

	return store, nil
}

// Close closes connections.
func (s *Store) Close() error {
	if s.redis != nil {
		_ = s.redis.Close()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Ping verifies database and Redis connectivity.
func (s *Store) Ping() error {
	if s.db != nil {
		if err := s.db.Ping(); err != nil {
			return err
		}
	}
	if s.redis != nil {
		if err := s.redis.Ping(context.Background()).Err(); err != nil {
			return err
		}
	}
	return nil
}

// SaveClient stores an OAuth client.
func (s *Store) SaveClient(client *Client) error {
	query := `
		INSERT INTO oauth_clients
			(client_id, client_secret_hash, redirect_uris, grant_types, response_types, scope, token_endpoint_auth_method, client_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (client_id)
		DO UPDATE SET
			client_secret_hash = EXCLUDED.client_secret_hash,
			redirect_uris = EXCLUDED.redirect_uris,
			grant_types = EXCLUDED.grant_types,
			response_types = EXCLUDED.response_types,
			scope = EXCLUDED.scope,
			token_endpoint_auth_method = EXCLUDED.token_endpoint_auth_method,
			client_name = EXCLUDED.client_name,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	if client.CreatedAt.IsZero() {
		client.CreatedAt = now
	}
	client.UpdatedAt = now

	_, err := s.db.Exec(
		query,
		client.ClientID,
		nullableString(client.ClientSecretHash),
		pq.Array(client.RedirectURIs),
		pq.Array(client.GrantTypes),
		pq.Array(client.ResponseTypes),
		nullableString(client.Scope),
		client.TokenEndpointAuthMethod,
		nullableString(client.ClientName),
		client.CreatedAt,
		client.UpdatedAt,
	)
	return err
}

// GetClient fetches an OAuth client by id.
func (s *Store) GetClient(clientID string) (*Client, error) {
	query := `
		SELECT client_id, client_secret_hash, redirect_uris, grant_types, response_types, scope, token_endpoint_auth_method, client_name, created_at, updated_at
		FROM oauth_clients
		WHERE client_id = $1
	`

	var client Client
	var redirectURIs, grantTypes, responseTypes []string
	var scope, secretHash, clientName sql.NullString

	err := s.db.QueryRow(query, clientID).Scan(
		&client.ClientID,
		&secretHash,
		pq.Array(&redirectURIs),
		pq.Array(&grantTypes),
		pq.Array(&responseTypes),
		&scope,
		&client.TokenEndpointAuthMethod,
		&clientName,
		&client.CreatedAt,
		&client.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	client.ClientSecretHash = secretHash.String
	client.RedirectURIs = redirectURIs
	client.GrantTypes = grantTypes
	client.ResponseTypes = responseTypes
	client.Scope = scope.String
	client.ClientName = clientName.String
	return &client, nil
}

// SaveAuthRequest stores an auth request in Redis or Postgres.
func (s *Store) SaveAuthRequest(req *AuthRequest) error {
	if s.redis != nil {
		payload, err := json.Marshal(req)
		if err != nil {
			return err
		}
		key := fmt.Sprintf("oauth:req:%s", req.RequestID)
		return s.redis.Set(context.Background(), key, payload, time.Until(req.ExpiresAt)).Err()
	}

	query := `
		INSERT INTO oauth_auth_requests
			(request_id, client_id, redirect_uri, scope, state, response_type, code_challenge, code_challenge_method, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`
	_, err := s.db.Exec(
		query,
		req.RequestID,
		req.ClientID,
		req.RedirectURI,
		req.Scope,
		req.State,
		req.ResponseType,
		req.CodeChallenge,
		req.CodeChallengeMethod,
		req.CreatedAt,
		req.ExpiresAt,
	)
	return err
}

// GetAuthRequest retrieves an auth request.
func (s *Store) GetAuthRequest(requestID string) (*AuthRequest, error) {
	if s.redis != nil {
		key := fmt.Sprintf("oauth:req:%s", requestID)
		val, err := s.redis.Get(context.Background(), key).Result()
		if err != nil {
			return nil, err
		}
		var req AuthRequest
		if err := json.Unmarshal([]byte(val), &req); err != nil {
			return nil, err
		}
		return &req, nil
	}

	query := `
		SELECT request_id, client_id, redirect_uri, scope, state, response_type, code_challenge, code_challenge_method, created_at, expires_at
		FROM oauth_auth_requests
		WHERE request_id = $1
	`
	var req AuthRequest
	err := s.db.QueryRow(query, requestID).Scan(
		&req.RequestID,
		&req.ClientID,
		&req.RedirectURI,
		&req.Scope,
		&req.State,
		&req.ResponseType,
		&req.CodeChallenge,
		&req.CodeChallengeMethod,
		&req.CreatedAt,
		&req.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// DeleteAuthRequest deletes an auth request.
func (s *Store) DeleteAuthRequest(requestID string) error {
	if s.redis != nil {
		key := fmt.Sprintf("oauth:req:%s", requestID)
		return s.redis.Del(context.Background(), key).Err()
	}
	_, err := s.db.Exec(`DELETE FROM oauth_auth_requests WHERE request_id = $1`, requestID)
	return err
}

// SaveAuthCode stores auth code data.
func (s *Store) SaveAuthCode(code *AuthCode) error {
	if s.redis != nil {
		payload, err := json.Marshal(code)
		if err != nil {
			return err
		}
		key := fmt.Sprintf("oauth:code:%s", code.CodeHash)
		return s.redis.Set(context.Background(), key, payload, time.Until(code.ExpiresAt)).Err()
	}

	query := `
		INSERT INTO oauth_auth_codes
			(code_hash, client_id, redirect_uri, user_id, scope, code_challenge, code_challenge_method, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`
	_, err := s.db.Exec(
		query,
		code.CodeHash,
		code.ClientID,
		code.RedirectURI,
		code.UserID,
		code.Scope,
		code.CodeChallenge,
		code.CodeChallengeMethod,
		code.CreatedAt,
		code.ExpiresAt,
	)
	return err
}

// ConsumeAuthCode retrieves and deletes an auth code.
func (s *Store) ConsumeAuthCode(codeHash string) (*AuthCode, error) {
	if s.redis != nil {
		key := fmt.Sprintf("oauth:code:%s", codeHash)
		val, err := s.redis.GetDel(context.Background(), key).Result()
		if err != nil {
			return nil, err
		}
		var code AuthCode
		if err := json.Unmarshal([]byte(val), &code); err != nil {
			return nil, err
		}
		return &code, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var code AuthCode
	query := `
		SELECT code_hash, client_id, redirect_uri, user_id, scope, code_challenge, code_challenge_method, created_at, expires_at
		FROM oauth_auth_codes
		WHERE code_hash = $1
		FOR UPDATE
	`
	if err = tx.QueryRow(query, codeHash).Scan(
		&code.CodeHash,
		&code.ClientID,
		&code.RedirectURI,
		&code.UserID,
		&code.Scope,
		&code.CodeChallenge,
		&code.CodeChallengeMethod,
		&code.CreatedAt,
		&code.ExpiresAt,
	); err != nil {
		return nil, err
	}

	if _, err = tx.Exec(`DELETE FROM oauth_auth_codes WHERE code_hash = $1`, codeHash); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &code, nil
}

// SaveRefreshToken persists a refresh token.
func (s *Store) SaveRefreshToken(token *RefreshToken) error {
	query := `
		INSERT INTO oauth_refresh_tokens
			(token_hash, client_id, user_id, scope, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`
	_, err := s.db.Exec(query, token.TokenHash, token.ClientID, token.UserID, token.Scope, token.CreatedAt, token.ExpiresAt)
	return err
}

// GetRefreshToken retrieves a refresh token.
func (s *Store) GetRefreshToken(hash string) (*RefreshToken, error) {
	query := `
		SELECT token_hash, client_id, user_id, scope, created_at, expires_at, revoked_at
		FROM oauth_refresh_tokens
		WHERE token_hash = $1
	`
	var token RefreshToken
	var revokedAt sql.NullTime
	err := s.db.QueryRow(query, hash).Scan(
		&token.TokenHash,
		&token.ClientID,
		&token.UserID,
		&token.Scope,
		&token.CreatedAt,
		&token.ExpiresAt,
		&revokedAt,
	)
	if err != nil {
		return nil, err
	}
	if revokedAt.Valid {
		token.RevokedAt = &revokedAt.Time
	}
	return &token, nil
}

// RevokeRefreshToken marks a refresh token as revoked.
func (s *Store) RevokeRefreshToken(hash string) error {
	now := time.Now()
	_, err := s.db.Exec(`UPDATE oauth_refresh_tokens SET revoked_at = $1 WHERE token_hash = $2`, now, hash)
	return err
}

// SaveAccessToken stores a JWT identifier for revocation checks.
func (s *Store) SaveAccessToken(token *AccessToken) error {
	query := `
		INSERT INTO oauth_access_tokens
			(jti, client_id, user_id, scope, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`
	_, err := s.db.Exec(query, token.JTI, token.ClientID, token.UserID, token.Scope, token.CreatedAt, token.ExpiresAt)
	return err
}

// IsAccessTokenRevoked checks if a token has been revoked.
func (s *Store) IsAccessTokenRevoked(jti string) (bool, error) {
	query := `
		SELECT revoked_at
		FROM oauth_access_tokens
		WHERE jti = $1
	`
	var revokedAt sql.NullTime
	err := s.db.QueryRow(query, jti).Scan(&revokedAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return revokedAt.Valid, nil
}

func (s *Store) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS oauth_clients (
		client_id VARCHAR(255) PRIMARY KEY,
		client_secret_hash TEXT,
		redirect_uris TEXT[] NOT NULL,
		grant_types TEXT[] NOT NULL,
		response_types TEXT[] NOT NULL,
		scope TEXT,
		token_endpoint_auth_method VARCHAR(50) NOT NULL,
		client_name TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS oauth_auth_requests (
		request_id VARCHAR(255) PRIMARY KEY,
		client_id VARCHAR(255) NOT NULL,
		redirect_uri TEXT NOT NULL,
		scope TEXT,
		state TEXT,
		response_type TEXT NOT NULL,
		code_challenge TEXT NOT NULL,
		code_challenge_method TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		expires_at TIMESTAMP NOT NULL
	);

	CREATE TABLE IF NOT EXISTS oauth_auth_codes (
		code_hash TEXT PRIMARY KEY,
		client_id VARCHAR(255) NOT NULL,
		redirect_uri TEXT NOT NULL,
		user_id VARCHAR(255) NOT NULL,
		scope TEXT,
		code_challenge TEXT NOT NULL,
		code_challenge_method TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		expires_at TIMESTAMP NOT NULL
	);

	CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
		token_hash TEXT PRIMARY KEY,
		client_id VARCHAR(255) NOT NULL,
		user_id VARCHAR(255) NOT NULL,
		scope TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		expires_at TIMESTAMP NOT NULL,
		revoked_at TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS oauth_access_tokens (
		jti TEXT PRIMARY KEY,
		client_id VARCHAR(255) NOT NULL,
		user_id VARCHAR(255) NOT NULL,
		scope TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		expires_at TIMESTAMP NOT NULL,
		revoked_at TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_oauth_clients_client_id ON oauth_clients(client_id);
	CREATE INDEX IF NOT EXISTS idx_oauth_auth_requests_expires ON oauth_auth_requests(expires_at);
	CREATE INDEX IF NOT EXISTS idx_oauth_auth_codes_expires ON oauth_auth_codes(expires_at);
	CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_user ON oauth_refresh_tokens(user_id);
	CREATE INDEX IF NOT EXISTS idx_oauth_access_tokens_user ON oauth_access_tokens(user_id);
	`

	_, err := s.db.Exec(query)
	return err
}

func nullableString(val string) sql.NullString {
	if val == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: val, Valid: true}
}

func parseEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return fallback
}

func parseEnvDuration(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			return parsed
		}
	}
	return fallback
}
