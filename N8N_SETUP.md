# ✅ n8n MCP Integration - FINAL SETUP

## 🎯 Use This Executable Path in n8n

```
/Users/vaibhavdashora/Desktop/IdeaUsher/Trilix-Atlassian-MCP-Server/mcp-stdio
```

## 📋 n8n Configuration Steps

### 1. Open n8n Workflow Editor

### 2. Add MCP Node
- Search for "MCP" or "Model Context Protocol"
- Add the MCP Tool/Agent node

### 3. Configure the MCP Connection

**Transport Type:** `stdio`

**Command:** 
```
/Users/vaibhavdashora/Desktop/IdeaUsher/Trilix-Atlassian-MCP-Server/mcp-stdio
```

**Arguments:** (leave empty)

**Environment Variables:** (optional, leave empty if not needed)

### 4. Test Connection
- Click "Test Connection" or "Fetch Tools"
- You should see 14 tools loaded:
  - 2 Management tools
  - 5 Confluence tools  
  - 7 Jira tools

## ✅ Prerequisites (Must Be Running)

Before using n8n, ensure these services are running:

```bash
# Check services
ps aux | grep "go run main.go"

# Start services if not running
./start-services.sh

# Verify RabbitMQ
docker ps | grep rabbitmq
```

You should see:
- ✅ Confluence Service
- ✅ Jira Service
- ✅ RabbitMQ

## 🧪 Test the Executable

Test if the executable works:

```bash
# Navigate to project directory
cd /Users/vaibhavdashora/Desktop/IdeaUsher/Trilix-Atlassian-MCP-Server

# Test the executable (it will wait for input)
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./mcp-stdio
```

Expected output: JSON response with server info

## 📋 Available Tools in n8n

Once connected, you'll have access to:

### Management (2 tools)
- `list_workspaces` - List all configured Atlassian workspaces
- `workspace_status` - Check workspace connectivity status

### Confluence (5 tools)
- `confluence_get_page` - Retrieve a page by ID
- `confluence_search` - Search using CQL queries
- `confluence_create_page` - Create a new page
- `confluence_copy_page` - Copy page between workspaces
- `confluence_list_spaces` - List all spaces

### Jira (7 tools)
- `jira_list_issues` - Search issues using JQL
- `jira_get_issue` - Get issue details by key
- `jira_create_issue` - Create a new issue
- `jira_update_issue` - Update existing issue
- `jira_add_comment` - Add comment to issue
- `jira_transition_issue` - Change issue status

## 🔧 Troubleshooting

### "Could not connect to your MCP server"

**Solution 1:** Check if backend services are running
```bash
ps aux | grep "confluence-service\|jira-service"
```

If not running:
```bash
./start-services.sh
```

**Solution 2:** Check RabbitMQ
```bash
docker ps | grep rabbitmq
```

If not running:
```bash
docker-compose up -d
```

**Solution 3:** Test the executable manually
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | ./mcp-stdio
```

### "Permission denied"

Make executable:
```bash
chmod +x mcp-stdio
```

### "Workspace not found"

Configure workspaces in `.config/workspaces.json`:
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

## 📝 Example n8n Workflow

### Simple Test Workflow

1. **Manual Trigger** → 
2. **MCP Tool** (list_workspaces) → 
3. **Code Node** (process results)

### Confluence Search Workflow

1. **Schedule Trigger** (daily) →
2. **MCP Tool** (confluence_search) →
   - Tool: `confluence_search`
   - Arguments:
     ```json
     {
       "workspace_id": "providentia",
       "query": "type=page AND lastModified >= now('-7d')",
       "limit": 10
     }
     ```
3. **Process Results** →
4. **Send Email/Slack**

### Jira Issue Creation Workflow

1. **Webhook Trigger** →
2. **MCP Tool** (jira_create_issue) →
   - Tool: `jira_create_issue`
   - Arguments:
     ```json
     {
       "workspace_id": "providentia",
       "project_key": "PROJ",
       "issue_type": "Task",
       "summary": "{{$json.title}}",
       "description": "{{$json.description}}"
     }
     ```
3. **Return Response**

## 🚀 Quick Start Commands

```bash
# 1. Ensure you're in the project directory
cd /Users/vaibhavdashora/Desktop/IdeaUsher/Trilix-Atlassian-MCP-Server

# 2. Start backend services
./start-services.sh

# 3. Verify services are running
ps aux | grep "confluence-service\|jira-service"

# 4. Test the MCP executable
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./mcp-stdio

# 5. Open n8n and use this path:
# /Users/vaibhavdashora/Desktop/IdeaUsher/Trilix-Atlassian-MCP-Server/mcp-stdio
```

## 📚 Additional Resources

- MCP Protocol: https://modelcontextprotocol.io
- n8n Documentation: https://docs.n8n.io
- Atlassian API: https://developer.atlassian.com

## ✅ Checklist

Before using in n8n:
- [ ] RabbitMQ is running (`docker ps | grep rabbitmq`)
- [ ] Confluence service is running
- [ ] Jira service is running
- [ ] `.config/workspaces.json` is configured
- [ ] `mcp-stdio` executable exists and is executable
- [ ] Tested executable with echo command

## 🎉 You're Ready!

Use this path in n8n:
```
/Users/vaibhavdashora/Desktop/IdeaUsher/Trilix-Atlassian-MCP-Server/mcp-stdio
```

Transport: **stdio**
