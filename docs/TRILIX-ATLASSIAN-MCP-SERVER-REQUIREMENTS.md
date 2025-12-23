# Trilix Atlassian MCP Server - Requirements Document

## Document Overview

This document provides comprehensive requirements for building an MCP (Model Context Protocol) server that connects AI assistants (ChatGPT, Claude, etc.) to Atlassian APIs (Confluence, Jira) using the TwistyGo library. The solution enables users to interact with multiple Atlassian workspaces simultaneously through natural language.

**Target Audience:** AI coding agents (Cursor, Replit), developers implementing the solution

---

## 1. Business Context

### 1.1 Problem Statement

ESO's AI Platform requires a trusted, governed layer between AI agents and operational tools (Jira, Confluence, Slack). Direct MCP access to these tools presents several challenges:

- Raw, noisy data exposure (old tickets, untriaged threads, conflicting docs)
- No central normalization or data model
- PHI and sensitive information leakage risks
- Non-deterministic, inconsistent context for AI agents

### 1.2 Solution Overview

Build a Trilix-powered MCP server that:

- Acts as a trusted intermediary between AI agents and Atlassian APIs
- Supports multi-tenant access to multiple Atlassian organizations (Providentia, ESO)
- Provides normalized, validated, PHI-safe data exposure
- Uses RabbitMQ for service communication
- Deploys to Kubernetes in production

### 1.3 MVP Use Cases

Users interacting with ChatGPT should be able to:

1. **Cross-organization page copy:** Fetch a page from ESO Confluence and create a copy in Providentia Confluence
2. **Jira summarization:** Fetch Jira tasks from ESO, summarize them, and create a summary page in ESO Confluence

---

## 2. Architecture Overview

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     AI Agents (Northbound)                       │
│         Claude, ChatGPT, Replit, Cody, Copilot, etc.            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      MCP Server Layer                            │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                  Trilix MCP Server                       │    │
│  │  - MCP Protocol Handler (stdio/SSE)                      │    │
│  │  - Tool Registry & Dispatch                              │    │
│  │  - User Authentication (Clerk)                           │    │
│  │  - API Key Management                                    │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼ RabbitMQ
┌─────────────────────────────────────────────────────────────────┐
│                    Backend Services Layer                        │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────┐   │
│  │ Confluence       │  │ Jira             │  │ Future       │   │
│  │ Service          │  │ Service          │  │ Services     │   │
│  │ - Read pages     │  │ - List issues    │  │ - Slack      │   │
│  │ - Create pages   │  │ - Get details    │  │ - Git        │   │
│  │ - Search         │  │ - Create issues  │  │ - Email      │   │
│  └──────────────────┘  └──────────────────┘  └──────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Atlassian APIs (Southbound)                     │
│         Providentia Workspace    │    ESO Workspace              │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| MCP Server | Protocol handling, tool dispatch, auth, API key management |
| Confluence Service | All Confluence API interactions via RabbitMQ |
| Jira Service | All Jira API interactions via RabbitMQ |
| RabbitMQ | Service-to-service communication, request/response routing |
| Clerk | User authentication and session management |

---

## 3. Technical Requirements

### 3.1 TwistyGo Library Usage

The solution **MUST** use the TwistyGo library (`github.com/providentiaww/twistygo`). Key patterns to follow:

#### 3.1.1 Service Initialization Pattern

```go
package main

import (
    "github.com/joho/godotenv"
    "github.com/providentiaww/twistygo"
)

const ServiceVersion = "v0.1.0"

var (
    confluenceRequestQ  *twistygo.ServiceQueue_t
    confluenceResponseQ *twistygo.ServiceQueue_t
)

func init() {
    godotenv.Load()
    
    // Initialize TwistyGo with service name
    twistygo.LogStartService("ConfluenceService", ServiceVersion)
    
    // Connect to RabbitMQ (uses config.yaml)
    rconn := twistygo.AmqpConnect()
    
    // Load queue definitions from settings.yaml
    rconn.AmqpLoadQueues(
        "ConfluenceRequests",
        "ConfluenceResponses",
    )
    
    // Load service definitions
    rconn.AmqpLoadServices("ConfluenceService")
    
    // Get queue handles
    confluenceRequestQ = rconn.AmqpConnectQueue("ConfluenceRequests")
    confluenceRequestQ.SetEncoding(twistygo.EncodingJson)
}
```

#### 3.1.2 Configuration Files Required

Each service requires two configuration files:

