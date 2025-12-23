package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"
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


		// Check for Service Token (Static Trust)
		serviceToken := os.Getenv("MCP_SERVICE_TOKEN")
		if serviceToken != "" && token == serviceToken {
			// Create a "Service" user context
			// This is a PLACEHOLDER identity to satisfy the non-nil requirement of the context.
			// It is effectively ignored because we check for "user_id" overrides below.
			serviceUserCtx := &UserContext{
				UserID: "service_account",
				Email:  "service@mcp.system",
			}

			// Trusted Service Override: Extract user_id from query params or (if possible) the body
			// to impersonate a specific Clerk user.
			if injectedUser := r.URL.Query().Get("user_id"); injectedUser != "" {
				serviceUserCtx.UserID = injectedUser
				fmt.Printf("🔒 Service Override (Query): Using user_id=%s\n", injectedUser)
			}
			// Note: We don't parse the body here to avoid draining it for downstream handlers.
			// Downstream handlers (like RestToolHandler) will also check the body.

			ctx := context.WithValue(r.Context(), UserContextKey, serviceUserCtx)
			ctx = context.WithValue(ctx, "IsServiceCall", true)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Verify Clerk token
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
