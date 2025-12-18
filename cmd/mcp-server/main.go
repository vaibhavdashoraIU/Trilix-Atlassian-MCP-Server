package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/joho/godotenv"
	"github.com/providentiaww/twistygo"
	"github.com/providentiaww/trilix-atlassian-mcp/cmd/mcp-server/auth"
	"github.com/providentiaww/trilix-atlassian-mcp/cmd/mcp-server/handlers"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/storage"
	"github.com/providentiaww/trilix-atlassian-mcp/pkg/mcp"
)

const ServiceVersion = "v1.0.0"

var rconn *twistygo.AmqpConn_t

func init() {
	// Load environment variables FIRST from project root
	if err := godotenv.Load("../../.env"); err != nil {
		// Try current directory as fallback
		if err := godotenv.Load(); err != nil {
			fmt.Printf("Warning: .env file not found: %v\n", err)
		}
	}

	// Initialize TwistyGo
	twistygo.LogStartService("MCPServer", ServiceVersion)
	rconn = twistygo.AmqpConnect()
	rconn.AmqpLoadQueues("ConfluenceRequests", "JiraRequests")
}

func main() {
	// Initialize credential store (file-based or database)
	credStore, err := storage.NewCredentialStoreFromEnv()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize credential store: %v", err))
	}
	defer credStore.Close()

	// Initialize Clerk authentication
	clerkAuth := auth.NewClerkAuth()
	if clerkAuth == nil {
		fmt.Println("Warning: Clerk authentication not configured (CLERK_SECRET_KEY not set)")
		fmt.Println("Running in development mode without authentication")
	}

	// Create service callers
	confluenceCaller := createConfluenceCaller()
	jiraCaller := createJiraCaller()

	// Create handlers
	confluenceHandler := handlers.NewConfluenceHandler(confluenceCaller)
	jiraHandler := handlers.NewJiraHandler(jiraCaller)
	managementHandler := handlers.NewManagementHandler(credStore)
	workspaceHandler := handlers.NewWorkspaceHandler(credStore)

	// Create MCP server
	server := mcp.NewServer()

	// Register all tools
	for _, tool := range confluenceHandler.ListTools() {
		server.RegisterTool(tool)
	}
	for _, tool := range jiraHandler.ListTools() {
		server.RegisterTool(tool)
	}
	for _, tool := range managementHandler.ListTools() {
		server.RegisterTool(tool)
	}

	// Create handler function with userID support
	handler := func(call mcp.ToolCall, userID string) (mcp.ToolResult, error) {
		if call.Name == "list_workspaces" || call.Name == "workspace_status" {
			return managementHandler.HandleTool(call, userID)
		} else if len(call.Name) >= 10 && call.Name[:10] == "confluence" {
			return confluenceHandler.HandleTool(call, userID)
		} else if len(call.Name) >= 5 && call.Name[:5] == "jira_" {
			return jiraHandler.HandleTool(call, userID)
		}

		return mcp.ToolResult{
			Content: []mcp.ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", call.Name)},
			},
			IsError: true,
		}, fmt.Errorf("unknown tool: %s", call.Name)
	}

	// Setup router
	mux := http.NewServeMux()

	// 1. Static File Serving (Replaces Python server)
	// Serve specific frontend files from frontend folder
	
	// Root path serves index.html
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "../../frontend/index.html")
			return
		}
		// For other paths, serve from root directory
		http.FileServer(http.Dir("../../")).ServeHTTP(w, r)
	})
	
	// Map frontend URLs to new frontend folder
	mux.HandleFunc("/docs/test-client.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../../frontend/test-client.html")
	})
	mux.HandleFunc("/workspaces.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../../frontend/workspaces.html")
	})
	mux.HandleFunc("/index.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../../frontend/index.html")
	})
	mux.HandleFunc("/trilix-preview.jsx", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../../frontend/trilix-preview.jsx")
	})

	// 2. Workspace Management API
	if clerkAuth != nil {
		authMiddleware := auth.RequireAuth(clerkAuth)
		
		workspaceRouteHandler := authMiddleware.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				workspaceHandler.HandleListWorkspaces(w, r)
			case http.MethodPost:
				workspaceHandler.HandleCreateWorkspace(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})

		mux.Handle("/api/workspaces", workspaceRouteHandler)
	mux.Handle("/api/workspaces/", authMiddleware.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/workspaces/" {
				workspaceRouteHandler.ServeHTTP(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/status") {
				workspaceHandler.HandleWorkspaceStatus(w, r)
			} else if r.Method == http.MethodDelete {
				workspaceHandler.HandleDeleteWorkspace(w, r)
			} else if r.Method == http.MethodPut {
				workspaceHandler.HandleUpdateWorkspace(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}))

		// REST Tool Execution (for ChatGPT)
		restToolHandler := handlers.NewRestToolHandler(confluenceHandler, jiraHandler)
		mux.Handle("/api/tools/", authMiddleware.HandlerFunc(restToolHandler.HandleToolRequest))
		
	} else {
		// Dev mode
		mux.HandleFunc("/api/workspaces", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				workspaceHandler.HandleListWorkspaces(w, r)
			} else if r.Method == "POST" {
				workspaceHandler.HandleCreateWorkspace(w, r)
			}
		})
		restToolHandler := handlers.NewRestToolHandler(confluenceHandler, jiraHandler)
		mux.HandleFunc("/api/tools/", restToolHandler.HandleToolRequest)
	}

	// 3. SSE Server (Replaces port 3000)
	sseServer := mcp.NewSSEServer(server, handler)
	
	// Create SSE handler with Auth if configured
	var sseHandler http.Handler
	if clerkAuth != nil {
		authMiddleware := auth.RequireAuth(clerkAuth)
		// SSE endpoint needs auth
		sseHandler = authMiddleware.HandlerFunc(sseServer.HandleSSE)
	} else {
		sseHandler = http.HandlerFunc(sseServer.HandleSSE)
	}

	mux.Handle("/sse", sseHandler)
	mux.Handle("/message", http.HandlerFunc(sseServer.HandleMessage)) // Message posting usually uses same auth header

	// Apply CORS to everything
	handlerWithCors := corsMiddleware(mux)

	// Start Single Server
	port := 3000
	fmt.Printf("🚀 Starting Unified Trilix Server on port %d...\n", port)
	fmt.Printf("   - Dashboard:    http://localhost:%d/\n", port)
	fmt.Printf("   - Test Client:  http://localhost:%d/docs/test-client.html\n", port)
	fmt.Printf("   - Workspaces:   http://localhost:%d/workspaces.html\n", port)
	fmt.Printf("   - API:          http://localhost:%d/api/workspaces\n", port)
	fmt.Printf("   - SSE:          http://localhost:%d/sse\n", port)
	
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), handlerWithCors); err != nil {
		panic(fmt.Sprintf("Failed to start server: %v", err))
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}