**config.yaml** - Infrastructure configuration:
```yaml
common:
  org: trilix
  loglevel: info
  amqplogging: false
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

**settings.yaml** - Queue and service definitions:
```yaml
services:
  - name: ConfluenceService
    category: atlassian
    type: rpc
    queueref: ConfluenceRequests
    instances:
      dev: 1
      tst: 2
      prd: 3

queues:
  - name: ConfluenceRequests
    category: atlassian
    type: rpc
    exchange:
      name: trilix.atlassian
    queue:
      name: confluence.requests
      autoack: false
    routingkey: confluence.rpc
    protocol: json
```

#### 3.1.3 Message Types (AmqType)

TwistyGo supports the following message patterns:

| Type | Use Case | Description |
|------|----------|-------------|
| `TOPIC` | Fire-and-forget | Publish to exchange, routed by key |
| `RPC` | Request-response | Synchronous call with response |
| `FANOUT` | Broadcast | Send to all bound queues |
| `DIRECT` | Point-to-point | Direct queue routing |

**For MCP Server:** Use `RPC` type for all Atlassian API calls (request-response pattern)

#### 3.1.4 Encoding Options

```go
// Available encodings
twistygo.EncodingJson   // application/json - RECOMMENDED
twistygo.EncodingAvro   // binary/AVRO - for high-throughput
twistygo.EncodingText   // text/plain
twistygo.EncodingBinary // application/binary
```

**Recommendation:** Use `EncodingJson` for all MCP-related messages for debuggability.

#### 3.1.5 RPC Service Handler Pattern

```go
func main() {
    // Get service handle
    service := rconn.AmqpConnectService("ConfluenceService")
    
    // Start listening with handler function
    service.StartService(handleConfluenceRequest)
}

