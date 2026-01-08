package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Context keys for storing user information
type contextKey string

const (
	UserContextKey contextKey = "user"
)

// ClerkAuth handles Clerk authentication
type ClerkAuth struct {
	secretKey  string
	jwksURL    string
	publicKeys map[string]*rsa.PublicKey
	keysMutex  sync.RWMutex
}

// UserContext represents authenticated user information
type UserContext struct {
	UserID    string
	Email     string
	SessionID string
}

// ClerkClaims represents the JWT claims from Clerk
type ClerkClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
	Email     string `json:"email"`
}

// NewClerkAuth creates a new Clerk auth handler
func NewClerkAuth() *ClerkAuth {
	secretKey := os.Getenv("CLERK_SECRET_KEY")
	if secretKey == "" {
		return nil
	}

	jwksURL := os.Getenv("CLERK_JWKS_URL")
	if jwksURL == "" {
		jwksURL = "https://api.clerk.com/v1/jwks"
	}

	auth := &ClerkAuth{
		secretKey:  secretKey,
		jwksURL:    jwksURL,
		publicKeys: make(map[string]*rsa.PublicKey),
	}

	// Fetch public keys on initialization
	go auth.refreshPublicKeys()

	return auth
}

// VerifyToken verifies a Clerk JWT token
func (c *ClerkAuth) VerifyToken(tokenString string) (*UserContext, error) {
	if c == nil {
		return nil, fmt.Errorf("Clerk authentication not configured")
	}

	// Parse token without verification first to get the kid
	token, err := jwt.ParseWithClaims(tokenString, &ClerkClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get key ID from header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}

		// Get public key
		publicKey, err := c.getPublicKey(kid)
		if err != nil {
			return nil, err
		}

		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*ClerkClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	return &UserContext{
		UserID:    claims.Subject,
		Email:     claims.Email,
		SessionID: claims.SessionID,
	}, nil
}

// getPublicKey retrieves a public key by kid
func (c *ClerkAuth) getPublicKey(kid string) (*rsa.PublicKey, error) {
	c.keysMutex.RLock()
	key, exists := c.publicKeys[kid]
	c.keysMutex.RUnlock()

	if exists {
		return key, nil
	}

	// Refresh keys and try again
	if err := c.refreshPublicKeys(); err != nil {
		return nil, err
	}

	c.keysMutex.RLock()
	key, exists = c.publicKeys[kid]
	c.keysMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("public key not found for kid: %s", kid)
	}

	return key, nil
}

// refreshPublicKeys fetches the latest public keys from Clerk's JWKS endpoint
func (c *ClerkAuth) refreshPublicKeys() error {
	req, err := http.NewRequest("GET", c.jwksURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.secretKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch JWKS: status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	for _, key := range jwks.Keys {
		if key.Kty != "RSA" {
			continue
		}

		// Decode modulus
		nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			continue
		}

		// Decode exponent
		eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil {
			continue
		}

		// Convert exponent to int
		var eInt int
		for _, b := range eBytes {
			eInt = eInt<<8 + int(b)
		}

		publicKey := &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: eInt,
		}

		c.publicKeys[key.Kid] = publicKey
	}

	return nil
}

// ExtractUserFromContext extracts user context from request context
func ExtractUserFromContext(ctx context.Context) (*UserContext, bool) {
	user, ok := ctx.Value(UserContextKey).(*UserContext)
	return user, ok
}

// ExtractTokenFromHeader extracts JWT token from Authorization header
func ExtractTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Expected format: "Bearer <token>"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

// ExtractTokenFromQuery extracts JWT token from query parameter
func ExtractTokenFromQuery(r *http.Request) string {
	return r.URL.Query().Get("token")
}
