package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/providentiaww/trilix-atlassian-mcp/cmd/mcp-server/auth"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/oauth"
	"golang.org/x/crypto/bcrypt"
)

// Server provides OAuth 2.1 endpoints.
type Server struct {
	cfg       oauth.Config
	keys      *oauth.KeyManager
	store     *oauth.Store
	clerkAuth *auth.ClerkAuth
}

// NewServer creates a new OAuth server.
func NewServer(cfg oauth.Config, keys *oauth.KeyManager, store *oauth.Store, clerkAuth *auth.ClerkAuth) *Server {
	return &Server{
		cfg:       cfg,
		keys:      keys,
		store:     store,
		clerkAuth: clerkAuth,
	}
}

// HandleAuthorize processes OAuth authorization requests.
func (s *Server) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req, err := s.parseAuthorizeRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userCtx, err := s.authenticateRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if userCtx == nil {
		if s.clerkAuth == nil {
			http.Error(w, "Clerk authentication not configured", http.StatusInternalServerError)
			return
		}
		if err := s.store.SaveAuthRequest(req); err != nil {
			http.Error(w, "Failed to store auth request", http.StatusInternalServerError)
			return
		}
		s.renderLoginPage(w, req.RequestID)
		return
	}

	redirectURL, err := s.issueAuthCode(req, userCtx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleAuthorizeComplete finalizes the OAuth authorization after Clerk login.
func (s *Server) HandleAuthorizeComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		fmt.Printf("OAuth authorize complete error: method not allowed (%s)\n", r.Method)
		return
	}

	var payload struct {
		RequestID  string `json:"request_id"`
		ClerkToken string `json:"clerk_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		fmt.Printf("OAuth authorize complete error: invalid JSON payload: %v\n", err)
		return
	}
	if payload.RequestID == "" || payload.ClerkToken == "" {
		http.Error(w, "Missing request_id or clerk_token", http.StatusBadRequest)
		fmt.Printf("OAuth authorize complete error: missing request_id or clerk_token\n")
		return
	}

	req, err := s.store.GetAuthRequest(payload.RequestID)
	if err != nil {
		http.Error(w, "Invalid or expired authorization request", http.StatusBadRequest)
		fmt.Printf("OAuth authorize complete error: invalid or expired auth request\n")
		return
	}
	_ = s.store.DeleteAuthRequest(payload.RequestID)

	if s.clerkAuth == nil {
		http.Error(w, "Clerk authentication not configured", http.StatusInternalServerError)
		fmt.Printf("OAuth authorize complete error: Clerk auth not configured\n")
		return
	}

	userCtx, err := s.clerkAuth.VerifyToken(payload.ClerkToken)
	if err != nil {
		http.Error(w, "Invalid Clerk token", http.StatusUnauthorized)
		fmt.Printf("OAuth authorize complete error: invalid Clerk token\n")
		return
	}

	redirectURL, err := s.issueAuthCode(req, userCtx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		fmt.Printf("OAuth authorize complete error: issue auth code failed: %v\n", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"redirect_to": redirectURL})
}

// HandleToken exchanges authorization codes or refresh tokens.
func (s *Server) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		fmt.Printf("OAuth token error: method not allowed (%s)\n", r.Method)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form body", http.StatusBadRequest)
		fmt.Printf("OAuth token error: invalid form body: %v\n", err)
		return
	}

	grantType := r.FormValue("grant_type")
	switch grantType {
	case "authorization_code":
		s.handleAuthorizationCodeGrant(w, r)
	case "refresh_token":
		s.handleRefreshTokenGrant(w, r)
	default:
		http.Error(w, "Unsupported grant_type", http.StatusBadRequest)
		fmt.Printf("OAuth token error: unsupported grant_type=%s\n", grantType)
	}
}

// HandleWellKnown serves OAuth discovery metadata.
func (s *Server) HandleWellKnown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	issuer := s.cfg.Issuer
	data := map[string]interface{}{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/oauth/authorize",
		"token_endpoint":                        issuer + "/oauth/token",
		"jwks_uri":                              issuer + "/oauth/jwks",
		"registration_endpoint":                 issuer + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_post"},
	}

	writeJSON(w, http.StatusOK, data)
}

// HandleJWKS serves the JWKS public keys.
func (s *Server) HandleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pub := s.keys.PublicKey()
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(bigIntToBytes(big.NewInt(int64(pub.E))))

	keys := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": s.keys.KID(),
				"alg": "RS256",
				"n":   n,
				"e":   e,
			},
		},
	}

	writeJSON(w, http.StatusOK, keys)
}

// HandleRegister registers dynamic clients.
func (s *Server) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.cfg.DCRMode == "protected" {
		if !s.checkDCRAccess(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	var req struct {
		RedirectURIs            []string `json:"redirect_uris"`
		ClientName              string   `json:"client_name"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
		Scope                   string   `json:"scope"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if len(req.RedirectURIs) == 0 {
		http.Error(w, "redirect_uris is required", http.StatusBadRequest)
		return
	}
	for _, uri := range req.RedirectURIs {
		if err := validateRedirectURI(uri); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if len(req.GrantTypes) == 0 {
		req.GrantTypes = []string{"authorization_code", "refresh_token"}
	}
	if len(req.ResponseTypes) == 0 {
		req.ResponseTypes = []string{"code"}
	}
	if req.TokenEndpointAuthMethod == "" {
		req.TokenEndpointAuthMethod = "none"
	}

	clientID, err := oauthRandomID("client")
	if err != nil {
		http.Error(w, "Failed to generate client_id", http.StatusInternalServerError)
		return
	}

	var clientSecret string
	var clientSecretHash string
	if req.TokenEndpointAuthMethod != "none" {
		clientSecret, err = oauthRandomSecret()
		if err != nil {
			http.Error(w, "Failed to generate client_secret", http.StatusInternalServerError)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Failed to hash client_secret", http.StatusInternalServerError)
			return
		}
		clientSecretHash = string(hash)
	}

	client := &oauth.Client{
		ClientID:                clientID,
		ClientSecretHash:        clientSecretHash,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              req.GrantTypes,
		ResponseTypes:           req.ResponseTypes,
		Scope:                   req.Scope,
		TokenEndpointAuthMethod: req.TokenEndpointAuthMethod,
		ClientName:              req.ClientName,
	}

	if err := s.store.SaveClient(client); err != nil {
		http.Error(w, "Failed to store client", http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"client_id":                  clientID,
		"client_id_issued_at":        time.Now().Unix(),
		"client_secret_expires_at":   0,
		"redirect_uris":              req.RedirectURIs,
		"grant_types":                req.GrantTypes,
		"response_types":             req.ResponseTypes,
		"token_endpoint_auth_method": req.TokenEndpointAuthMethod,
		"client_name":                req.ClientName,
		"scope":                      req.Scope,
	}
	if clientSecret != "" {
		resp["client_secret"] = clientSecret
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "Missing code", http.StatusBadRequest)
		fmt.Printf("OAuth token error: missing code\n")
		return
	}

	client, err := s.authenticateClient(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		fmt.Printf("OAuth token error: client auth failed: %v\n", err)
		return
	}

	codeHash := oauthHash(code)
	authCode, err := s.store.ConsumeAuthCode(codeHash)
	if err != nil {
		http.Error(w, "Invalid or expired code", http.StatusBadRequest)
		fmt.Printf("OAuth token error: invalid or expired code\n")
		return
	}

	if time.Now().After(authCode.ExpiresAt) {
		http.Error(w, "Authorization code expired", http.StatusBadRequest)
		fmt.Printf("OAuth token error: code expired\n")
		return
	}

	if authCode.ClientID != client.ClientID {
		http.Error(w, "Client mismatch", http.StatusBadRequest)
		fmt.Printf("OAuth token error: client mismatch\n")
		return
	}

	redirectURI := r.FormValue("redirect_uri")
	if redirectURI == "" || redirectURI != authCode.RedirectURI {
		http.Error(w, "redirect_uri mismatch", http.StatusBadRequest)
		fmt.Printf("OAuth token error: redirect_uri mismatch\n")
		return
	}

	codeVerifier := r.FormValue("code_verifier")
	if err := verifyPKCE(authCode, codeVerifier); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		fmt.Printf("OAuth token error: pkce verification failed: %v\n", err)
		return
	}

	accessToken, refreshToken, expiresIn, err := s.issueTokens(authCode.UserID, authCode.Scope, client.ClientID)
	if err != nil {
		http.Error(w, "Failed to issue tokens", http.StatusInternalServerError)
		fmt.Printf("OAuth token error: token issuance failed: %v\n", err)
		return
	}

	response := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    int(expiresIn.Seconds()),
		"refresh_token": refreshToken,
		"scope":         authCode.Scope,
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.FormValue("refresh_token")
	if refreshToken == "" {
		http.Error(w, "Missing refresh_token", http.StatusBadRequest)
		fmt.Printf("OAuth token error: missing refresh_token\n")
		return
	}

	client, err := s.authenticateClient(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		fmt.Printf("OAuth token error: client auth failed: %v\n", err)
		return
	}

	hash := oauthHash(refreshToken)
	stored, err := s.store.GetRefreshToken(hash)
	if err != nil || stored.RevokedAt != nil || time.Now().After(stored.ExpiresAt) {
		http.Error(w, "Invalid refresh_token", http.StatusBadRequest)
		fmt.Printf("OAuth token error: invalid refresh_token\n")
		return
	}

	if stored.ClientID != client.ClientID {
		http.Error(w, "Client mismatch", http.StatusBadRequest)
		fmt.Printf("OAuth token error: client mismatch\n")
		return
	}

	_ = s.store.RevokeRefreshToken(hash)

	accessToken, newRefresh, expiresIn, err := s.issueTokens(stored.UserID, stored.Scope, client.ClientID)
	if err != nil {
		http.Error(w, "Failed to issue tokens", http.StatusInternalServerError)
		fmt.Printf("OAuth token error: token issuance failed: %v\n", err)
		return
	}

	response := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    int(expiresIn.Seconds()),
		"refresh_token": newRefresh,
		"scope":         stored.Scope,
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) authenticateClient(r *http.Request) (*oauth.Client, error) {
	clientID := r.FormValue("client_id")
	if clientID == "" {
		clientID = r.PostFormValue("client_id")
	}
	if clientID == "" {
		return nil, fmt.Errorf("client_id required")
	}

	client, err := s.store.GetClient(clientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client_id")
	}

	if client.TokenEndpointAuthMethod == "none" {
		return client, nil
	}

	secret := r.FormValue("client_secret")
	if secret == "" {
		return nil, fmt.Errorf("client_secret required")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(client.ClientSecretHash), []byte(secret)); err != nil {
		return nil, fmt.Errorf("invalid client_secret")
	}
	return client, nil
}

func (s *Server) parseAuthorizeRequest(r *http.Request) (*oauth.AuthRequest, error) {
	query := r.URL.Query()
	responseType := query.Get("response_type")
	if responseType != "code" {
		return nil, fmt.Errorf("unsupported response_type")
	}

	clientID := query.Get("client_id")
	if clientID == "" {
		return nil, fmt.Errorf("client_id required")
	}

	client, err := s.store.GetClient(clientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client_id")
	}

	redirectURI := query.Get("redirect_uri")
	if redirectURI == "" {
		return nil, fmt.Errorf("redirect_uri required")
	}

	if !isRedirectAllowed(redirectURI, client.RedirectURIs) {
		return nil, fmt.Errorf("redirect_uri not allowed")
	}

	codeChallenge := query.Get("code_challenge")
	codeChallengeMethod := query.Get("code_challenge_method")
	if codeChallenge == "" {
		if client.TokenEndpointAuthMethod == "none" {
			return nil, fmt.Errorf("PKCE S256 is required")
		}
		codeChallengeMethod = "none"
	} else if strings.ToUpper(codeChallengeMethod) != "S256" {
		return nil, fmt.Errorf("PKCE S256 is required")
	}

	scope := strings.TrimSpace(query.Get("scope"))
	state := query.Get("state")

	requestID := uuid.New().String()
	now := time.Now()
	return &oauth.AuthRequest{
		RequestID:           requestID,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		State:               state,
		ResponseType:        responseType,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: strings.ToUpper(codeChallengeMethod),
		CreatedAt:           now,
		ExpiresAt:           now.Add(s.cfg.AuthCodeTTL),
	}, nil
}

func (s *Server) issueAuthCode(req *oauth.AuthRequest, user *auth.UserContext) (string, error) {
	code, err := oauthRandomCode()
	if err != nil {
		return "", err
	}

	codeHash := oauthHash(code)
	now := time.Now()
	record := &oauth.AuthCode{
		CodeHash:            codeHash,
		ClientID:            req.ClientID,
		RedirectURI:         req.RedirectURI,
		UserID:              user.UserID,
		Scope:               req.Scope,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		CreatedAt:           now,
		ExpiresAt:           now.Add(s.cfg.AuthCodeTTL),
	}

	if err := s.store.SaveAuthCode(record); err != nil {
		return "", err
	}

	return buildRedirect(req.RedirectURI, code, req.State), nil
}

func (s *Server) issueTokens(userID, scope, clientID string) (string, string, time.Duration, error) {
	now := time.Now()
	jti := uuid.New().String()
	claims := jwt.MapClaims{
		"iss":       s.cfg.Issuer,
		"sub":       userID,
		"aud":       s.cfg.Audience,
		"iat":       now.Unix(),
		"exp":       now.Add(s.cfg.AccessTokenTTL).Unix(),
		"jti":       jti,
		"scope":     scope,
		"client_id": clientID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.keys.KID()

	signed, err := token.SignedString(s.keys.PrivateKey())
	if err != nil {
		return "", "", 0, err
	}

	if err := s.store.SaveAccessToken(&oauth.AccessToken{
		JTI:       jti,
		ClientID:  clientID,
		UserID:    userID,
		Scope:     scope,
		CreatedAt: now,
		ExpiresAt: now.Add(s.cfg.AccessTokenTTL),
	}); err != nil {
		return "", "", 0, err
	}

	refreshToken, err := oauthRandomSecret()
	if err != nil {
		return "", "", 0, err
	}

	refreshHash := oauthHash(refreshToken)
	if err := s.store.SaveRefreshToken(&oauth.RefreshToken{
		TokenHash: refreshHash,
		ClientID:  clientID,
		UserID:    userID,
		Scope:     scope,
		CreatedAt: now,
		ExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
	}); err != nil {
		return "", "", 0, err
	}

	return signed, refreshToken, s.cfg.AccessTokenTTL, nil
}

func (s *Server) authenticateRequest(r *http.Request) (*auth.UserContext, error) {
	if s.clerkAuth == nil {
		return nil, nil
	}

	token := auth.ExtractTokenFromHeader(r)
	if token == "" {
		token = r.URL.Query().Get("clerk_token")
	}
	if token == "" {
		return nil, nil
	}

	userCtx, err := s.clerkAuth.VerifyToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid Clerk token")
	}
	return userCtx, nil
}

func (s *Server) renderLoginPage(w http.ResponseWriter, requestID string) {
	if s.cfg.ClerkPublishableKey == "" {
		http.Error(w, "CLERK_PUBLISHABLE_KEY is required for OAuth login", http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Authorize Trilix MCP</title>
  <style>
    body { font-family: Arial, sans-serif; background:#0f172a; color:#e2e8f0; display:flex; align-items:center; justify-content:center; height:100vh; margin:0; }
    .card { background:#111827; border:1px solid #1f2937; padding:32px; border-radius:12px; max-width:420px; text-align:center; }
    h1 { margin:0 0 12px; font-size:22px; }
    p { margin:0 0 18px; color:#94a3b8; }
    #status { margin-top:16px; font-size:14px; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Authorize Trilix MCP</h1>
    <p>Sign in with Clerk to continue.</p>
    <div id="clerk-sign-in"></div>
    <div id="status"></div>
  </div>
  <script src="%s" data-clerk-publishable-key="%s"></script>
  <script>
    const requestId = %q;
    const statusEl = document.getElementById('status');

    function setStatus(msg) { statusEl.textContent = msg; }

    let finalized = false;
    async function finalizeOnce(clerkToken) {
      if (finalized) {
        return;
      }
      finalized = true;
      const res = await fetch('/oauth/authorize/complete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ request_id: requestId, clerk_token: clerkToken })
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok || !data.redirect_to) {
        setStatus(data.error || 'Authorization failed');
        return;
      }
      window.location = data.redirect_to;
    }

    async function initClerk() {
      if (!window.Clerk) {
        setStatus('Clerk failed to load.');
        return;
      }
      await window.Clerk.load();
      const currentUrl = window.location.href;
      window.Clerk.mountSignIn(document.getElementById('clerk-sign-in'), {
        afterSignInUrl: currentUrl,
        redirectUrl: currentUrl
      });
      window.Clerk.addListener(async ({ user }) => {
        if (user && window.Clerk.session) {
          const token = await window.Clerk.session.getToken();
          finalizeOnce(token);
        }
      });
      if (window.Clerk.user && window.Clerk.session) {
        const token = await window.Clerk.session.getToken();
        finalizeOnce(token);
      }
    }
    initClerk();
  </script>
</body>
</html>`, s.cfg.ClerkJSURL, s.cfg.ClerkPublishableKey, requestID)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

