# Cursor AI Agent Rules

> **File Location:** Place this file as `.cursorrules` in the project root, or as `CURSOR_RULES.md` in the `/docs` directory.

---

## 🎯 Project Context

You are building the **Trilix Atlassian MCP Server** - a microservices-based system that:

1. Implements the Model Context Protocol (MCP) for AI assistant integration
2. Connects to multiple Atlassian workspaces (Confluence, Jira) simultaneously
3. Uses TwistyGo library for RabbitMQ-based service communication
4. Deploys to Kubernetes in production

**Primary Documentation:** `docs/TRILIX-ATLASSIAN-MCP-SERVER-REQUIREMENTS.md`

---

## 🚨 Critical Rules

### Rule 1: Always Use TwistyGo for Service Communication

```go
// ✅ CORRECT - Use TwistyGo for inter-service communication
import "github.com/providentiaww/twistygo"

func init() {
    twistygo.LogStartService("ConfluenceService", "v1.0.0")
    rconn := twistygo.AmqpConnect()
    rconn.AmqpLoadQueues("ConfluenceRequests")
    rconn.AmqpLoadServices("ConfluenceService")
}

// ❌ WRONG - Do NOT use HTTP servers for service-to-service calls
http.ListenAndServe(":8080", handler)
```

### Rule 2: Always Include workspace_id

Every Atlassian operation must specify which workspace to use:

```go
// ✅ CORRECT
type ConfluenceRequest struct {
    Action      string         `json:"action"`
    WorkspaceID string         `json:"workspace_id"`  // REQUIRED!
    UserID      string         `json:"user_id"`
    Params      map[string]any `json:"params"`
    RequestID   string         `json:"request_id"`
}

// ❌ WRONG - Missing workspace context
type ConfluenceRequest struct {
    Action string         `json:"action"`
    Params map[string]any `json:"params"`
}
```

### Rule 3: Never Expose API Tokens

```go
// ✅ CORRECT
log.Info("Connecting to workspace", "workspace_id", workspaceID)
return fmt.Errorf("authentication failed for workspace %s", workspaceID)

// ❌ WRONG - Exposes sensitive data
log.Info("Using token", "token", apiToken)
return fmt.Errorf("failed with token %s", token)
```

### Rule 4: Always Create config.yaml AND settings.yaml

Every TwistyGo service requires both files:

**config.yaml** - Infrastructure:
```yaml
common:
  org: trilix
  loglevel: info
  consolelogging: true
  settingsfiles:
    - settings.yaml

rabbitmq:
  primary:
    default: true
    host: '@env:RABBITMQ_HOST'
    vhost: '@env:RABBITMQ_VHOST'
    port: 5672
    username: '@env:RABBITMQ_USER'
    password: '@env:RABBITMQ_PASSWORD'
```

**settings.yaml** - Queues and services:
```yaml
services:
  - name: ConfluenceService
    category: atlassian
    type: rpc
    queueref: ConfluenceRequests
    instances:
      dev: 1
      prd: 3

queues:
  - name: ConfluenceRequests
    category: atlassian
    type: rpc
    exchange:
      name: trilix.atlassian
    queue:
      name: confluence.requests
    routingkey: confluence.rpc
    protocol: json
```

---

## 📁 Project Structure

```
trilix-atlassian-mcp/
├── cmd/
│   ├── mcp-server/              # Northbound MCP interface
│   │   ├── main.go
│   │   ├── config.yaml
│   │   ├── settings.yaml
│   │   └── handlers/
│   │       ├── confluence.go
│   │       ├── jira.go
│   │       └── management.go
│   │
│   ├── confluence-service/      # Confluence API backend
│   │   ├── main.go
│   │   ├── config.yaml
│   │   ├── settings.yaml
│   │   └── api/
│   │       └── client.go
│   │
│   └── jira-service/            # Jira API backend
│       ├── main.go
│       ├── config.yaml
│       ├── settings.yaml
│       └── api/
│           └── client.go
│
├── internal/
│   ├── models/                  # Shared request/response types
│   ├── crypto/                  # Token encryption utilities
│   └── storage/                 # Credential database access
│
├── pkg/
│   └── mcp/                     # MCP protocol implementation
│
├── manifest.yaml                # K8s deployment via twistydeploy
├── docker-compose.yaml          # Local development
└── docs/
    ├── TRILIX-ATLASSIAN-MCP-SERVER-REQUIREMENTS.md
    └── CURSOR_RULES.md
```

---

## 🔧 Implementation Patterns

### TwistyGo Service Handler (Backend)

```go
package main

import (
    "encoding/json"
    "github.com/providentiaww/twistygo"
    amqp "github.com/rabbitmq/amqp091-go"
)

const ServiceVersion = "v1.0.0"

var rconn *twistygo.AmqpConnection_t

func init() {
    twistygo.LogStartService("ConfluenceService", ServiceVersion)
    rconn = twistygo.AmqpConnect()
    rconn.AmqpLoadQueues("ConfluenceRequests")
    rconn.AmqpLoadServices("ConfluenceService")
}

func main() {
    service := rconn.AmqpConnectService("ConfluenceService")
    service.StartService(handleRequest)
}

func handleRequest(d amqp.Delivery) []byte {
    var req ConfluenceRequest
    if err := json.Unmarshal(d.Body, &req); err != nil {
        return errorResponse("INVALID_REQUEST", err.Error())
    }
    
    switch req.Action {
    case "get_page":
        return handleGetPage(req)
    case "create_page":
        return handleCreatePage(req)
    case "copy_page":
        return handleCopyPage(req)
    default:
        return errorResponse("UNKNOWN_ACTION", req.Action)
    }
}
```