func handleConfluenceRequest(d amqp.Delivery) []byte {
    // Unmarshal request
    var request ConfluenceRequest
    json.Unmarshal(d.Body, &request)
    
    // Process request
    response := processConfluenceRequest(request)
    
    // Return response (auto-published to ReplyTo queue)
    responseBytes, _ := json.Marshal(response)
    return responseBytes
}
```

#### 3.1.6 Publishing RPC Requests

```go
func callConfluenceService(request ConfluenceRequest) (*ConfluenceResponse, error) {
    // Prepare message
    sq := rconn.AmqpConnectQueue("ConfluenceRequests")
    sq.SetEncoding(twistygo.EncodingJson)
    
    // Append data to message
    sq.Message.AppendData(request)
    
    // Publish and wait for response (RPC)
    responseBytes, err := sq.Publish()
    if err != nil {
        return nil, err
    }
    
    // Unmarshal response
    var response ConfluenceResponse
    json.Unmarshal(responseBytes, &response)
    
    return &response, nil
}
```

### 3.2 MCP Protocol Implementation

#### 3.2.1 MCP Specification

Implement the Model Context Protocol as defined at: https://modelcontextprotocol.io/docs/getting-started/intro

Key MCP concepts:
- **Tools**: Functions the AI can call (e.g., `confluence_get_page`, `jira_list_issues`)
- **Resources**: Data the AI can read (e.g., page content, issue details)
- **Prompts**: Pre-defined prompt templates

#### 3.2.2 Required MCP Tools

##### Confluence Tools

| Tool Name | Description | Parameters |
|-----------|-------------|------------|
| `confluence_get_page` | Retrieve a page by ID | `workspace_id`, `page_id` |
| `confluence_search` | Search pages by query | `workspace_id`, `query`, `space_key?`, `limit?` |
| `confluence_create_page` | Create a new page | `workspace_id`, `space_key`, `title`, `body`, `parent_id?` |
| `confluence_update_page` | Update existing page | `workspace_id`, `page_id`, `title?`, `body?` |
| `confluence_list_spaces` | List available spaces | `workspace_id`, `limit?` |

##### Jira Tools

| Tool Name | Description | Parameters |
|-----------|-------------|------------|
| `jira_list_issues` | List issues with JQL | `workspace_id`, `jql`, `fields?`, `limit?` |
| `jira_get_issue` | Get issue by key | `workspace_id`, `issue_key` |
| `jira_create_issue` | Create new issue | `workspace_id`, `project_key`, `issue_type`, `summary`, `description?` |
| `jira_update_issue` | Update issue fields | `workspace_id`, `issue_key`, `fields` |
| `jira_add_comment` | Add comment to issue | `workspace_id`, `issue_key`, `body` |

##### Management Tools

| Tool Name | Description | Parameters |
|-----------|-------------|------------|
| `list_workspaces` | List configured workspaces | (none) |
| `workspace_status` | Check workspace connectivity | `workspace_id` |

#### 3.2.3 Workspace ID Convention

Since users connect to multiple Atlassian organizations, each tool requires a `workspace_id` parameter:

```json
{
  "tool": "confluence_get_page",
  "arguments": {
    "workspace_id": "providentia",
    "page_id": "123456789"
  }
}
```

Workspace IDs are user-defined labels mapped to stored API credentials.

### 3.3 Authentication & API Key Management

#### 3.3.1 User Authentication

- Use **Clerk** (https://clerk.dev) for user authentication
- Users authenticate via Clerk before accessing the MCP server
- Session tokens validate user identity for API key access

#### 3.3.2 Atlassian API Key Storage

Users store their Atlassian API keys in the system:

```go
type AtlassianCredential struct {
    UserID        string    `json:"user_id"`        // Clerk user ID
    WorkspaceID   string    `json:"workspace_id"`   // User-defined label
    WorkspaceName string    `json:"workspace_name"` // Display name
    AtlassianURL  string    `json:"atlassian_url"`  // e.g., "https://providentia.atlassian.net"
    Email         string    `json:"email"`          // Atlassian account email
    APIToken      string    `json:"api_token"`      // Encrypted Atlassian API token
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}
```

#### 3.3.3 Security Requirements

- API tokens **MUST** be encrypted at rest (AES-256 or similar)
- API tokens **MUST NOT** be logged or exposed in error messages
- Implement rate limiting per user/workspace
- Audit log all API key access

### 3.4 Atlassian API Integration

#### 3.4.1 API Documentation References

- **Confluence REST API:** https://developer.atlassian.com/cloud/confluence/rest/v2/intro/
- **Jira REST API:** https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/
- **Authentication:** https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/

#### 3.4.2 API Authentication Pattern

```go
// Basic Auth with API Token
func createAtlassianClient(cred AtlassianCredential) *http.Client {
    // Create authenticated HTTP client
    // Authorization: Basic base64(email:api_token)
}
```

#### 3.4.3 Required API Endpoints

##### Confluence Endpoints

| Operation | Method | Endpoint |
|-----------|--------|----------|
| Get page | GET | `/wiki/api/v2/pages/{id}` |
| Search | GET | `/wiki/rest/api/content/search` |
| Create page | POST | `/wiki/api/v2/pages` |
| Update page | PUT | `/wiki/api/v2/pages/{id}` |
| List spaces | GET | `/wiki/api/v2/spaces` |

##### Jira Endpoints

| Operation | Method | Endpoint |
|-----------|--------|----------|
| Search issues | POST | `/rest/api/3/search` |
| Get issue | GET | `/rest/api/3/issue/{issueIdOrKey}` |
| Create issue | POST | `/rest/api/3/issue` |
| Update issue | PUT | `/rest/api/3/issue/{issueIdOrKey}` |
| Add comment | POST | `/rest/api/3/issue/{issueIdOrKey}/comment` |

### 3.5 Deployment Requirements

#### 3.5.1 Kubernetes Deployment

Use TwistyGo's `twistydeploy` tool with a `manifest.yaml`:

```yaml
services:
  mcp-server:
    exec: mcp-server
    source: ./cmd/mcp-server
    configpath: ./cmd/mcp-server/config/$deployment
    instances:
      dev: 1
      tst: 2
      prd: 3

  confluence-service:
    exec: confluence-service
    source: ./cmd/confluence-service
    configpath: ./cmd/confluence-service/config/$deployment
    instances:
      dev: 1
      tst: 2
      prd: 3

  jira-service:
    exec: jira-service
    source: ./cmd/jira-service
    configpath: ./cmd/jira-service/config/$deployment
    instances:
      dev: 1
      tst: 2
      prd: 3

deployment:
  production:
    namespace: trilix-mcp
    buildcmd: docker
    containerrepo: your-registry.azurecr.io/trilix
    imagerelease: latest
    configgroup: trilix-mcp-config
    instances: prd
    goversion: golang:1.22
    gobuildenv:
      - CGO_ENABLED=0
      - GOOS=linux
      - GOARCH=amd64
    resources:
      limits:
        cpu: "500m"
        memory: "512Mi"
      requests:
        cpu: "250m"
        memory: "256Mi"
