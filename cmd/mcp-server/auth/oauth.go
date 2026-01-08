package auth

import (
	"crypto/rsa"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/oauth"
)

// OAuthVerifier validates OAuth access tokens issued by this server.
type OAuthVerifier struct {
	issuer    string
	audience  string
	publicKey *rsa.PublicKey
	store     *oauth.Store
}

// OAuthClaims represents OAuth JWT claims.
type OAuthClaims struct {
	jwt.RegisteredClaims
	Scope    string `json:"scope"`
	Email    string `json:"email,omitempty"`
	ClientID string `json:"client_id,omitempty"`
}

// NewOAuthVerifier creates a new OAuth verifier.
func NewOAuthVerifier(cfg oauth.Config, keys *oauth.KeyManager, store *oauth.Store) *OAuthVerifier {
	return &OAuthVerifier{
		issuer:    cfg.Issuer,
		audience:  cfg.Audience,
		publicKey: keys.PublicKey(),
		store:     store,
	}
}

// VerifyToken verifies an OAuth access token.
func (v *OAuthVerifier) VerifyToken(tokenString string) (*UserContext, error) {
	if v == nil {
		return nil, fmt.Errorf("OAuth not configured")
	}

	token, err := jwt.ParseWithClaims(tokenString, &OAuthClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*OAuthClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	if claims.Issuer != v.issuer {
		return nil, fmt.Errorf("issuer mismatch")
	}
	if len(claims.Audience) == 0 || !audienceContains(claims.Audience, v.audience) {
		return nil, fmt.Errorf("audience mismatch")
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("missing subject")
	}

	if v.store != nil && claims.ID != "" {
		revoked, err := v.store.IsAccessTokenRevoked(claims.ID)
		if err != nil {
			return nil, fmt.Errorf("token revocation check failed")
		}
		if revoked {
			return nil, fmt.Errorf("token revoked")
		}
	}

	return &UserContext{
		UserID:    claims.Subject,
		Email:     claims.Email,
		SessionID: "",
	}, nil
}

func audienceContains(values jwt.ClaimStrings, target string) bool {
	for _, val := range values {
		if val == target {
			return true
		}
	}
	return false
}
