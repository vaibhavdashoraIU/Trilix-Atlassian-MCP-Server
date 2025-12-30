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

## Authentication Flow

```
1. User signs in via Clerk (frontend)
2. Frontend obtains JWT token
3. Frontend sends requests with Authorization: Bearer <token>
4. Server verifies JWT using Clerk's public keys
5. Server extracts user identity
6. Server executes request with user context
```

## Security Features

✅ JWT signature verification  
✅ API tokens encrypted at rest  
✅ Tokens never exposed in responses  
✅ User isolation (users can only access their own data)  
✅ CORS configured for frontend integration  

## Development Mode

If `CLERK_SECRET_KEY` is not set, the server runs in development mode without authentication. This is useful for local testing but **should never be used in production**.

## Production Deployment

1. Use `sk_live_` Clerk secret key
2. Set up PostgreSQL database
3. Configure `DATABASE_URL` and `API_KEY_ENCRYPTION_KEY`
4. Remove `WORKSPACES_FILE` from environment
5. Deploy behind HTTPS reverse proxy
6. Update CORS origins to specific domains

## Troubleshooting

### "Clerk authentication not configured"
- Check that `CLERK_SECRET_KEY` is set in `.env`
- Verify the key starts with `sk_test_` or `sk_live_`

### "401 Unauthorized"
- Verify JWT token is valid
- Check that token is not expired
- Ensure Clerk SDK is properly initialized in frontend

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

## 4. Service Tokens (Bots/AI)
For automated tools like ChatGPT or other bots that cannot perform interactive login, use the **Service Token** flow.

### Configuration
Add `MCP_SERVICE_TOKEN` to your `.env` or Secrets:
```env
MCP_SERVICE_TOKEN=your-secure-static-secret-123
```

### Usage (API / ChatGPT)
1.  **Authentication**: Use the token as a Bearer token.
    ```
    Authorization: Bearer your-secure-static-secret-123
    ```

2.  **Impersonation**: Since the token is generic, you can specify which user to act as by including `user_id` in the tool arguments.
    ```json
    {
      "user_id": "user_2px... (Target Clerk User ID)",
      "workspace_id": "...",
      ...
    }
    ```

> [!NOTE]
> The `user_id` argument is **only** accepted when authenticated via `MCP_SERVICE_TOKEN`. Regular user tokens cannot impersonate others.
