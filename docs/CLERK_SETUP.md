# Clerk Authentication Setup Guide

## Quick Start

### 1. Install Dependencies

```bash
go mod download
```

### 2. Configure Clerk

1. Create a Clerk account at https://clerk.com
2. Create a new application
3. Get your keys from the Clerk dashboard:
   - **Secret Key** (starts with `sk_test_` or `sk_live_`)
   - **Publishable Key** (starts with `pk_test_` or `pk_live_`)

### 3. Configure Environment

```bash
# Copy sample environment file
cp sample.env .env

# Edit .env and add your Clerk secret key
# CLERK_SECRET_KEY=sk_test_xxxxx
```

### 4. Run the Server

We have provided convenience scripts to run the unified server and all required services (Frontend + Backend + Microservices).

**Mac/Linux:**
```bash
# Start
./start-services.sh

# Stop
./stop-services.sh
```

**Windows:**
```powershell
# Start
.\start-services.ps1

# Stop
.\stop-services.ps1
```

**Expected output:**
```
🚀 Starting Unified Trilix Server on port 3000...
   - Dashboard:    http://localhost:3000/
   - Test Client:  http://localhost:3000/docs/test-client.html
   - API:          http://localhost:3000/api/workspaces
   - SSE:          http://localhost:3000/sse
```

### 5. Check Services

Open http://localhost:3000/docs/test-client.html in your browser.

## API Endpoints (All on Port 3000)

### Workspace Management
- `GET /api/workspaces` - List workspaces
- `POST /api/workspaces` - Create workspace
- `GET /api/workspaces/:id/status` - Check status

### MCP Tool Execution (REST)
- `POST /api/tools/:tool_name` - Run any tool (e.g., `confluence_list_spaces`, `jira_list_issues`)

### MCP SSE
- `GET /sse` - Establish SSE connection
- `POST /message` - Execute MCP commands via SSE

## Authentication Flow (OAuth 2.1 + Clerk)

Clerk is **only** the Identity Provider (login). The MCP server issues OAuth tokens.

```
1. MCP client starts OAuth 2.1 + PKCE at /oauth/authorize
2. User signs in via Clerk on the hosted authorize page
3. Server issues an authorization code
4. MCP client exchanges code at /oauth/token
5. Server issues access/refresh tokens (issuer = your OAuth server)
6. MCP API calls use Authorization: Bearer <access_token>
```

## Security Features

✅ OAuth 2.1 + PKCE  
✅ JWT signature verification (server-issued tokens)  
✅ API tokens encrypted at rest  
✅ Tokens never exposed in responses  
✅ User isolation (users can only access their own data)  
✅ CORS configured for frontend integration  

## Development Mode

If `CLERK_SECRET_KEY` is not set, the server runs in development mode without authentication. This is useful for local testing but **should never be used in production**.

## Production Deployment

1. Use `sk_live_` Clerk secret key
2. Set up PostgreSQL database (required for OAuth)
3. Set up Redis for auth code/session storage (recommended)
4. Configure `DATABASE_URL`, `API_KEY_ENCRYPTION_KEY`, and OAuth envs
5. Remove `WORKSPACES_FILE` from environment
6. Deploy behind HTTPS reverse proxy
7. Update CORS origins to specific domains

## Troubleshooting

### "Clerk authentication not configured"
- Check that `CLERK_SECRET_KEY` is set in `.env`
- Verify the key starts with `sk_test_` or `sk_live_`

### "401 Unauthorized"
- Verify OAuth access token is valid
- Check token issuer/audience/expiry
- Ensure Clerk login succeeds during /oauth/authorize

### "Failed to fetch JWKS"
- Check internet connectivity
- Verify `CLERK_SECRET_KEY` is correct
- Check firewall settings

## Documentation

- [API Documentation](./docs/API.md) - Complete API reference
- [Test Client](./docs/test-client.html) - Interactive test page
- [Walkthrough](../.gemini/antigravity/brain/8e9ee788-5f33-4b5a-8d05-0c6b8beb6392/walkthrough.md) - Implementation details

## Support

For issues or questions:
1. Check the API documentation
2. Review the walkthrough document
3. Test with the HTML client
4. Check Clerk dashboard for authentication issues

---

## OAuth 2.1 for LLMs (Dynamic Client Registration)

### 1) Configure OAuth
Set these variables in `.env`:
```env
OAUTH_ISSUER=https://trilix-eso-auth.aws.providentiaworldwide.com
OAUTH_AUDIENCE=https://trilix-eso-auth.aws.providentiaworldwide.com
OAUTH_PRIVATE_KEY_PEM="-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----"
OAUTH_DCR_MODE=protected
OAUTH_DCR_ACCESS_TOKEN=your-dcr-admin-token
OAUTH_ACCESS_TOKEN_TTL=60m
OAUTH_REFRESH_TOKEN_TTL=720h
OAUTH_AUTH_CODE_TTL=10m
CLERK_PUBLISHABLE_KEY=pk_live_xxx
REDIS_URL=redis://localhost:6379/0
```

For local development, set:
```env
OAUTH_ISSUER=http://localhost:3000
OAUTH_AUDIENCE=http://localhost:3000
```

### 2) Register OAuth Client (Dynamic)
```bash
curl -X POST https://trilix-eso-auth.aws.providentiaworldwide.com/oauth/register \
  -H "Authorization: Bearer your-dcr-admin-token" \
  -H "Content-Type: application/json" \
  -d '{
    "redirect_uris": ["https://your-mcp-client/callback"],
    "client_name": "Any LLM Client",
    "token_endpoint_auth_method": "none"
  }'
```

### 3) Use OAuth 2.1 + PKCE
Discovery document:
```
GET /.well-known/oauth-authorization-server
```

Authorization endpoint:
```
GET /oauth/authorize
```

Token endpoint:
```
POST /oauth/token
```


curl -X POST https://trilix-eso.aws.providentiaworldwide.com/oauth/register \
  -H "Authorization: Bearer 99713cc1e21c6d7564a3076a504bcb6487879a6db32510cae521e2c1559038df" \
  -H "Content-Type: application/json" \
  -d '{
    "redirect_uris": [
      "https://chat.openai.com/aip/g-*/oauth/callback",
      "https://chatgpt.com/aip/g-*/oauth/callback"
    ],
    "client_name": "ChatGPT",
    "token_endpoint_auth_method": "client_secret_post"
  }'


https://trilix-eso.aws.providentiaworldwide.com/oauth/authorize
https://trilix-eso.aws.providentiaworldwide.com/oauth/token