```

#### 3.5.2 Required Infrastructure

| Component | Purpose | Notes |
|-----------|---------|-------|
| RabbitMQ | Service messaging | Quorum queues for durability |
| PostgreSQL | User credentials, audit logs | Encrypted storage |
| Redis | Session cache, rate limiting | Optional |
| Kubernetes | Container orchestration | Production deployment |

---

## 4. Project Structure

```
trilix-atlassian-mcp/
├── cmd/
│   ├── mcp-server/           # MCP protocol server
│   │   ├── main.go
│   │   ├── config.yaml
│   │   ├── settings.yaml
│   │   ├── handlers/         # MCP tool handlers
│   │   │   ├── confluence.go
│   │   │   ├── jira.go
│   │   │   └── management.go
│   │   └── auth/             # Clerk integration
│   │       └── clerk.go
│   │
│   ├── confluence-service/   # Confluence API service
│   │   ├── main.go
│   │   ├── config.yaml
│   │   ├── settings.yaml
│   │   ├── api/              # Atlassian API client
│   │   │   └── confluence.go
│   │   └── handlers/
│   │       └── handlers.go
│   │
│   └── jira-service/         # Jira API service
│       ├── main.go
│       ├── config.yaml
│       ├── settings.yaml
│       ├── api/
│       │   └── jira.go
│       └── handlers/
│           └── handlers.go
│
├── internal/
│   ├── models/               # Shared data models
│   │   ├── confluence.go
│   │   ├── jira.go
│   │   └── workspace.go
│   ├── crypto/               # API key encryption
│   │   └── encryption.go
│   └── storage/              # Credential storage
│       └── postgres.go
│
├── pkg/
│   └── mcp/                  # MCP protocol implementation
│       ├── server.go
│       ├── tools.go
│       └── types.go
│
├── manifest.yaml             # TwistyDeploy manifest
├── docker-compose.yaml       # Local development
├── go.mod
├── go.sum
└── docs/
    └── REQUIREMENTS.md       # This document
```

---

## 5. Data Models

### 5.1 Request/Response Models

#### Confluence Request

```go
type ConfluenceRequest struct {
    Action      string            `json:"action"`       // get_page, search, create_page, etc.
    WorkspaceID string            `json:"workspace_id"` // User's workspace label
    UserID      string            `json:"user_id"`      // Clerk user ID
    Params      map[string]any    `json:"params"`       // Action-specific parameters
    RequestID   string            `json:"request_id"`   // Correlation ID
}

type ConfluenceResponse struct {
    Success   bool              `json:"success"`
    Data      any               `json:"data,omitempty"`
    Error     *ErrorInfo        `json:"error,omitempty"`
    RequestID string            `json:"request_id"`
}
```

#### Jira Request

```go
type JiraRequest struct {
    Action      string            `json:"action"`       // list_issues, get_issue, create_issue, etc.
    WorkspaceID string            `json:"workspace_id"`
    UserID      string            `json:"user_id"`
    Params      map[string]any    `json:"params"`
    RequestID   string            `json:"request_id"`
}

type JiraResponse struct {
    Success   bool              `json:"success"`
    Data      any               `json:"data,omitempty"`
    Error     *ErrorInfo        `json:"error,omitempty"`
    RequestID string            `json:"request_id"`
}
```

### 5.2 Error Handling

```go
type ErrorInfo struct {
    Code    string `json:"code"`    // e.g., "AUTH_FAILED", "NOT_FOUND", "RATE_LIMITED"
    Message string `json:"message"` // Human-readable message
    Details any    `json:"details,omitempty"` // Additional context
}

// Standard error codes
const (
    ErrCodeAuthFailed     = "AUTH_FAILED"
    ErrCodeNotFound       = "NOT_FOUND"
    ErrCodeRateLimited    = "RATE_LIMITED"
    ErrCodeInvalidRequest = "INVALID_REQUEST"
    ErrCodeAPIError       = "API_ERROR"
    ErrCodeInternal       = "INTERNAL_ERROR"
)
```

---

## 6. Implementation Phases

### Phase 1: Foundation (MVP)

1. **MCP Server skeleton** with TwistyGo integration
2. **Confluence Service** with basic operations (get, create, search)
3. **Jira Service** with basic operations (list, get, create)
4. **Clerk authentication** integration
5. **API key storage** (PostgreSQL with encryption)
6. **Local development** environment (docker-compose)

### Phase 2: Production Readiness

1. **Kubernetes deployment** manifests
2. **Rate limiting** and abuse prevention
3. **Audit logging** for compliance
4. **Monitoring and alerting** (Prometheus metrics)
5. **Error handling** improvements
6. **Performance optimization**

### Phase 3: Expansion

1. **Additional Atlassian APIs** (Bitbucket, etc.)
2. **Slack integration** service
3. **Git integration** service
4. **Caching layer** (Redis)
5. **Versioned snapshots** for reproducible context

---

## 7. Testing Requirements

### 7.1 Unit Tests

- All Atlassian API client functions
- Request/response serialization
- Encryption/decryption utilities
- MCP tool handlers

### 7.2 Integration Tests

- RabbitMQ message flow (send request, receive response)
- Atlassian API connectivity (with test workspace)
- Clerk authentication flow

### 7.3 End-to-End Tests

- Complete MCP tool invocation flow
- Cross-workspace operations (MVP use case 1)
- Jira summarization flow (MVP use case 2)

---

## 8. Environment Variables

```bash
# RabbitMQ
RABBITMQ_HOST=localhost
RABBITMQ_VHOST=trilix
RABBITMQ_USER=trilix
RABBITMQ_PASSWORD=secret

