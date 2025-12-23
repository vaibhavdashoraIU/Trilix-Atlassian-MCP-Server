package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/providentiaww/twistygo"
	"github.com/providentiaww/trilix-atlassian-mcp/cmd/mcp-server/auth"
	"github.com/providentiaww/trilix-atlassian-mcp/cmd/mcp-server/handlers"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/storage"
	"github.com/providentiaww/trilix-atlassian-mcp/pkg/mcp"
	amqp "github.com/rabbitmq/amqp091-go"
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"gopkg.in/yaml.v3"
)

const ServiceVersion = "v1.0.0"

var rconn *twistygo.AmqpConnection_t

type AppConfig struct {
	Common struct {
		App struct {
			Port       int    `yaml:"port"`
			RPCTimeout string `yaml:"rpc_timeout"`
		} `yaml:"app"`
	} `yaml:"common"`
}

func init() {
	// Load environment variables FIRST from project root or current dir
	envFile := os.Getenv("ENV_FILE_PATH")
	if envFile == "" {
		envFile = "../../.env"
	}

	if err := godotenv.Load(envFile); err != nil {
		// Try current directory as fallback
		if err := godotenv.Load(); err != nil {
			// Don't log if running in K8s/Docker where env is injected
			if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
				fmt.Printf("Note: .env file not found at %s. Using system environment variables.\n", envFile)
			}
		}
	}
}

