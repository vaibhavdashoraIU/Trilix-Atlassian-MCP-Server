package auth

import (
	"context"
	"fmt"
	"net/http"
)

// AuthMiddleware creates HTTP middleware for authentication
type AuthMiddleware struct {
	clerkAuth *ClerkAuth
	optional  bool
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(clerkAuth *ClerkAuth, optional bool) *AuthMiddleware {
	return &AuthMiddleware{
		clerkAuth: clerkAuth,
		optional:  optional,
	}
}

// Handler wraps an HTTP handler with authentication
func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow OPTIONS requests (CORS preflight) to pass through without auth
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}
		
		// Try to extract token from header first
		token := ExtractTokenFromHeader(r)
		
		// If not in header, try query parameter (for SSE)
		if token == "" {
			token = ExtractTokenFromQuery(r)
		}

		// If no token and auth is required, return 401
		if token == "" {
			if !m.optional {
				http.Error(w, "Unauthorized: missing authentication token", http.StatusUnauthorized)
				return
			}
			// Optional auth - continue without user context
			next.ServeHTTP(w, r)
			return
		}

		// Verify token
		userCtx, err := m.clerkAuth.VerifyToken(token)
		if err != nil {
			if !m.optional {
				http.Error(w, fmt.Sprintf("Unauthorized: %v", err), http.StatusUnauthorized)
				return
			}
			// Optional auth - continue without user context
			next.ServeHTTP(w, r)
			return
		}

		// Inject user context into request
		ctx := context.WithValue(r.Context(), UserContextKey, userCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HandlerFunc wraps an HTTP handler function with authentication
func (m *AuthMiddleware) HandlerFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.Handler(next).ServeHTTP(w, r)
	}
}

// RequireAuth creates middleware that requires authentication
func RequireAuth(clerkAuth *ClerkAuth) *AuthMiddleware {
	return NewAuthMiddleware(clerkAuth, false)
}

// OptionalAuth creates middleware that allows optional authentication
func OptionalAuth(clerkAuth *ClerkAuth) *AuthMiddleware {
	return NewAuthMiddleware(clerkAuth, true)
}