func createConfluenceCaller() func(models.ConfluenceRequest) (*models.ConfluenceResponse, error) {
	return func(req models.ConfluenceRequest) (*models.ConfluenceResponse, error) {
		// Connect to ConfluenceRequests queue
		sq := rconn.AmqpConnectQueue("ConfluenceRequests")
		sq.SetEncoding(twistygo.EncodingJson)

		// Marshal single request as object (not array) for the RPC payload
		reqBytes, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		sq.Message.ResetDataList()
		sq.Message.AppendData(req)
		sq.Message.Encoded = reqBytes

		// Publish and wait for response (RPC)
		responseBytes, err := sq.Publish()
		if err != nil {
			return nil, err
		}

		// Debug log raw response to aid troubleshooting unexpected payload shapes
		fmt.Printf("Confluence RPC raw response: %s\n", string(responseBytes))

		// Unmarshal response
		var response models.ConfluenceResponse
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			return nil, err
		}

		return &response, nil
	}
}

func createJiraCaller() func(models.JiraRequest) (*models.JiraResponse, error) {
	return func(req models.JiraRequest) (*models.JiraResponse, error) {
		// Connect to JiraRequests queue
		sq := rconn.AmqpConnectQueue("JiraRequests")
		sq.SetEncoding(twistygo.EncodingJson)

		// Marshal single request as object (not array) for the RPC payload
		reqBytes, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		sq.Message.ResetDataList()
		sq.Message.AppendData(req)
		sq.Message.Encoded = reqBytes

		// Publish and wait for response (RPC)
		responseBytes, err := sq.Publish()
		if err != nil {
			return nil, err
		}

		// Unmarshal response
		var response models.JiraResponse
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			return nil, err
		}

		return &response, nil
	}
}