### Atlassian API Client

```go
package api

import (
    "encoding/base64"
    "encoding/json"
    "fmt"
    "net/http"
)

type WorkspaceCredentials struct {
    Site  string
    Email string
    Token string
}

type Client struct {
    creds      WorkspaceCredentials
    httpClient *http.Client
}

func NewClient(creds WorkspaceCredentials) *Client {
    return &Client{creds: creds, httpClient: &http.Client{}}
}

func (c *Client) authHeader() string {
    cred := fmt.Sprintf("%s:%s", c.creds.Email, c.creds.Token)
    return "Basic " + base64.StdEncoding.EncodeToString([]byte(cred))
}

func (c *Client) GetPage(pageID string) (*Page, error) {
    url := fmt.Sprintf("%s/rest/api/content/%s?expand=body.storage,version",
        c.creds.Site, pageID)
    
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("Authorization", c.authHeader())
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var page Page
    json.NewDecoder(resp.Body).Decode(&page)
    return &page, nil
}
```

### Cross-Workspace Operation

```go
func handleCopyPage(req ConfluenceRequest) []byte {
    srcWorkspace := req.Params["src_workspace"].(string)
    dstWorkspace := req.Params["dst_workspace"].(string)
    srcPageID := req.Params["src_page_id"].(string)
    dstSpaceKey := req.Params["dst_space_key"].(string)
    
    // Get credentials for BOTH workspaces
    srcCreds, _ := credStore.Get(req.UserID, srcWorkspace)
    dstCreds, _ := credStore.Get(req.UserID, dstWorkspace)
    
    srcClient := api.NewClient(srcCreds)
    dstClient := api.NewClient(dstCreds)
    
    // Read from source
    page, _ := srcClient.GetPage(srcPageID)
    
    // Create in destination
    newPage, _ := dstClient.CreatePage(dstSpaceKey, page.Title, page.Body)
    
    return successResponse(newPage)
}
```

### MCP Tool Registration

```go
var tools = []mcp.Tool{
    {
        Name:        "confluence_get_page",
        Description: "Retrieve a Confluence page by ID from a specific workspace",
        InputSchema: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "workspace_id": map[string]any{
                    "type":        "string",
                    "description": "Workspace ID (e.g., 'eso', 'providentia')",
                },
                "page_id": map[string]any{
                    "type":        "string",
                    "description": "Confluence page ID",
                },
            },
            "required": []string{"workspace_id", "page_id"},
        },
    },
    {
        Name:        "confluence_copy_page",
        Description: "Copy a page from one workspace to another",
        InputSchema: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "src_workspace":  map[string]any{"type": "string"},
                "dst_workspace":  map[string]any{"type": "string"},
                "src_page_id":    map[string]any{"type": "string"},
                "dst_space_key":  map[string]any{"type": "string"},
                "dst_parent_id":  map[string]any{"type": "string"},
            },
            "required": []string{"src_workspace", "dst_workspace", "src_page_id", "dst_space_key"},
        },
    },
}
```

---

## 🌐 API Reference

### Confluence Endpoints

| Operation | Method | URL |
|-----------|--------|-----|
| Get page | GET | `{site}/rest/api/content/{id}?expand=body.storage,version` |
| Create page | POST | `{site}/rest/api/content` |
| Get children | GET | `{site}/rest/api/content/{id}/child/page` |
| Search | GET | `{site}/rest/api/content/search?cql={query}` |

### Jira Endpoints

| Operation | Method | URL |
|-----------|--------|-----|
| Search issues | POST | `{site}/rest/api/3/search` |
| Get issue | GET | `{site}/rest/api/3/issue/{key}` |
| Create issue | POST | `{site}/rest/api/3/issue` |

---

## ⚠️ Common Mistakes

| ❌ Don't | ✅ Do |
|----------|-------|
| Use HTTP between services | Use RabbitMQ via TwistyGo |
| Hardcode workspace URLs | Load from credential storage |
| Store tokens in config files | Encrypt in database |
| Forget workspace_id | Include in every request |
| Log API tokens | Log only workspace_id |
| Skip settings.yaml | Create both config files |

---

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run specific service tests
go test ./cmd/confluence-service/...

# Run with coverage
go test -cover ./...
```

---

## 🚀 Deployment

```bash
# Local development
docker-compose up -d
go run ./cmd/confluence-service
go run ./cmd/jira-service
go run ./cmd/mcp-server

# Build for K8s
twistydeploy -m manifest.yaml -d production -g

# Deploy
twistydeploy -m manifest.yaml -d production
```

---

## ✅ MVP Completion Checklist

- [ ] MCP server responds to `tools/list`
- [ ] `confluence_get_page` works with workspace_id
- [ ] `confluence_create_page` creates pages
- [ ] `confluence_copy_page` copies between workspaces
- [ ] `jira_list_issues` returns JQL results
- [ ] `jira_get_issue` returns issue details
- [ ] Clerk authentication integrated
- [ ] Credential storage with encryption
- [ ] Docker-compose runs locally
- [ ] All services communicate via RabbitMQ
