# MCP Server Architecture - Requirements Document

## Document Purpose

This document defines **WHAT** to build for the Trilix Atlassian MCP Server. It describes the business requirements, architecture, and functional specifications without prescribing implementation details.

**Companion Document:** See `MCP_SERVER_IMPLEMENTATION_GUIDE.md` for **HOW** to build it.

---

## 1. Executive Summary

### 1.1 Project Goal

Build an MCP (Model Context Protocol) server that enables AI assistants (ChatGPT, Claude, etc.) to interact with multiple Atlassian workspaces (Confluence, Jira) simultaneously through natural language.

### 1.2 Key Requirements

- Support multiple Atlassian organizations per user (e.g., "Providentia" and "ESO")
- Expose Confluence and Jira operations as MCP tools
- Authenticate users via Clerk
- Store encrypted API credentials per workspace
- Use microservices architecture with RabbitMQ messaging
- Deploy to Kubernetes for production scalability

### 1.3 MVP Use Cases

**Use Case 1: Cross-Organization Page Copy**
```
User: "Fetch page 123456 from ESO Confluence and create a copy in Providentia Confluence space DOCS"
```

**Use Case 2: Jira Summarization**
```
User: "Fetch all open Jira tasks from ESO project PLATFORM, summarize them, and create a summary page in ESO Confluence"
```

---

## 2. System Architecture

### 2.1 Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    AI Assistants Layer                       │
│         ChatGPT, Claude, Replit, Cody, Copilot, etc.        │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼ MCP Protocol (stdio/SSE)
┌─────────────────────────────────────────────────────────────┐
│                      MCP Server                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  • Protocol Handler (stdio/SSE/HTTP)                  │  │
│  │  • Tool Registry & Dispatch                           │  │
│  │  • User Authentication (Clerk)                        │  │
│  │  • Workspace Management                               │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼ RabbitMQ (RPC Pattern)
┌─────────────────────────────────────────────────────────────┐
│                   Backend Services Layer                     │
│  ┌──────────────────┐         ┌──────────────────┐          │
│  │ Confluence       │         │ Jira             │          │
│  │ Service          │         │ Service          │          │
│  │ - Get pages      │         │ - List issues    │          │
│  │ - Create pages   │         │ - Get issue      │          │
│  │ - Search         │         │ - Create issue   │          │
│  │ - Copy pages     │         │ - Add comment    │          │
│  └──────────────────┘         └──────────────────┘          │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼ HTTPS + Basic Auth
┌─────────────────────────────────────────────────────────────┐
│                  Atlassian Cloud APIs                        │
│    Providentia Workspace    │    ESO Workspace              │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Data Flow

1. **AI Assistant** sends MCP tool call (e.g., `confluence_get_page`)
2. **MCP Server** validates request, extracts workspace_id and user_id
3. **MCP Server** publishes RPC message to appropriate service queue (RabbitMQ)
4. **Backend Service** (Confluence/Jira) receives message
5. **Backend Service** retrieves workspace credentials from storage
6. **Backend Service** calls Atlassian API with Basic Auth
7. **Backend Service** returns response via RabbitMQ
8. **MCP Server** formats response and returns to AI Assistant

---

## 3. MCP Protocol Implementation

### 3.1 Protocol Specification

Implement Model Context Protocol as defined at: https://modelcontextprotocol.io/

**Key Concepts:**
- **Tools**: Functions AI can invoke (e.g., `confluence_get_page`)
- **Resources**: Data AI can read (future: page content as resources)
- **Prompts**: Pre-defined templates (future enhancement)

### 3.2 Transport Layers

Support multiple transport mechanisms:

| Transport | Use Case | Priority |
|-----------|----------|----------|
| **stdio** | Claude Desktop, local CLI tools | MVP |
| **SSE** | n8n, web-based integrations | MVP |
| **HTTP** | Custom integrations, webhooks | Phase 2 |

### 3.3 Required MCP Methods

| Method | Description | Response |
|--------|-------------|----------|
| `initialize` | Handshake, protocol version | Server capabilities |
| `tools/list` | List available tools | Array of tool definitions |
| `tools/call` | Execute a tool | Tool result or error |

---

## 4. Tool Definitions

### 4.1 Workspace Management Tools

#### `list_workspaces`
**Description:** List all configured Atlassian workspaces for the current user

**Parameters:** None

**Returns:**
```json
{
  "workspaces": [
    {
      "workspace_id": "providentia",
      "workspace_name": "Providentia Worldwide",
      "atlassian_url": "https://providentiaworldwide.atlassian.net"
    },
    {
      "workspace_id": "eso",
      "workspace_name": "ESO",
      "atlassian_url": "https://eso.atlassian.net"
    }
  ]
}
```

