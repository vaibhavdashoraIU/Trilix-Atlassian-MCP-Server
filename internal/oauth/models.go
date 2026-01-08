package oauth

import "time"

// Client represents an OAuth client registration.
type Client struct {
	ClientID                string
	ClientSecretHash        string
	RedirectURIs            []string
	GrantTypes              []string
	ResponseTypes           []string
	Scope                   string
	TokenEndpointAuthMethod string
	ClientName              string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// AuthRequest represents a pending authorization request prior to login.
type AuthRequest struct {
	RequestID           string
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	ResponseType        string
	CodeChallenge       string
	CodeChallengeMethod string
	CreatedAt           time.Time
	ExpiresAt           time.Time
}

// AuthCode represents an authorization code exchange record.
type AuthCode struct {
	CodeHash            string
	ClientID            string
	RedirectURI         string
	UserID              string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
	CreatedAt           time.Time
	ExpiresAt           time.Time
}

// RefreshToken represents a refresh token record.
type RefreshToken struct {
	TokenHash string
	ClientID  string
	UserID    string
	Scope     string
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// AccessToken represents a JWT record for revocation checks.
type AccessToken struct {
	JTI       string
	ClientID  string
	UserID    string
	Scope     string
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}
