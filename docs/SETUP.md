# Setup Guide for Trilix Atlassian MCP Server

This guide will help you set up and run the Trilix Atlassian MCP Server locally.

## Prerequisites

- Go 1.22 or later
- Docker and Docker Compose (for local infrastructure)
- TwistyGo library (cloned at `D:\Idea Usher\twistygo`)

## Step 1: Start Infrastructure Services

Start RabbitMQ and PostgreSQL using Docker Compose:

```bash
docker-compose up -d
```

This will start:
- **RabbitMQ** on port 5672 (Management UI on port 15672)
- **PostgreSQL** on port 5432

You can verify they're running:
```bash
docker-compose ps
```

Access RabbitMQ Management UI: http://localhost:15672
- Username: `trilix`
- Password: `secret`

## Step 2: Configure Environment Variables

Copy the example environment file and update it:

```bash
copy .env.example .env

# for mac
cp sample.env .env
```

Edit `.env` and set:
- `API_KEY_ENCRYPTION_KEY`: Generate a 32-byte key:
  ```bash
  # On Windows PowerShell:
  [Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Minimum 0 -Maximum 256 }))
  
  # Or use OpenSSL if available:
  openssl rand -hex 32
  ```
- `CLERK_SECRET_KEY`: (Optional) Your Clerk secret key if using authentication

## Step 3: Install Go Dependencies

```bash
go mod download
```

This will download all dependencies. The TwistyGo library will be used from the local path specified in `go.mod`.

## Step 4: Start the Services

You need to run three services in separate terminals:

### Terminal 1: Confluence Service

```bash
cd cmd/confluence-service
go run main.go
```

You should see:
```
[INFO] Starting service: ConfluenceService v1.0.0
```

### Terminal 2: Jira Service

```bash
cd cmd/jira-service
go run main.go
```

You should see:
```
[INFO] Starting service: JiraService v1.0.0
```

### Terminal 3: MCP Server

```bash
cd cmd/mcp-server
go run main.go
```

The MCP server will start and listen on stdio for MCP protocol messages.

## Step 5: Verify Services Are Running

Check that all services are connected to RabbitMQ:

1. Open RabbitMQ Management UI: http://localhost:15672
2. Go to **Queues** tab
3. You should see:
   - `confluence.requests` queue
   - `jira.requests` queue

## Step 6: Test the MCP Server

The MCP server communicates via stdio using the Model Context Protocol. You can test it by sending JSON-RPC messages:

### Example: List Tools

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list"
}
```

### Example: Initialize

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "test-client",
      "version": "1.0.0"
    }
  }
}
```

## Troubleshooting

### Services won't start

1. **Check RabbitMQ is running:**
   ```bash
   docker-compose ps
   ```

2. **Check environment variables:**
   Make sure `.env` file exists and has all required variables

3. **Check TwistyGo path:**
   Verify `D:\Idea Usher\twistygo` exists and contains the TwistyGo source code

4. **Check database connection:**
   ```bash
   # Test PostgreSQL connection
   psql -h localhost -U trilix -d trilix_mcp
   # Password: secret
   ```

### RabbitMQ Connection Errors

- Verify RabbitMQ is running: `docker-compose ps`
- Check credentials in `.env` match `docker-compose.yaml`
- Check RabbitMQ logs: `docker-compose logs rabbitmq`

### Database Errors

- Verify PostgreSQL is running: `docker-compose ps`
- Check database exists: `docker-compose exec postgres psql -U trilix -l`
- Check connection string in `.env`

### TwistyGo Import Errors

If you see import errors for TwistyGo:

1. Verify the path in `go.mod` is correct:
   ```
   replace github.com/providentiaww/twistygo => D:/Idea Usher/twistygo
   ```

2. Run `go mod tidy` to refresh dependencies

3. Check that TwistyGo has a valid `go.mod` file

## Development Workflow

1. **Make changes** to any service code
2. **Restart the service** (Ctrl+C and run `go run main.go` again)
3. **Check logs** for any errors

## Building for Production

Build all services:

```bash
# Build MCP Server
go build -o bin/mcp-server ./cmd/mcp-server

# Build Confluence Service
go build -o bin/confluence-service ./cmd/confluence-service

# Build Jira Service
go build -o bin/jira-service ./cmd/jira-service
```

## Step 7: Configure Multiple Workspaces

The MCP server supports connecting to **multiple Atlassian workspaces simultaneously**. This allows users to query Confluence or Jira from different organizations in the same chat session.

### Using File-Based Storage (Recommended for Local Development)

1. Edit `.config/workspaces.json` and add multiple workspace configurations:

```json
[
  {
    "name": "workspace-1",
    "baseUrl": "https://example1.atlassian.net",
    "email": "user@example.com",
    "apiToken": "your-api-token-here"
  },
  {
    "name": "workspace-2",
    "baseUrl": "https://example2.atlassian.net",
    "email": "user@example.com",
    "apiToken": "your-api-token-here"
  },
  {
    "name": "providentia",
    "baseUrl": "https://providentiaworldwide.atlassian.net",
    "email": "user@example.com",
    "apiToken": "your-api-token-here"
  }
]
```

2. The `name` field becomes the `workspace_id` used in API calls
3. Set `WORKSPACES_FILE=.config/workspaces.json` in your `.env` file

### Using Multiple Workspaces in ChatGPT

When using the MCP server with ChatGPT, you can query different workspaces in the same conversation:

- **List available workspaces:** Use the `list_workspaces` tool
- **Query a specific workspace:** Include `workspace_id` in your tool calls:
  - `confluence_get_page` with `workspace_id: "workspace-1"`
  - `jira_list_issues` with `workspace_id: "workspace-2"`
  - Switch between workspaces seamlessly in the same chat

### Example Usage

```json
// Get a page from workspace-1
{
  "tool": "confluence_get_page",
  "arguments": {
    "workspace_id": "workspace-1",
    "page_id": "123456"
  }
}

// Get issues from workspace-2
{
  "tool": "jira_list_issues",
  "arguments": {
    "workspace_id": "workspace-2",
    "jql": "project = PROJ"
  }
}
```

## Next Steps

- Add multiple workspaces to `.config/workspaces.json`
- Test querying different workspaces in the same session
- Configure Clerk authentication (optional)
- Deploy to Kubernetes using `manifest.yaml` and `twistydeploy`

## Architecture Overview

```
┌─────────────────┐
│   MCP Server    │ (stdio)
└────────┬────────┘
         │ RabbitMQ
    ┌────┴────┐
    │         │
┌───▼───┐ ┌──▼───┐
│Confl. │ │ Jira │
│Service│ │Service│
└───────┘ └──────┘
    │         │
    └────┬────┘
         │
    ┌────▼────┐
    │PostgreSQL│
    └─────────┘
```

All services communicate via RabbitMQ using TwistyGo RPC pattern.

