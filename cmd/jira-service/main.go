package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/providentiaww/twistygo"
	"github.com/providentiaww/trilix-atlassian-mcp/cmd/jira-service/handlers"
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
	// Initialize TwistyGo
	twistygo.LogStartService("JiraService", ServiceVersion)

	// Initialize RabbitMQ with retries
	maxRetries := 5
	var err error
	for i := 0; i < maxRetries; i++ {
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

	// Load queue and service definitions
	rconn.AmqpLoadQueues("JiraRequests")
	rconn.AmqpLoadServices("JiraService")

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

	// Initialize credential store (file-based or database) with retries
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

	// Create service handler
	service := handlers.NewService(credStore, timeout)

	// Get service handle
	svc := rconn.AmqpConnectService("JiraService")
	if svc == nil {
		panic("Failed to connect to JiraService queue")
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
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("❌ Consumer panic recovered: %v\n", r)
						// Nack the message so it might be retried or dead-lettered
						// Requeue=false to avoid infinite loop of death if it's deterministic
						delivery.Nack(false, false)
					}
				}()

				responseBytes := service.HandleRequest(delivery)

				// Use twistygo's global channel to publish reply
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

				// Manually acknowledge the message after processing (since autoack is now false)
				if err := delivery.Ack(false); err != nil {
					fmt.Printf("Error acknowledging message: %v\n", err)
				}
			}(d)
		}
	}()

	// Start a simple health check server for Kubernetes
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := credStore.Ping(); err != nil {
			http.Error(w, "Database down", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	healthSrv := &http.Server{
		Addr:    ":8080",
		Handler: healthMux,
	}

	go func() {
		fmt.Println("🏥 Health check server running on :8080")
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Health check server error: %v\n", err)
		}
	}()

	fmt.Printf("Jira Service v%s is running (Multi-threaded). To exit press CTRL+C\n", ServiceVersion)
	
	// Wait for termination signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	
	fmt.Println("🛑 Shutting down Jira Service...")
	
	// Graceful shutdown for health server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	healthSrv.Shutdown(ctx)
}