func (s *Server) checkDCRAccess(r *http.Request) bool {
	if s.cfg.DCRAccessToken == "" {
		return false
	}
	authHeader := r.Header.Get("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return false
	}
	return parts[1] == s.cfg.DCRAccessToken
}

func verifyPKCE(code *oauth.AuthCode, verifier string) error {
	if code.CodeChallengeMethod == "" || code.CodeChallengeMethod == "NONE" {
		return nil
	}
	if verifier == "" {
		return fmt.Errorf("code_verifier required")
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if challenge != code.CodeChallenge {
		return fmt.Errorf("invalid code_verifier")
	}
	return nil
}

func isRedirectAllowed(redirectURI string, allowed []string) bool {
	for _, uri := range allowed {
		if uri == redirectURI {
			return true
		}
		if strings.Contains(uri, "*") && wildcardMatch(uri, redirectURI) {
			return true
		}
	}
	return false
}

func validateRedirectURI(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid redirect_uri: %s", raw)
	}
	if strings.Contains(raw, "*") {
		return validateWildcardRedirect(parsed)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	host := strings.Split(parsed.Host, ":")[0]
	if parsed.Scheme == "http" && (host == "localhost" || host == "127.0.0.1") {
		return nil
	}
	return fmt.Errorf("redirect_uri must use https (or localhost http): %s", raw)
}

func validateWildcardRedirect(parsed *url.URL) error {
	if parsed.Scheme != "https" {
		return fmt.Errorf("wildcard redirect_uri must use https")
	}
	host := strings.Split(parsed.Host, ":")[0]
	if host != "chat.openai.com" && host != "chatgpt.com" {
		return fmt.Errorf("wildcard redirect_uri only allowed for chat.openai.com or chatgpt.com")
	}
	if !strings.HasPrefix(parsed.Path, "/aip/g-") || !strings.HasSuffix(parsed.Path, "/oauth/callback") {
		return fmt.Errorf("wildcard redirect_uri must match /aip/g-*/oauth/callback")
	}
	if !strings.Contains(parsed.Path, "*") {
		return fmt.Errorf("wildcard redirect_uri must include '*' in the path")
	}
	return nil
}

func wildcardMatch(pattern, value string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == value
	}
	if !strings.HasPrefix(value, parts[0]) {
		return false
	}
	rest := value[len(parts[0]):]
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		if part == "" {
			continue
		}
		idx := strings.Index(rest, part)
		if idx == -1 {
			return false
		}
		rest = rest[idx+len(part):]
	}
	return strings.HasSuffix(value, parts[len(parts)-1])
}

func buildRedirect(base, code, state string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func oauthRandomCode() (string, error) {
	return oauth.RandomString(32)
}

func oauthRandomSecret() (string, error) {
	return oauth.RandomString(48)
}

func oauthRandomID(prefix string) (string, error) {
	id, err := oauth.RandomString(18)
	if err != nil {
		return "", err
	}
	return prefix + "_" + id, nil
}

func oauthHash(value string) string {
	return oauth.HashToken(value)
}

func bigIntToBytes(value *big.Int) []byte {
	if value == nil {
		return []byte{0}
	}
	return value.Bytes()
}