#### `workspace_status`
**Description:** Check connectivity to a workspace

**Parameters:**
- `workspace_id` (string, required): Workspace identifier

**Returns:**
```json
{
  "workspace_id": "eso",
  "status": "connected",
  "last_checked": "2024-01-15T10:30:00Z"
}
```

### 4.2 Confluence Tools

#### `confluence_get_page`
**Description:** Retrieve a Confluence page by ID

**Parameters:**
- `workspace_id` (string, required): Target workspace
- `page_id` (string, required): Confluence page ID

**Returns:**
```json
{
  "id": "123456",
  "title": "Project Documentation",
  "body": "<p>Page content in storage format</p>",
  "version": 5,
  "space_key": "DOCS"
}
```

#### `confluence_search`
**Description:** Search Confluence pages using CQL

**Parameters:**
- `workspace_id` (string, required)
- `query` (string, required): CQL search query
- `space_key` (string, optional): Limit to specific space
- `limit` (integer, optional, default: 25)

**Returns:**
```json
{
  "results": [
    {
      "id": "123456",
      "title": "Matching Page",
      "excerpt": "...highlighted text...",
      "space_key": "DOCS"
    }
  ],
  "total": 42
}
```

#### `confluence_create_page`
**Description:** Create a new Confluence page

**Parameters:**
- `workspace_id` (string, required)
- `space_key` (string, required)
- `title` (string, required)
- `body` (string, required): HTML content in storage format
- `parent_id` (string, optional): Parent page ID

**Returns:**
```json
{
  "id": "789012",
  "title": "New Page",
  "url": "https://workspace.atlassian.net/wiki/spaces/DOCS/pages/789012"
}
```

#### `confluence_update_page`
**Description:** Update an existing page

**Parameters:**
- `workspace_id` (string, required)
- `page_id` (string, required)
- `title` (string, optional)
- `body` (string, optional)
- `version` (integer, required): Current version number

**Returns:**
```json
{
  "id": "123456",
  "version": 6,
  "updated": true
}
```

#### `confluence_copy_page`
**Description:** Copy a page from one workspace to another (MVP Use Case 1)

**Parameters:**
- `src_workspace` (string, required): Source workspace ID
- `dst_workspace` (string, required): Destination workspace ID
- `src_page_id` (string, required): Page to copy
- `dst_space_key` (string, required): Destination space
- `dst_parent_id` (string, optional): Parent in destination
- `recursive` (boolean, optional, default: false): Copy child pages

**Returns:**
```json
{
  "src_page_id": "123456",
  "dst_page_id": "789012",
  "copied_children": 3,
  "url": "https://dst.atlassian.net/wiki/spaces/DOCS/pages/789012"
}
```

### 4.3 Jira Tools

#### `jira_list_issues`
**Description:** Search Jira issues using JQL

**Parameters:**
- `workspace_id` (string, required)
- `jql` (string, required): JQL query (e.g., "project = PLATFORM AND status = Open")
- `fields` (array, optional): Fields to return
- `limit` (integer, optional, default: 50)

**Returns:**
```json
{
  "issues": [
    {
      "key": "PLATFORM-123",
      "summary": "Fix authentication bug",
      "status": "In Progress",
      "assignee": "john.doe@example.com",
      "priority": "High"
    }
  ],
  "total": 15
}
```

#### `jira_get_issue`
**Description:** Get detailed information about a Jira issue

**Parameters:**
- `workspace_id` (string, required)
- `issue_key` (string, required): Issue key (e.g., "PLATFORM-123")

**Returns:**
```json
{
  "key": "PLATFORM-123",
  "summary": "Fix authentication bug",
  "description": "Detailed description...",
  "status": "In Progress",
  "assignee": "john.doe@example.com",
  "created": "2024-01-10T09:00:00Z",
  "updated": "2024-01-15T14:30:00Z",
  "comments": [...]
}
```

#### `jira_create_issue`
**Description:** Create a new Jira issue

**Parameters:**
- `workspace_id` (string, required)
- `project_key` (string, required)
- `issue_type` (string, required): "Task", "Bug", "Story", etc.
- `summary` (string, required)
- `description` (string, optional)
- `priority` (string, optional): "High", "Medium", "Low"
- `assignee` (string, optional): Email or account ID

**Returns:**
```json
{
  "key": "PLATFORM-456",
  "id": "10234",
  "url": "https://workspace.atlassian.net/browse/PLATFORM-456"
}
```

#### `jira_update_issue`
**Description:** Update Jira issue fields

