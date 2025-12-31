package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/providentiaww/trilix-atlassian-mcp/cmd/mcp-server/handlers"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/config"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/models"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/storage"
	"github.com/providentiaww/trilix-atlassian-mcp/pkg/mcp"
	"github.com/providentiaww/twistygo"
)

var rconn *twistygo.AmqpConnection_t

func init() {
	config.LoadEnv("../../.env")
	twistygo.LogStartService("MCPStdio", "1.0.0")
	rconn = twistygo.AmqpConnect()
	rconn.AmqpLoadQueues("ConfluenceRequests", "JiraRequests")
}

func main() {
	credStore, err := storage.NewCredentialStoreFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize credential store: %v\n", err)
		os.Exit(1)
	}
	defer credStore.Close()

	confluenceCaller := createConfluenceCaller()
	jiraCaller := createJiraCaller()

	confluenceHandler := handlers.NewConfluenceHandler(confluenceCaller)
	jiraHandler := handlers.NewJiraHandler(jiraCaller)
	managementHandler := handlers.NewManagementHandler(credStore)

	server := mcp.NewServer()

	for _, tool := range confluenceHandler.ListTools() {
		server.RegisterTool(tool)
	}
	for _, tool := range jiraHandler.ListTools() {
		server.RegisterTool(tool)
	}
	for _, tool := range managementHandler.ListTools() {
		server.RegisterTool(tool)
	}

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

	server.Start(handler)
}

func createConfluenceCaller() func(models.ConfluenceRequest) (*models.ConfluenceResponse, error) {
	return func(req models.ConfluenceRequest) (*models.ConfluenceResponse, error) {
		sq := rconn.AmqpConnectQueue("ConfluenceRequests")
		sq.SetEncoding(twistygo.EncodingJson)
		sq.Message.AppendData(req)
		responseBytes, err := sq.Publish()
		if err != nil {
			return nil, err
		}
		var response models.ConfluenceResponse
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			return nil, err
		}
		return &response, nil
	}
}

func createJiraCaller() func(models.JiraRequest) (*models.JiraResponse, error) {
	return func(req models.JiraRequest) (*models.JiraResponse, error) {
		sq := rconn.AmqpConnectQueue("JiraRequests")
		sq.SetEncoding(twistygo.EncodingJson)
		sq.Message.AppendData(req)
		responseBytes, err := sq.Publish()
		if err != nil {
			return nil, err
		}
		var response models.JiraResponse
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			return nil, err
		}
		return &response, nil
	}
}
