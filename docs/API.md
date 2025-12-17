# Trilix Atlassian MCP Server API Documentation

## Overview

The MCP server provides two main APIs:
1. **Workspace Management API** (Port 3002) - Manage Atlassian workspace credentials
2. **MCP SSE API** (Port 3001) - Execute MCP tools with Server-Sent Events streaming

## Authentication

All API endpoints (except `/health`) require Clerk authentication.

### Getting a JWT Token

Frontend applications should use Clerk's JavaScript SDK:

```javascript
const token = await window.Clerk.session.getToken();
```

### Using the Token

**For HTTP requests:**
```
Authorization: Bearer <jwt_token>
```

**For SSE connections:**
```
/mcp/stream?token=<jwt_token>
```

---

## Workspace Management API (Port 3002)

### Health Check

**GET /health**

No authentication required.

**Response:**
```json
{
  "status": "ok"
}
```

---

### Create Workspace

**POST /api/workspaces**

Add a new Atlassian workspace with API credentials.

**Headers:**
```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request Body:**
```json
{
  "workspaceName": "My Company",
  "siteUrl": "https://mycompany.atlassian.net",
  "email": "user@mycompany.com",
  "apiToken": "ATATT3xFfGF0..."
}
```

**Response (201 Created):**
```json
{
  "workspaceId": "550e8400-e29b-41d4-a716-446655440000",
  "workspaceName": "My Company",
  "siteUrl": "https://mycompany.atlassian.net",
  "email": "user@mycompany.com",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:30:00Z"
}
```

**Error Responses:**
- `400 Bad Request` - Missing required fields
- `401 Unauthorized` - Invalid Atlassian credentials
- `500 Internal Server Error` - Failed to save credentials

---

### List Workspaces

**GET /api/workspaces**

List all workspaces for the authenticated user.

**Headers:**
```
Authorization: Bearer <jwt_token>
```

**Response (200 OK):**
```json
[
  {
    "workspaceId": "550e8400-e29b-41d4-a716-446655440000",
    "workspaceName": "My Company",
    "siteUrl": "https://mycompany.atlassian.net",
    "email": "user@mycompany.com",
    "createdAt": "2024-01-15T10:30:00Z",
    "updatedAt": "2024-01-15T10:30:00Z"
  }
]
```

---

### Delete Workspace

**DELETE /api/workspaces/:id**

Remove a workspace and its credentials.

**Headers:**
```
Authorization: Bearer <jwt_token>
```

**Response:**
- `204 No Content` - Successfully deleted
- `404 Not Found` - Workspace not found
- `500 Internal Server Error` - Failed to delete

---

### Check Workspace Status

**GET /api/workspaces/:id/status**

Test connectivity to an Atlassian workspace.

**Headers:**
```
Authorization: Bearer <jwt_token>
```

**Response (200 OK):**
```json
{
  "workspaceId": "550e8400-e29b-41d4-a716-446655440000",
  "connected": true
}
```

**Or if connection failed:**
```json
{
  "workspaceId": "550e8400-e29b-41d4-a716-446655440000",
  "connected": false,
  "error": "authentication failed"
}
```

---

## MCP SSE API (Port 3001)

### SSE Connection

**GET /sse**

Establish a Server-Sent Events connection.

**Query Parameters:**
- `token` - Clerk JWT token (optional for now, required in production)

**Response:**
```
Content-Type: text/event-stream

event: endpoint
data: /message
```

---

### Initialize MCP

**POST /message**

Initialize the MCP protocol.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {}
    },
    "serverInfo": {
      "name": "trilix-atlassian-mcp-server",
      "version": "1.0.0"
    }
  }
}
```

---

### List Tools

**POST /message**

List all available MCP tools.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "confluence_list_spaces",
        "description": "List all Confluence spaces",
        "inputSchema": { ... }
      },
      ...
    ]
  }
}
```

---

### Call Tool

**POST /message**

Execute an MCP tool.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "confluence_list_spaces",
    "arguments": {
      "workspace_id": "550e8400-e29b-41d4-a716-446655440000"
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "..."
      }
    ]
  }
}
```

---

## Frontend Integration Example

### HTML + Clerk

```html
<!DOCTYPE html>
<html>
<head>
  <title>MCP Client</title>
  <script src="https://cdn.clerk.com/clerk.browser.js"></script>
</head>
<body>
  <div id="clerk-mountpoint"></div>
  <button onclick="addWorkspace()">Add Workspace</button>
  <button onclick="listWorkspaces()">List Workspaces</button>

  <script>
    const clerkPubKey = 'pk_test_...'; // Your Clerk publishable key
    
    window.Clerk.load({
      publishableKey: clerkPubKey
    });

    async function getToken() {
      if (!window.Clerk.session) {
        alert('Please sign in first');
        return null;
      }
      return await window.Clerk.session.getToken();
    }

    async function addWorkspace() {
      const token = await getToken();
      if (!token) return;

      const response = await fetch('http://localhost:3002/api/workspaces', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          workspaceName: 'My Company',
          siteUrl: 'https://mycompany.atlassian.net',
          email: 'user@mycompany.com',
          apiToken: 'ATATT3xFfGF0...'
        })
      });

      const data = await response.json();
      console.log('Workspace created:', data);
    }

    async function listWorkspaces() {
      const token = await getToken();
      if (!token) return;

      const response = await fetch('http://localhost:3002/api/workspaces', {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });

      const data = await response.json();
      console.log('Workspaces:', data);
    }

    // Mount Clerk UI
    window.Clerk.mountUserButton(
      document.getElementById('clerk-mountpoint')
    );
  </script>
</body>
</html>
```

---

## Security Notes

1. **API tokens are never returned** - Once stored, Atlassian API tokens are never included in API responses
2. **Tokens are encrypted** - When using database storage, tokens are encrypted at rest
3. **JWT verification** - All requests are verified using Clerk's public keys
4. **User isolation** - Users can only access their own workspaces
5. **HTTPS required** - Always use HTTPS in production to protect tokens in transit

---

## Error Codes

| Code | Description |
|------|-------------|
| 400  | Bad Request - Invalid input |
| 401  | Unauthorized - Invalid or missing authentication |
| 403  | Forbidden - Insufficient permissions |
| 404  | Not Found - Resource doesn't exist |
| 500  | Internal Server Error - Server-side error |
