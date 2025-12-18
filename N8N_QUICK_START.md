# n8n MCP Integration - Quick Start

## ✅ Server is Running

Your MCP server is running on: **http://localhost:3000**

## 🚀 Connect to n8n

### Using n8n MCP Node

1. **Open n8n** workflow editor

2. **Add MCP Tool Node**:
   - Search for "MCP" in the node panel
   - Add **"@n8n/n8n-nodes-langchain.agent"** or **"MCP Tool"** node

3. **Configure Connection**:
   ```
   Transport: stdio
   Command: /full/path/to/Trilix-Atlassian-MCP-Server/mcp-stdio
   ```
   
   **Example:**
   ```
   /Users/vaibhavdashora/Desktop/IdeaUsher/Trilix-Atlassian-MCP-Server/mcp-stdio
   ```

4. **Test Connection**:
   - Click "Test Connection" or "Fetch Tools"
   - You should see all available tools loaded

### Option 2: Using HTTP Request Node (Alternative)

If MCP node is not available, use HTTP Request:

1. **Add HTTP Request Node**

2. **Configure**:
   ```
   Method: POST
   URL: http://localhost:3000/message
   Content-Type: application/json
   ```

3. **Body** (to list tools):
   ```json
   {
     "jsonrpc": "2.0",
     "id": 1,
     "method": "tools/list",
     "params": {}
   }
   ```

4. **Body** (to call a tool):
   ```json
   {
     "jsonrpc": "2.0",
     "id": 2,
     "method": "tools/call",
     "params": {
       "name": "list_workspaces",
       "arguments": {}
     }
   }
   ```

## 📋 Available Tools

### Management
- ✅ `list_workspaces` - List all configured workspaces
- ✅ `workspace_status` - Check workspace connectivity

### Confluence (11 tools)
- ✅ `confluence_get_page` - Get page by ID
- ✅ `confluence_search` - Search using CQL
- ✅ `confluence_create_page` - Create new page
- ✅ `confluence_copy_page` - Copy between workspaces
- ✅ `confluence_list_spaces` - List all spaces

### Jira (7 tools)
- ✅ `jira_list_issues` - Search with JQL
- ✅ `jira_get_issue` - Get issue by key
- ✅ `jira_create_issue` - Create new issue
- ✅ `jira_update_issue` - Update issue
- ✅ `jira_add_comment` - Add comment
- ✅ `jira_transition_issue` - Change status

## 🧪 Test the Server

### Test 1: Check if server is running
```bash
curl http://localhost:3000/sse
```
Expected: `event: endpoint` message

### Test 2: Initialize MCP
```bash
curl -X POST http://localhost:3000/message \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
```

### Test 3: List all tools
```bash
curl -X POST http://localhost:3000/message \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
```

### Test 4: Call a tool
```bash
curl -X POST http://localhost:3000/message \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "id":3,
    "method":"tools/call",
    "params":{
      "name":"list_workspaces",
      "arguments":{}
    }
  }'
```

## ⚙️ Configuration

### Workspace Setup
Edit `.config/workspaces.json`:
```json
[
  {
    "name": "providentia",
    "baseUrl": "https://your-domain.atlassian.net",
    "email": "your-email@example.com",
    "apiToken": "your-atlassian-api-token"
  }
]
```

### Get Atlassian API Token
1. Go to: https://id.atlassian.com/manage-profile/security/api-tokens
2. Click "Create API token"
3. Copy the token to `workspaces.json`

## 🔧 Troubleshooting

### "Could not connect to MCP server"

**Check 1:** Is the server running?
```bash
ps aux | grep "mcp-server"
```

**Check 2:** Test the endpoint
```bash
curl http://localhost:3000/sse
```

**Check 3:** Check logs
```bash
tail -f mcp-server.log
```

**Fix:** Restart services
```bash
./start-services.sh
```

### Port 3000 already in use

**Check what's using it:**
```bash
lsof -i :3000
```

**Kill the process:**
```bash
lsof -ti:3000 | xargs kill -9
```

### Services not starting

**Check RabbitMQ:**
```bash
docker ps | grep rabbitmq
```

**Start RabbitMQ:**
```bash
docker-compose up -d
```

## 📊 Service Status

Check all services:
```bash
# Confluence Service
tail -5 confluence-service.log

# Jira Service  
tail -5 jira-service.log

# MCP Server
tail -5 mcp-server.log
```

All services should show: `Waiting for rpc messages` or `MCP SSE Server listening`

## 🎯 Example n8n Workflows

### Example 1: List Workspaces
```
Trigger → MCP Tool (list_workspaces) → Process Results
```

### Example 2: Search Confluence
```
Trigger → MCP Tool (confluence_search) → Filter Results → Send Email
```

### Example 3: Create Jira Issue
```
Webhook → MCP Tool (jira_create_issue) → Notify Slack
```

### Example 4: Sync Confluence to Jira
```
Schedule → MCP Tool (confluence_search) → 
Loop → MCP Tool (jira_create_issue) → Done
```

## 📚 More Information

- Full documentation: `N8N_INTEGRATION.md`
- MCP Protocol: https://modelcontextprotocol.io
- n8n Documentation: https://docs.n8n.io

## 🆘 Need Help?

1. Check logs: `tail -f mcp-server.log`
2. Test endpoints with curl (see tests above)
3. Verify RabbitMQ is running
4. Ensure `.config/workspaces.json` is configured