# PostgreSQL
DATABASE_URL=postgres://user:pass@localhost:5432/trilix_mcp

# Clerk
CLERK_SECRET_KEY=sk_test_xxxxx
CLERK_PUBLISHABLE_KEY=pk_test_xxxxx

# Encryption
API_KEY_ENCRYPTION_KEY=32-byte-key-for-aes-256-encryption

# Service Configuration
LOG_LEVEL=info
ENVIRONMENT=development
```

---

## 9. Reference Implementation Examples

### 9.1 Example: Confluence API Client (Multi-Workspace)

A complete reference implementation for multi-workspace Confluence access is documented in **Appendix A**. This includes:

- Multi-workspace credential configuration pattern
- Core API functions (get page, create page, get children, download attachments)
- Cross-workspace page copy algorithm (MVP Use Case 1)
- Go implementation patterns translated from production Python code

**Source:** https://github.com/providentiaww/eso-tools/tree/master/confluence

Key patterns demonstrated:
- Separate credential sets per workspace (site + email + token)
- Dynamic client creation based on `workspace_id` in requests
- Cross-workspace operations using two authenticated clients simultaneously

### 9.2 Example: TwistyGo Service

See the `stockproducer` example in the TwistyGo library for patterns on:
- Service initialization
- Queue configuration
- Message publishing

See **Appendix C** for a quick reference of TwistyGo patterns.

---

## 10. Acceptance Criteria

### MVP Completion Checklist

- [ ] MCP server responds to `tools/list` request with available tools
- [ ] User can authenticate via Clerk and store Atlassian API keys
- [ ] `confluence_get_page` retrieves page content from specified workspace
- [ ] `confluence_create_page` creates new page in specified workspace
- [ ] `jira_list_issues` returns issues matching JQL query
- [ ] Cross-workspace page copy works (ESO → Providentia)
- [ ] Jira summarization and Confluence page creation works
- [ ] Services communicate via RabbitMQ using TwistyGo patterns
- [ ] Local development environment works with docker-compose
- [ ] Basic error handling and logging implemented

---

## 11. Glossary

| Term | Definition |
|------|------------|
| MCP | Model Context Protocol - standard for AI-tool communication |
| TwistyGo | Providentia's Go library for message-based microservices |
| Workspace | User-defined label for an Atlassian organization connection |
| Trilix | Providentia's middleware platform for enterprise integrations |
| RPC | Remote Procedure Call - request/response message pattern |

---

## Appendix A: Reference Implementation - Multi-Workspace Confluence API

This appendix provides a **proven reference implementation** for accessing multiple Atlassian instances simultaneously. The patterns shown here are extracted from production code at `github.com/providentiaww/eso-tools/confluence`.

### A.1 Multi-Workspace Configuration Pattern

The key to supporting multiple Atlassian organizations is maintaining separate credential sets for each workspace. Each API call must specify which workspace (site + email + token) to use.

#### Environment Configuration (.env)

```bash
# Source Workspace (e.g., ESO)
SRC_SITE=https://eso.atlassian.net/wiki
SRC_EMAIL=service-account@eso.com
SRC_CONF_TOKEN=ATATT3xFfGF0...  # Atlassian API token

# Destination Workspace (e.g., Providentia)
DST_SITE=https://providentiaworldwide.atlassian.net/wiki
DST_EMAIL=service-account@providentia.com
DST_CONF_TOKEN=ATATT3xFfGF0...  # Atlassian API token

# Operation Parameters
SRC_PAGE_ID=123456789
DST_SPACE=DOCS
DST_PARENT_ID=987654321
```

### A.2 Core Confluence API Functions (Reference Python)

These functions demonstrate the exact API patterns to implement in Go:

#### Get Page with Content

```python
def get_page(site, email, token, page_id):
    """Fetch page metadata + storage body."""
    url = f"{site}/rest/api/content/{page_id}"
    params = {"expand": "body.storage,version"}
    resp = requests.get(url, auth=(email, token), params=params)

    if not resp.ok:
        raise RuntimeError(f"Unable to fetch page {page_id}: {resp.text}")

    return resp.json()