func main() {
	// Initialize TwistyGo logging
	twistygo.LogStartService("MCPServer", ServiceVersion)

	// Initialize RabbitMQ with retries
	maxRetries := 5
	var err error
	for i := 0; i < maxRetries; i++ {
		// Capture panic from twistygo.AmqpConnect if it fails
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			rconn = twistygo.AmqpConnect()
			if rconn != nil {
				err = nil
			}
		}()

		if err == nil && rconn != nil {
			break
		}

		if i < maxRetries-1 {
			fmt.Printf("⚠️ Failed to connect to RabbitMQ (attempt %d/%d): %v. Retrying in 5s...\n", i+1, maxRetries, err)
			time.Sleep(5 * time.Second)
		}
	}

	if rconn == nil {
		panic(fmt.Sprintf("❌ Failed to connect to RabbitMQ after %d attempts: %v", maxRetries, err))
	}
	rconn.AmqpLoadQueues("ConfluenceRequests", "JiraRequests")

	// Initialize credential store (file-based or database) with retries for K8s resilience
	var credStore storage.CredentialStoreInterface
	for i := 0; i < maxRetries; i++ {
		credStore, err = storage.NewCredentialStoreFromEnv()
		if err == nil {
			break
		}
		if i < maxRetries-1 {
			fmt.Printf("⚠️ Failed to initialize credential store (attempt %d/%d): %v. Retrying in 5s...\n", i+1, maxRetries, err)
			time.Sleep(5 * time.Second)
		}
	}
	if err != nil {
		panic(fmt.Sprintf("❌ Failed to initialize credential store after %d attempts: %v", maxRetries, err))
	}
	defer credStore.Close()

	// Initialize Clerk authentication
	clerkAuth := auth.NewClerkAuth()
	if clerkAuth == nil {
		fmt.Println("Warning: Clerk authentication not configured (CLERK_SECRET_KEY not set)")
		fmt.Println("Running in development mode without authentication")
	}

	// Load custom config
	var appConfig AppConfig
	if configData, err := os.ReadFile("config.yaml"); err == nil {
		yaml.Unmarshal(configData, &appConfig)
	}
	
	port := appConfig.Common.App.Port
	if port == 0 {
		port = 3000
	}
	// Allow environment variable override for K8s
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	rpcTimeout := 35 * time.Second
	if appConfig.Common.App.RPCTimeout != "" {
		if d, err := time.ParseDuration(appConfig.Common.App.RPCTimeout); err == nil {
			rpcTimeout = d
		}
	}

	// Create service callers with configurable timeout
	confluenceCaller := createConfluenceCaller(rpcTimeout)
	jiraCaller := createJiraCaller(rpcTimeout)

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
	// Use FRONTEND_PATH override for containerization
	frontendPath := os.Getenv("FRONTEND_PATH")
	if frontendPath == "" {
		frontendPath = "../../frontend"
	}

	// Root path serves index.html
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(frontendPath, "index.html"))
			return
		}
		// For other paths, serve from root directory
		http.FileServer(http.Dir(frontendPath)).ServeHTTP(w, r)
	})
	
	// Map frontend URLs to new frontend folder
	mux.HandleFunc("/docs/test-client.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(frontendPath, "test-client.html"))
	})
	mux.HandleFunc("/workspaces.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(frontendPath, "workspaces.html"))
	})
	mux.HandleFunc("/index.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(frontendPath, "index.html"))
	})
	mux.HandleFunc("/trilix-preview.jsx", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(frontendPath, "trilix-preview.jsx"))
	})

	// 2. Global Request Logger
	mux.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(os.Stderr, "GLOBAL LOG: %s %s from %s\n", r.Method, r.URL.Path, r.RemoteAddr)
		http.NotFound(w, r)
	})

	// 3. Workspace Management API
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

	mux.HandleFunc("/api/workspaces", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(os.Stderr, "GLOBAL LOG: %s %s\n", r.Method, r.URL.Path)
		workspaceRouteHandler.ServeHTTP(w, r)
	})
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
		restToolHandler := handlers.NewRestToolHandler(confluenceHandler, jiraHandler, managementHandler)
		mux.HandleFunc("/api/tools/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(os.Stderr, "GLOBAL LOG: %s %s\n", r.Method, r.URL.Path)
			authMiddleware.Handler(http.HandlerFunc(restToolHandler.HandleToolRequest)).ServeHTTP(w, r)
		})
		
	} else {
		// Dev mode
		mux.HandleFunc("/api/workspaces", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				workspaceHandler.HandleListWorkspaces(w, r)
			} else if r.Method == "POST" {
				workspaceHandler.HandleCreateWorkspace(w, r)
			}
		})
		restToolHandler := handlers.NewRestToolHandler(confluenceHandler, jiraHandler, managementHandler)
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

	// Deep Health Check Endpoint (for Kubernetes)
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		status := "UP"
		details := make(map[string]string)
		code := http.StatusOK

		// Check Database
		if err := credStore.Ping(); err != nil {
			status = "DOWN"
			details["database"] = fmt.Sprintf("DOWN: %v", err)
			code = http.StatusServiceUnavailable
		} else {
			details["database"] = "UP"
		}

		// Check RabbitMQ (TwistyGo)
		// We use a simple nil check here as twistygo handles the internal connection state.
		if rconn != nil {
			details["rabbitmq"] = "UP"
		} else {
			status = "DOWN"
			details["rabbitmq"] = "DOWN"
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  status,
			"details": details,
			"version": ServiceVersion,
		})
	})

	// Apply CORS to everything
	handlerWithCors := corsMiddleware(mux)

	// Setup Server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handlerWithCors,
	}

	// 4. Handle Graceful Shutdown (SIGTERM/SIGINT)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("🚀 Starting Unified Trilix Server on port %d...\n", port)
		fmt.Printf("   - Dashboard:    http://localhost:%d/\n", port)
		fmt.Printf("   - Health:       http://localhost:%d/api/health\n", port)
		fmt.Printf("   - Test Client:  http://localhost:%d/docs/test-client.html\n", port)
		fmt.Printf("   - Workspaces:   http://localhost:%d/workspaces.html\n", port)
		fmt.Printf("   - API List:     http://localhost:%d/api/workspaces\n", port)
		
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Sprintf("❌ Failed to start server: %v", err))
		}
	}()

	// Wait for termination signal
	<-stop
	fmt.Println("\n🛑 Shutting down server...")

	// Create a timeout context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("❌ Server forced to shutdown: %v\n", err)
	}

	fmt.Println("👋 Server exited gracefully")
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


