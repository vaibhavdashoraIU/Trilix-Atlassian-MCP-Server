package main

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/providentiaww/twistygo"
	"github.com/providentiaww/trilix-atlassian-mcp/cmd/jira-service/handlers"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/storage"
	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

const ServiceVersion = "v1.0.0"

var rconn *twistygo.AmqpConnection_t

type AppConfig struct {
	Atlassian struct {
		Timeout string `yaml:"timeout"`
	} `yaml:"atlassian"`
}

func init() {
	// Load environment variables FIRST from project root
	if err := godotenv.Load("../../.env"); err != nil {
		// Try current directory as fallback
		if err := godotenv.Load(); err != nil {
			fmt.Printf("Warning: .env file not found: %v\n", err)
		}
	}

	// Initialize TwistyGo with service name
	twistygo.LogStartService("JiraService", ServiceVersion)

	// Connect to RabbitMQ (uses config.yaml)
	rconn = twistygo.AmqpConnect()

	// Load queue definitions from settings.yaml
	rconn.AmqpLoadQueues("JiraRequests")

	// Load service definitions
	rconn.AmqpLoadServices("JiraService")
}

func main() {
	// Load custom config for timeout
	var appConfig AppConfig
	if configData, err := os.ReadFile("config.yaml"); err == nil {
		yaml.Unmarshal(configData, &appConfig)
	}
	timeout := 30 * time.Second
	if appConfig.Atlassian.Timeout != "" {
		if d, err := time.ParseDuration(appConfig.Atlassian.Timeout); err == nil {
			timeout = d
		}
	}

	// Initialize credential store (file-based or database)
	credStore, err := storage.NewCredentialStoreFromEnv()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize credential store: %v", err))
	}
	defer credStore.Close()

	// Create service handler
	service := handlers.NewService(credStore, timeout)

	// Get service handle
	svc := rconn.AmqpConnectService("JiraService")

	// Defensive check: ensure ResponseQueue is initialized if the library missed it
	if svc != nil && svc.ResponseQueue == nil {
		svc.ResponseQueue = &amqp.Queue{}
	}

	// Start listening with handler function
	svc.StartService(func(d amqp.Delivery) []byte {
		return service.HandleRequest(d)
	})
}