```

**Go equivalent signature:**
```go
func GetPage(site, email, token, pageID string) (*ConfluencePage, error)
```

**Key points:**
- URL format: `{site}/rest/api/content/{page_id}`
- Use `expand=body.storage,version` to get page content
- Auth: Basic authentication with `(email, api_token)`

#### Get Child Pages

```python
def get_children(site, email, token, page_id):
    """Return all direct child pages of a parent page."""
    url = f"{site}/rest/api/content/{page_id}/child/page"
    params = {"expand": "version"}
    resp = requests.get(url, auth=(email, token), params=params)

    if not resp.ok:
        raise RuntimeError(f"Unable to fetch children of {page_id}: {resp.text}")

    children = resp.json().get("results", [])
    return children
```

**Go equivalent signature:**
```go
func GetChildren(site, email, token, pageID string) ([]ConfluencePage, error)
```

#### Create Page

```python
def create_page(site, email, token, space_key, title, body, parent_id=None):
    """Create a new page with a storage-format body."""
    url = f"{site}/rest/api/content"
    headers = {"Content-Type": "application/json"}

    payload = {
        "type": "page",
        "title": title,
        "space": {"key": space_key},
        "body": {
            "storage": {
                "value": body,
                "representation": "storage",
            }
        }
    }

    if parent_id:
        payload["ancestors"] = [{"id": str(parent_id)}]

    resp = requests.post(
        url,
        auth=(email, token),
        headers=headers,
        data=json.dumps(payload),
    )

    if not resp.ok:
        raise RuntimeError(f"Unable to create page '{title}': {resp.text}")

    return resp.json()
```

**Go equivalent signature:**
```go
func CreatePage(site, email, token, spaceKey, title, body string, parentID *string) (*ConfluencePage, error)
```

**Key points:**
- URL: `{site}/rest/api/content`
- Method: POST
- Content-Type: `application/json`
- Body format: Storage representation
- Parent specified via `ancestors` array

#### Download Attachments

```python
def download_attachments(site, email, token, page_id, page_title):
    url = f"{site}/rest/api/content/{page_id}/child/attachment"
    resp = requests.get(url, auth=(email, token))
    
    if not resp.ok:
        print(f"Failed to list attachments for {page_id}: {resp.text}")
        return
        
    results = resp.json().get("results", [])
    
    for att in results:
        filename = att["title"]
        download_link = att["_links"]["download"]
        file_url = f"{site}{download_link}"
        
        file_resp = requests.get(file_url, auth=(email, token), stream=True)
        # Save to file...
```

**Go equivalent signature:**
```go
func DownloadAttachments(site, email, token, pageID string) ([]Attachment, error)
```

### A.3 Cross-Workspace Copy Pattern (MVP Use Case 1)

This is the **core algorithm** for the MVP use case of copying content between organizations:

```python
def copy_page_recursive(
    src_site, src_email, src_token,      # Source workspace credentials
    dst_site, dst_email, dst_token,      # Destination workspace credentials
    src_page_id, dst_space, dst_parent_id=None,
    indent=0
):
    # 1. Read from SOURCE workspace
    page = get_page(src_site, src_email, src_token, src_page_id)
    title = page["title"]
    body = page["body"]["storage"]["value"]

    # 2. Download attachments from SOURCE
    download_attachments(src_site, src_email, src_token, src_page_id, title)

    # 3. Create in DESTINATION workspace
    created = create_page(
        dst_site,
        dst_email,
        dst_token,
        dst_space,
        title,
        body,
        parent_id=dst_parent_id,
    )
    new_page_id = created["id"]

    # 4. Recursively copy all children
    children = get_children(src_site, src_email, src_token, src_page_id)
    for child in children:
        child_id = child["id"]
        copy_page_recursive(
            src_site, src_email, src_token,
            dst_site, dst_email, dst_token,
            child_id, dst_space,
            dst_parent_id=new_page_id,
            indent=indent + 4,
        )

    return new_page_id
```

### A.4 Go Implementation Pattern

Translate the reference Python to Go for the Confluence service:

```go
package confluence