// cloneServiceQueue creates a shallow-ish copy of a ServiceQueue_t with its own Message
// and ResponseQueue to avoid race conditions during concurrent tool calls.
func cloneServiceQueue(src *twistygo.ServiceQueue_t) *twistygo.ServiceQueue_t {
	if src == nil {
		return nil
	}
	dst := *src
	dst.Message = twistygo.MessageSet_t{}
	dst.ResponseQueue = &amqp.Queue{}
	dst.Headers = make(amqp.Table)
	if src.Headers != nil {
		for k, v := range src.Headers {
			dst.Headers[k] = v
		}
	}
	// Deep copy Queue parameters because twistygo modifies sq.Queue.Args in publishRPC
	if src.Queue != nil {
		qCopy := *src.Queue
		if src.Queue.Args != nil {
			argsCopy := make(amqp.Table)
			for k, v := range *src.Queue.Args {
				argsCopy[k] = v
			}
			qCopy.Args = &argsCopy
		}
		dst.Queue = &qCopy
	}
	return &dst
}

func createConfluenceCaller(rpcTimeout time.Duration) func(models.ConfluenceRequest) (*models.ConfluenceResponse, error) {
	return func(req models.ConfluenceRequest) (*models.ConfluenceResponse, error) {
		// Connect to ConfluenceRequests queue
		sqGlobal := rconn.AmqpConnectQueue("ConfluenceRequests")
		sq := cloneServiceQueue(sqGlobal)
		if sq == nil {
			return nil, fmt.Errorf("confluence queue not initialized")
		}
		sq.SetEncoding(twistygo.EncodingJson)

		// Marshal single request as object (not array) for the RPC payload
		reqBytes, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		sq.Message.ResetDataList()
		sq.Message.AppendData(req)
		sq.Message.Encoded = reqBytes

		// Publish and wait for response (RPC) with timeout
		type publishResult struct {
			resp []byte
			err  error
		}
		resChan := make(chan publishResult, 1)
		go func() {
			resp, err := sq.Publish()
			resChan <- publishResult{resp, err}
		}()

		var responseBytes []byte
		select {
		case res := <-resChan:
			if res.err != nil {
				return nil, res.err
			}
			responseBytes = res.resp
		case <-time.After(rpcTimeout):
			return nil, fmt.Errorf("RPC timeout: confluence service did not respond within %v", rpcTimeout)
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

func createJiraCaller(rpcTimeout time.Duration) func(models.JiraRequest) (*models.JiraResponse, error) {
	return func(req models.JiraRequest) (*models.JiraResponse, error) {
		// Connect to JiraRequests queue
		sqGlobal := rconn.AmqpConnectQueue("JiraRequests")
		sq := cloneServiceQueue(sqGlobal)
		if sq == nil {
			return nil, fmt.Errorf("jira queue not initialized")
		}
		sq.SetEncoding(twistygo.EncodingJson)

		// Marshal single request as object (not array) for the RPC payload
		reqBytes, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		sq.Message.ResetDataList()
		sq.Message.AppendData(req)
		sq.Message.Encoded = reqBytes

		// Publish and wait for response (RPC) with timeout
		type publishResult struct {
			resp []byte
			err  error
		}
		resChan := make(chan publishResult, 1)
		go func() {
			resp, err := sq.Publish()
			resChan <- publishResult{resp, err}
		}()

		var responseBytes []byte
		select {
		case res := <-resChan:
			if res.err != nil {
				return nil, res.err
			}
			responseBytes = res.resp
		case <-time.After(rpcTimeout):
			return nil, fmt.Errorf("RPC timeout: jira service did not respond within %v", rpcTimeout)
		}

		// Unmarshal response
		var response models.JiraResponse
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			return nil, err
		}

		return &response, nil
	}
}