**Parameters:**
- `workspace_id` (string, required)
- `issue_key` (string, required)
- `fields` (object, required): Fields to update

**Returns:**
```json
{
  "key": "PLATFORM-123",
  "updated": true
}
```

#### `jira_add_comment`
**Description:** Add a comment to a Jira issue

**Parameters:**
- `workspace_id` (string, required)
- `issue_key` (string, required)
- `body` (string, required): Comment text

**Returns:**
```json
{
  "comment_id": "10567",
  "created": "2024-01-15T15:00:00Z"
}
```

---

## 5. Authentication & Security

### 5.1 User Authentication

**Provider:** Clerk (https://clerk.dev)

**Flow:**
1. User authenticates via Clerk (web UI or API)
2. Clerk issues JWT token
3. MCP Server validates JWT on each request
4. User ID extracted from JWT for credential lookup

**Optional:** For local development, support unauthenticated mode with default user.

### 5.2 Atlassian API Credentials

**Storage Model:**
```
User (Clerk ID)
  └── Workspace 1 (e.g., "providentia")
      ├── workspace_id: "providentia"
      ├── workspace_name: "Providentia Worldwide"
      ├── atlassian_url: "https://providentiaworldwide.atlassian.net"
      ├── email: "user@providentia.com"
      └── api_token: [ENCRYPTED]
  └── Workspace 2 (e.g., "eso")
      ├── workspace_id: "eso"
      ├── workspace_name: "ESO"
      ├── atlassian_url: "https://eso.atlassian.net"
      ├── email: "user@eso.com"
      └── api_token: [ENCRYPTED]
```

**Security Requirements:**
- API tokens MUST be encrypted at rest (AES-256-GCM)
- API tokens MUST NOT appear in logs or error messages
- Credentials MUST be scoped per user
- Support both PostgreSQL and file-based storage

### 5.3 Atlassian API Authentication

**Method:** HTTP Basic Authentication

**Format:**
```
Authorization: Basic base64(email:api_token)
```

**Example:**
```bash
# Email: user@example.com
# Token: ATATT3xFfGF0...
# Encoded: dXNlckBleGFtcGxlLmNvbTpBVEFUVDN4RmZHRjAuLi4=

curl -H "Authorization: Basic dXNlckBleGFtcGxlLmNvbTpBVEFUVDN4RmZHRjAuLi4=" \
     https://workspace.atlassian.net/wiki/rest/api/content/123456
```

---

## 6. Data Models

### 6.1 Workspace Configuration

```go
type WorkspaceConfig struct {
    UserID        string    // Clerk user ID
    WorkspaceID   string    // User-defined label (e.g., "eso")
    WorkspaceName string    // Display name
    AtlassianURL  string    // Base URL (e.g., "https://eso.atlassian.net")
    Email         string    // Atlassian account email
    APIToken      string    // Encrypted API token
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

### 6.2 Service Request/Response

#### Confluence Request
```go
type ConfluenceRequest struct {
    Action      string         // "get_page", "create_page", etc.
    WorkspaceID string         // Target workspace
    UserID      string         // Clerk user ID
    Params      map[string]any // Action-specific parameters
    RequestID   string         // Correlation ID
}
```

#### Confluence Response
```go
type ConfluenceResponse struct {
    Success   bool
    Data      any
    Error     *ErrorInfo
    RequestID string
}
```

#### Error Info
```go
type ErrorInfo struct {
    Code    string // "AUTH_FAILED", "NOT_FOUND", etc.
    Message string
    Details any
}
```

### 6.3 Atlassian API Models

#### Confluence Page
```go
type ConfluencePage struct {
    ID       string
    Title    string
    SpaceKey string
    Body     struct {
        Storage struct {
            Value          string
            Representation string
        }
    }
    Version struct {
        Number int
    }
}
```

#### Jira Issue
```go
type JiraIssue struct {
    Key    string
    Fields struct {
        Summary     string
        Description string
        Status      struct {
            Name string
        }
        Assignee struct {
            DisplayName string
            EmailAddress string
        }
        Priority struct {
            Name string
        }
        Created time.Time
        Updated time.Time
    }
}
```

---

## 7. Atlassian API Endpoints

### 7.1 Confluence REST API

**Base URL:** `{atlassian_url}/wiki`

| Operation | Method | Endpoint |
|-----------|--------|----------|
| Get page | GET | `/rest/api/content/{id}?expand=body.storage,version` |
| Search | GET | `/rest/api/content/search?cql={query}` |
| Create page | POST | `/rest/api/content` |
| Update page | PUT | `/rest/api/content/{id}` |
| Get children | GET | `/rest/api/content/{id}/child/page` |
| List spaces | GET | `/rest/api/space` |

**Documentation:** https://developer.atlassian.com/cloud/confluence/rest/v2/

### 7.2 Jira REST API

**Base URL:** `{atlassian_url}`

| Operation | Method | Endpoint |
|-----------|--------|----------|
| Search issues | POST | `/rest/api/3/search` |
| Get issue | GET | `/rest/api/3/issue/{issueIdOrKey}` |
| Create issue | POST | `/rest/api/3/issue` |
| Update issue | PUT | `/rest/api/3/issue/{issueIdOrKey}` |
| Add comment | POST | `/rest/api/3/issue/{issueIdOrKey}/comment` |

**Documentation:** https://developer.atlassian.com/cloud/jira/platform/rest/v3/

---

## 8. Deployment Architecture

### 8.1 Local Development

**Components:**
- RabbitMQ (Docker)
- PostgreSQL (Docker)
- MCP Server (local process)
- Confluence Service (local process)
- Jira Service (local process)

**Start Command:**
```bash
docker-compose up -d
go run ./cmd/mcp-server
go run ./cmd/confluence-service
go run ./cmd/jira-service
```

### 8.2 Production (Kubernetes)

**Deployment Units:**
- `mcp-server` deployment (3 replicas)
- `confluence-service` deployment (3 replicas)
- `jira-service` deployment (3 replicas)
- RabbitMQ StatefulSet (3 nodes, quorum queues)
- PostgreSQL StatefulSet (primary + replica)

**Scaling Strategy:**
- Horizontal scaling via replica count
- RabbitMQ quorum queues for durability
- Load balancing via Kubernetes Service

---

## 9. MVP Acceptance Criteria

### 9.1 Functional Requirements

- [ ] MCP server responds to `initialize` request
- [ ] MCP server responds to `tools/list` with all tools
- [ ] `list_workspaces` returns configured workspaces
- [ ] `confluence_get_page` retrieves page from specified workspace
- [ ] `confluence_create_page` creates page in specified workspace
- [ ] `confluence_copy_page` copies page between workspaces (Use Case 1)
- [ ] `jira_list_issues` returns issues matching JQL
- [ ] `jira_get_issue` returns issue details
- [ ] Cross-workspace operations work (ESO → Providentia)
- [ ] Jira summarization + Confluence page creation works (Use Case 2)

### 9.2 Non-Functional Requirements

- [ ] API tokens encrypted at rest
- [ ] No tokens in logs or error messages
- [ ] Services communicate via RabbitMQ (no direct HTTP)
- [ ] Local development environment works with docker-compose
- [ ] All services use TwistyGo library
- [ ] Configuration via config.yaml and settings.yaml
- [ ] Error handling with standard error codes

---

## 10. Future Enhancements (Post-MVP)

### 10.1 Phase 2 Features

- Attachment handling (upload/download)
- Confluence page versioning and history
- Jira transitions and workflows
- Bulk operations (copy multiple pages, update multiple issues)
- Caching layer (Redis) for frequently accessed data
- Rate limiting per user/workspace
- Audit logging for compliance

### 10.2 Phase 3 Features

- Additional Atlassian products (Bitbucket, Trello)
- Slack integration service
- Git integration service
- Email notification service
- Webhook support for real-time updates
- MCP Resources (expose pages as readable resources)
- MCP Prompts (pre-defined templates)

---

## 11. Reference Implementation

**Source:** https://github.com/providentiaww/eso-tools/tree/master/confluence

This Python implementation demonstrates:
- Multi-workspace credential management
- Cross-workspace page copying
- Confluence API patterns (get, create, search)
- Attachment handling
- Recursive page copying

**Key Patterns to Replicate:**
1. Separate credential sets per workspace
2. Dynamic client creation based on workspace_id
3. Cross-workspace operations using two authenticated clients
4. Storage format for page content

---

## 12. Glossary

| Term | Definition |
|------|------------|
| **MCP** | Model Context Protocol - standard for AI-tool communication |
| **Workspace** | User-defined label for an Atlassian organization (e.g., "eso", "providentia") |
| **Tool** | MCP function that AI can invoke (e.g., `confluence_get_page`) |
| **RPC** | Remote Procedure Call - request/response message pattern |
| **CQL** | Confluence Query Language - search syntax |
| **JQL** | Jira Query Language - issue search syntax |
| **Storage Format** | Confluence's internal HTML representation |

---

**Document Version:** 1.0  
**Last Updated:** January 2025  
**Status:** MVP Requirements
