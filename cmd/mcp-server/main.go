package main

import (
	"encoding/json"
	"fmt"

	"github.com/joho/godotenv"
	"github.com/providentiaww/twistygo"
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

	// Create service callers
	confluenceCaller := createConfluenceCaller()
	jiraCaller := createJiraCaller()

	// Create handlers
	confluenceHandler := handlers.NewConfluenceHandler(confluenceCaller)
	jiraHandler := handlers.NewJiraHandler(jiraCaller)
	managementHandler := handlers.NewManagementHandler(credStore)

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

	// Create handler function
	handler := func(call mcp.ToolCall) (mcp.ToolResult, error) {
		userID := ""

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

	// Start SSE server for n8n MCP integration
	sseServer := mcp.NewSSEServer(server, handler)
	port := 3001
	fmt.Printf("Starting MCP SSE Server on port %d...\n", port)
	if err := sseServer.StartSSE(port); err != nil {
		panic(fmt.Sprintf("Failed to start SSE server: %v", err))
	}
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