import (
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

// WorkspaceCredentials holds connection info for one Atlassian instance
type WorkspaceCredentials struct {
    Site   string // e.g., "https://eso.atlassian.net/wiki"
    Email  string // e.g., "service@eso.com"
    Token  string // Atlassian API token
}

// Client wraps HTTP client with Atlassian auth
type Client struct {
    creds      WorkspaceCredentials
    httpClient *http.Client
}

// NewClient creates an authenticated Confluence client
func NewClient(creds WorkspaceCredentials) *Client {
    return &Client{
        creds:      creds,
        httpClient: &http.Client{},
    }
}

// authHeader returns the Basic auth header value
func (c *Client) authHeader() string {
    credentials := fmt.Sprintf("%s:%s", c.creds.Email, c.creds.Token)
    encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
    return "Basic " + encoded
}

// GetPage fetches a page by ID with body content
func (c *Client) GetPage(pageID string) (*Page, error) {
    url := fmt.Sprintf("%s/rest/api/content/%s?expand=body.storage,version", 
        c.creds.Site, pageID)
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", c.authHeader())
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to get page %s: %s", pageID, string(body))
    }
    
    var page Page
    if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
        return nil, err
    }
    
    return &page, nil
}

// CreatePage creates a new page in the specified space
func (c *Client) CreatePage(spaceKey, title, body string, parentID *string) (*Page, error) {
    url := fmt.Sprintf("%s/rest/api/content", c.creds.Site)
    
    payload := CreatePageRequest{
        Type:  "page",
        Title: title,
        Space: SpaceRef{Key: spaceKey},
        Body: BodyContent{
            Storage: StorageContent{
                Value:          body,
                Representation: "storage",
            },
        },
    }
    
    if parentID != nil {
        payload.Ancestors = []AncestorRef{{ID: *parentID}}
    }
    
    jsonPayload, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }
    
    req, err := http.NewRequest("POST", url, bytes.NewReader(jsonPayload))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", c.authHeader())
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to create page '%s': %s", title, string(body))
    }
    
    var page Page
    if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
        return nil, err
    }
    
    return &page, nil
}

// Data structures
type Page struct {
    ID      string      `json:"id"`
    Title   string      `json:"title"`
    Version VersionInfo `json:"version"`
    Body    struct {
        Storage StorageContent `json:"storage"`
    } `json:"body"`
}

type CreatePageRequest struct {
    Type      string        `json:"type"`
    Title     string        `json:"title"`
    Space     SpaceRef      `json:"space"`
    Body      BodyContent   `json:"body"`
    Ancestors []AncestorRef `json:"ancestors,omitempty"`
}

type SpaceRef struct {
    Key string `json:"key"`
}

type BodyContent struct {
    Storage StorageContent `json:"storage"`
}

type StorageContent struct {
    Value          string `json:"value"`
    Representation string `json:"representation"`
}

type AncestorRef struct {
    ID string `json:"id"`
}

type VersionInfo struct {
    Number int `json:"number"`
}
```

### A.5 Multi-Workspace Service Pattern

The Confluence service must handle requests that specify which workspace to use:

```go
// ConfluenceRequest includes workspace identifier
type ConfluenceRequest struct {
    Action      string         `json:"action"`
    WorkspaceID string         `json:"workspace_id"`  // e.g., "eso" or "providentia"
    UserID      string         `json:"user_id"`
    Params      map[string]any `json:"params"`
    RequestID   string         `json:"request_id"`
}

// Service holds clients for multiple workspaces
type Service struct {
    credStore CredentialStore  // Retrieves workspace credentials by ID
}

// HandleRequest routes to the correct workspace
func (s *Service) HandleRequest(req ConfluenceRequest) (*ConfluenceResponse, error) {
    // 1. Get credentials for the specified workspace
    creds, err := s.credStore.GetCredentials(req.UserID, req.WorkspaceID)
    if err != nil {
        return nil, fmt.Errorf("workspace not found: %s", req.WorkspaceID)
    }
    
    // 2. Create client for this workspace
    client := NewClient(creds)
    
    // 3. Execute the requested action
    switch req.Action {
    case "get_page":
        pageID := req.Params["page_id"].(string)
        return s.getPage(client, pageID, req.RequestID)
    case "create_page":
        return s.createPage(client, req.Params, req.RequestID)
    case "copy_page":
        return s.copyPage(req)  // Special: involves TWO workspaces
    default:
        return nil, fmt.Errorf("unknown action: %s", req.Action)
    }
}

