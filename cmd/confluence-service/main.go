package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/providentiaww/twistygo"
	"github.com/providentiaww/trilix-atlassian-mcp/cmd/confluence-service/handlers"
	"github.com/providentiaww/trilix-atlassian-mcp/internal/storage"
	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
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
	twistygo.LogStartService("ConfluenceService", ServiceVersion)

	// Connect to RabbitMQ (uses config.yaml)
	rconn = twistygo.AmqpConnect()

	// Load queue definitions from settings.yaml
	rconn.AmqpLoadQueues("ConfluenceRequests")

	// Load service definitions
	rconn.AmqpLoadServices("ConfluenceService")
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
	svc := rconn.AmqpConnectService("ConfluenceService")
	if svc == nil {
		panic("Failed to connect to ConfluenceService queue")
	}

	// Manual multi-threaded service loop to avoid twistygo single-threaded bottleneck
	msgs, err := svc.Amqp.Channel.Consume(
		svc.Queue.Name,      // queue
		"",                 // consumer
		svc.Queue.AutoAck,   // auto-ack
		svc.Queue.Exclusive, // exclusive
		false,              // no-local
		svc.Queue.NoWait,    // no-wait
		nil,                // args
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to start consumer: %v", err))
	}

	go func() {
		for d := range msgs {
			go func(delivery amqp.Delivery) {
				// Process in goroutine
				responseBytes := service.HandleRequest(delivery)

				// Use twistygo's global channel to publish reply
				// We need to set the ReplyTo and the response data
				err := svc.Amqp.Channel.Publish(
					"",               // exchange
					delivery.ReplyTo, // routing key (the reply queue)
					false,            // mandatory
					false,            // immediate
					amqp.Publishing{
						ContentType:   "application/json",
						CorrelationId: delivery.CorrelationId,
						Body:          responseBytes,
					},
				)
				if err != nil {
					fmt.Printf("Error publishing reply: %v\n", err)
				}
			}(d)
		}
	}()

	fmt.Printf("Confluence Service v%s is running (Multi-threaded). To exit press CTRL+C\n", ServiceVersion)
	
	// Wait for termination signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("Shutting down Confluence Service...")
}
