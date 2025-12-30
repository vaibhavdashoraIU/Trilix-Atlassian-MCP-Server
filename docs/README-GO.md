# Trilix Atlassian MCP Server (Go Implementation)

This is the Go-based microservices implementation of the Trilix Atlassian MCP Server, following the architecture described in `CURSOR_RULES.md` and `TRILIX-ATLASSIAN-MCP-SERVER-REQUIREMENTS.md`.

## Architecture

The system consists of three main services:

1. **MCP Server** (`cmd/mcp-server`) - Handles MCP protocol communication and routes requests to backend services
2. **Confluence Service** (`cmd/confluence-service`) - Handles all Confluence API operations
3. **Jira Service** (`cmd/jira-service`) - Handles all Jira API operations

All services communicate via RabbitMQ using the TwistyGo library.

## Prerequisites

- Go 1.22+
- Docker and Docker Compose (for local development)
- PostgreSQL (for credential storage)
- RabbitMQ (for service communication)
- TwistyGo library (cloned locally at `D:\Idea Usher\twistygo`)

**Note:** The `go.mod` file is configured to use the local TwistyGo library via a `replace` directive.

## Local Development Setup

See [SETUP.md](SETUP.md) for detailed setup instructions.

Quick start:

1. **Start infrastructure services:**
   ```bash
   docker-compose up -d
   ```

2. **Configure environment:**
   ```bash
   copy .env.example .env
   # Edit .env and set API_KEY_ENCRYPTION_KEY (generate with: openssl rand -hex 32)
   ```

3. **Install dependencies:**
   ```bash
   go mod download
   ```

4. **Run services in separate terminals:**
   ```bash
   # Terminal 1: Confluence Service
   cd cmd/confluence-service && go run main.go
   
   # Terminal 2: Jira Service  
   cd cmd/jira-service && go run main.go
   
   # Terminal 3: MCP Server
   cd cmd/mcp-server && go run main.go
   ```

## Building

```bash
# Build all services
go build -o bin/mcp-server ./cmd/mcp-server
go build -o bin/confluence-service ./cmd/confluence-service
go build -o bin/jira-service ./cmd/jira-service
```

## Deployment

Use `twistydeploy` for Kubernetes deployment:

```bash
# Build and deploy
twistydeploy -m manifest.yaml -d production -g
twistydeploy -m manifest.yaml -d production
```

## Project Structure

```
trilix-atlassian-mcp/
├── cmd/
│   ├── mcp-server/           # MCP protocol server
│   │   ├── main.go
│   │   ├── config.yaml
│   │   ├── handlers/         # MCP tool handlers
│   │   └── auth/             # Clerk integration
│   ├── confluence-service/   # Confluence API service
│   │   ├── main.go
│   │   ├── config.yaml
│   │   ├── settings.yaml
│   │   ├── api/              # Atlassian API client
│   │   └── handlers/
│   └── jira-service/         # Jira API service
│       ├── main.go
│       ├── config.yaml
│       ├── settings.yaml
│       ├── api/
│       └── handlers/
├── internal/
│   ├── models/               # Shared data models
│   ├── crypto/               # API key encryption
│   └── storage/              # PostgreSQL credential storage
├── pkg/
│   └── mcp/                  # MCP protocol implementation
├── manifest.yaml             # K8s deployment manifest
└── docker-compose.yaml       # Local development
```

## Key Features

- **Multi-workspace support**: Connect to multiple Atlassian organizations simultaneously
- **Encrypted credential storage**: API tokens encrypted at rest in PostgreSQL
- **Clerk authentication**: Optional user authentication via Clerk
- **Microservices architecture**: Scalable, distributed system using RabbitMQ
- **MCP protocol**: Standard Model Context Protocol for AI assistant integration

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...
```

## Notes

- The TwistyGo library must be available in your Go module path
- Ensure RabbitMQ and PostgreSQL are running before starting services
- API tokens are encrypted using AES-256-GCM
- All services require both `config.yaml` and `settings.yaml` files

