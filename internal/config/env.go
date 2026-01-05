package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/joho/godotenv"
)

// LoadEnv pulls secrets from AWS Secrets Manager (if configured) and then loads
// local .env files. This lets containers source secrets securely while still
// supporting local development.
func LoadEnv(defaultEnvPath string) {
	if err := loadAWSSecretsIntoEnv(); err != nil {
		fmt.Printf("⚠️  Skipping AWS Secrets Manager load: %v\n", err)
	}
	loadDotEnv(defaultEnvPath)
}

func loadDotEnv(defaultEnvPath string) {
	envFile := os.Getenv("ENV_FILE_PATH")
	if envFile == "" {
		envFile = defaultEnvPath
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

func loadAWSSecretsIntoEnv() error {
	secretID := os.Getenv("AWS_SECRETS_MANAGER_SECRET_ID")
	if secretID == "" {
		secretID = os.Getenv("AWS_SECRET_ID")
	}
	if secretID == "" {
		fmt.Println("ℹ️ AWS Secrets Manager: no secret ID provided, skipping fetch")
		return nil
	}

	region := os.Getenv("AWS_SECRETS_MANAGER_REGION")
	versionStage := os.Getenv("AWS_SECRETS_MANAGER_VERSION_STAGE")
	if versionStage == "" {
		versionStage = "AWSCURRENT"
	}
	overwrite := strings.EqualFold(os.Getenv("AWS_SECRETS_MANAGER_OVERWRITE"), "true")

	ctx := context.Background()
	cfg, err := loadAWSConfig(ctx, region)
	if err != nil {
		return err
	}

	client := secretsmanager.NewFromConfig(cfg)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretID),
	}
	if versionStage != "" {
		input.VersionStage = aws.String(versionStage)
	}

	output, err := client.GetSecretValue(ctx, input)
	if err != nil {
		fmt.Printf("⚠️ AWS Secrets Manager: failed to fetch %s: %v\n", secretID, err)
		return fmt.Errorf("fetching secret %s: %w", secretID, err)
	}

	payload := ""
	switch {
	case output.SecretString != nil:
		payload = *output.SecretString
	case len(output.SecretBinary) > 0:
		payload = string(output.SecretBinary)
	default:
		return fmt.Errorf("secret %s has no payload", secretID)
	}

	var kv map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &kv); err != nil {
		fmt.Printf("⚠️ AWS Secrets Manager: secret %s is not valid JSON: %v\n", secretID, err)
		return fmt.Errorf("parsing secret %s as JSON: %w", secretID, err)
	}

	applied := 0
	for key, val := range kv {
		value := fmt.Sprint(val)
		if !overwrite && os.Getenv(key) != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("setting env %s from secret: %w", key, err)
		}
		applied++
	}

	if applied > 0 {
		fmt.Printf("ℹ️ Loaded %d env vars from AWS Secrets Manager secret %s\n", applied, secretID)
	} else {
		fmt.Printf("ℹ️ AWS Secrets Manager: no env vars applied from secret %s (overwrite=%v)\n", secretID, overwrite)
	}

	return nil
}

func loadAWSConfig(ctx context.Context, region string) (aws.Config, error) {
	if region != "" {
		return awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	}
	return awsconfig.LoadDefaultConfig(ctx)
}