// copyPage handles cross-workspace operations
func (s *Service) copyPage(req ConfluenceRequest) (*ConfluenceResponse, error) {
    srcWorkspace := req.Params["src_workspace"].(string)
    dstWorkspace := req.Params["dst_workspace"].(string)
    srcPageID := req.Params["src_page_id"].(string)
    dstSpaceKey := req.Params["dst_space_key"].(string)
    dstParentID := req.Params["dst_parent_id"].(string)  // optional
    
    // Get credentials for BOTH workspaces
    srcCreds, _ := s.credStore.GetCredentials(req.UserID, srcWorkspace)
    dstCreds, _ := s.credStore.GetCredentials(req.UserID, dstWorkspace)
    
    srcClient := NewClient(srcCreds)
    dstClient := NewClient(dstCreds)
    
    // Read from source
    page, err := srcClient.GetPage(srcPageID)
    if err != nil {
        return nil, err
    }
    
    // Create in destination
    var parentPtr *string
    if dstParentID != "" {
        parentPtr = &dstParentID
    }
    
    newPage, err := dstClient.CreatePage(
        dstSpaceKey,
        page.Title,
        page.Body.Storage.Value,
        parentPtr,
    )
    if err != nil {
        return nil, err
    }
    
    return &ConfluenceResponse{
        Success:   true,
        Data:      newPage,
        RequestID: req.RequestID,
    }, nil
}
```

### A.6 MCP Tool Registration for Cross-Workspace Operations

```json
{
  "name": "confluence_copy_page",
  "description": "Copy a Confluence page from one workspace to another, including all content",
  "inputSchema": {
    "type": "object",
    "properties": {
      "src_workspace": {
        "type": "string",
        "description": "Source workspace ID (e.g., 'eso')"
      },
      "dst_workspace": {
        "type": "string",
        "description": "Destination workspace ID (e.g., 'providentia')"
      },
      "src_page_id": {
        "type": "string",
        "description": "Page ID to copy from source"
      },
      "dst_space_key": {
        "type": "string",
        "description": "Destination space key"
      },
      "dst_parent_id": {
        "type": "string",
        "description": "Optional parent page ID in destination"
      },
      "recursive": {
        "type": "boolean",
        "description": "If true, copy all child pages recursively",
        "default": false
      }
    },
    "required": ["src_workspace", "dst_workspace", "src_page_id", "dst_space_key"]
  }
}
```

---

## Appendix B: Atlassian API Authentication

### Creating an API Token

1. Log in to https://id.atlassian.com/manage-profile/security/api-tokens
2. Click "Create API token"
3. Give it a descriptive label
4. Copy and securely store the token

### Using the Token

```bash
# Base64 encode email:token
echo -n "your-email@example.com:your-api-token" | base64

# Use in Authorization header
curl -H "Authorization: Basic <base64-encoded-credentials>" \
     https://your-instance.atlassian.net/wiki/api/v2/pages/123
```

---

## Appendix C: TwistyGo Quick Reference

### Starting a Service

```go
twistygo.LogStartService("ServiceName", "v1.0.0")
rconn := twistygo.AmqpConnect()
rconn.AmqpLoadQueues("QueueName")
rconn.AmqpLoadServices("ServiceName")
```

### Publishing a Message

```go
sq := rconn.AmqpConnectQueue("QueueName")
sq.SetEncoding(twistygo.EncodingJson)
sq.Message.AppendData(myData)
response, err := sq.Publish()
```

### Handling Requests

```go
service := rconn.AmqpConnectService("ServiceName")
service.StartService(func(d amqp.Delivery) []byte {
    // Process d.Body
    return responseBytes
})
```

---

## Appendix D: Environment Variables Reference

### Development Environment (.env)

```bash
# ============================================
# RabbitMQ Configuration
# ============================================
RABBITMQ_HOST=localhost
RABBITMQ_VHOST=trilix
RABBITMQ_USER=trilix
RABBITMQ_PASSWORD=secret

# ============================================
# PostgreSQL (Credential Storage)
# ============================================
DATABASE_URL=postgres://user:pass@localhost:5432/trilix_mcp

# ============================================
# Clerk Authentication
# ============================================
CLERK_SECRET_KEY=sk_test_xxxxx
CLERK_PUBLISHABLE_KEY=pk_test_xxxxx

# ============================================
# Security
# ============================================
API_KEY_ENCRYPTION_KEY=32-byte-key-for-aes-256-encryption

# ============================================
# Service Configuration
# ============================================
LOG_LEVEL=info
ENVIRONMENT=development

# ============================================
# Atlassian Workspaces (for testing/dev only)
# In production, these are stored per-user in the database
# ============================================

# ESO Workspace
ESO_SITE=https://eso.atlassian.net/wiki
ESO_EMAIL=service@eso.com
ESO_TOKEN=ATATT3xFfGF0...

# Providentia Workspace
PROVIDENTIA_SITE=https://providentiaworldwide.atlassian.net/wiki
PROVIDENTIA_EMAIL=service@providentia.com
PROVIDENTIA_TOKEN=ATATT3xFfGF0...
```

---

*Document Version: 1.1*
*Last Updated: November 2025*
*Authors: Providentia Worldwide / Trilix Team*
*Reference Implementation: github.com/providentiaww/eso-tools/confluence*
