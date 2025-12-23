# n8n Integration Guide

## MCP Server SSE Endpoint

The MCP server implements the Model Context Protocol (MCP) over Server-Sent Events (SSE) for n8n integration.

### MCP Endpoint for n8n
```
http://localhost:3000
```

**Important:** Use this base URL in n8n's MCP node configuration.

### MCP Protocol Endpoints

The server implements the standard MCP protocol:

#### 1. SSE Connection
```
GET /sse
```
Establishes Server-Sent Events connection for MCP protocol.

#### 2. Message Endpoint
```
POST /message
Content-Type: application/json
```
Handles MCP JSON-RPC requests (initialize, tools/list, tools/call).

---

## Available Tools

### Management Tools
- `list_workspaces` - List all configured workspaces
- `workspace_status` - Check workspace connectivity

### Confluence Tools
- `confluence_get_page` - Get a page by ID
- `confluence_search` - Search using CQL
- `confluence_create_page` - Create a new page
- `confluence_copy_page` - Copy page between workspaces
- `confluence_list_spaces` - List all spaces

### Jira Tools
- `jira_list_issues` - Search issues using JQL
- `jira_get_issue` - Get issue by key
- `jira_create_issue` - Create new issue
- `jira_update_issue` - Update existing issue
- `jira_add_comment` - Add comment to issue
- `jira_transition_issue` - Change issue status

---

## n8n Setup Instructions

### Step 1: Add MCP Agent Node
1. In your n8n workflow, add an **@n8n/n8n-nodes-langchain.agent** node
2. Or add **MCP Tool** node if available in your n8n version

### Step 2: Configure MCP Connection
1. In the MCP configuration:
   - **Transport Type**: `SSE` (Server-Sent Events)
   - **Base URL**: `http://localhost:3000`
   - **Authentication**: None (or configure if needed)

### Step 3: Available Tools
Once connected, n8n will automatically discover all available tools:
- `list_workspaces`
- `confluence_get_page`
- `confluence_search`
- `confluence_create_page`
- `jira_list_issues`
- `jira_get_issue`
- `jira_create_issue`
- And more...

### Step 4: Use Tools in Workflow
The MCP agent will automatically call the appropriate tools based on your prompts.

**Example Prompts:**
- "List all available workspaces"
- "Search for pages about 'API documentation' in Confluence"
- "Get Jira issue PROJ-123"
- "Create a new task in project PROJ with summary 'Review API docs'"

---

## Testing the MCP Server

### Test SSE Connection
```bash
curl http://localhost:3000/sse
```

### Test Initialize
```bash
curl -X POST http://localhost:3000/message \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {}
  }'
```

### Test List Tools
```bash
curl -X POST http://localhost:3000/message \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list",
    "params": {}
  }'
```

### Test Tool Call
```bash
curl -X POST http://localhost:3000/message \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "list_workspaces",
      "arguments": {}
    }
  }'
```

---

## Configuration

### Workspace Configuration
Edit `.config/workspaces.json`:
```json
[
  {
    "name": "providentia",
    "baseUrl": "https://your-domain.atlassian.net",
    "email": "your-email@example.com",
    "apiToken": "your-api-token"
  }
]
```

### Environment Variables
Edit `.env`:
```bash
WORKSPACES_FILE=../../.config/workspaces.json
RABBITMQ_HOST=localhost
RABBITMQ_VHOST=trilix
RABBITMQ_USER=trilix
RABBITMQ_PASSWORD=secret
```

---

## Troubleshooting

### Port Already in Use
If port 3000 is in use, the server will fail to start. Check the logs:
```bash
tail -f mcp-server.log
```

### Service Not Responding
Check if all services are running:
```bash
ps aux | grep "go run main.go"
```

Restart services:
```bash
./start-services.sh
```

### RabbitMQ Connection Issues
Ensure RabbitMQ is running:
```bash
docker ps | grep rabbitmq
```

Check RabbitMQ queues:
```
http://localhost:15672/#/queues
```
