package oauth

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds OAuth server settings.
type Config struct {
	Issuer              string
	Audience            string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	AuthCodeTTL         time.Duration
	DCRMode             string
	DCRAccessToken      string
	ClerkPublishableKey string
	ClerkJSURL          string
}

// LoadConfigFromEnv loads OAuth config from environment variables.
func LoadConfigFromEnv() (Config, error) {
	issuer := strings.TrimSpace(os.Getenv("OAUTH_ISSUER"))
	if issuer == "" {
		return Config{}, fmt.Errorf("OAUTH_ISSUER is required")
	}

	audience := strings.TrimSpace(os.Getenv("OAUTH_AUDIENCE"))
	if audience == "" {
		return Config{}, fmt.Errorf("OAUTH_AUDIENCE is required")
	}

	accessTTL := parseDurationEnv("OAUTH_ACCESS_TOKEN_TTL", 60*time.Minute)
	refreshTTL := parseDurationEnv("OAUTH_REFRESH_TOKEN_TTL", 30*24*time.Hour)
	codeTTL := parseDurationEnv("OAUTH_AUTH_CODE_TTL", 10*time.Minute)

	dcrMode := strings.ToLower(strings.TrimSpace(os.Getenv("OAUTH_DCR_MODE")))
	if dcrMode == "" {
		dcrMode = "protected"
	}

	clerkPublishableKey := strings.TrimSpace(os.Getenv("CLERK_PUBLISHABLE_KEY"))
	clerkJSURL := strings.TrimSpace(os.Getenv("CLERK_JS_URL"))
	if clerkJSURL == "" {
		clerkJSURL = "https://cdn.jsdelivr.net/npm/@clerk/clerk-js@latest/dist/clerk.browser.js"
	}

	return Config{
		Issuer:              strings.TrimRight(issuer, "/"),
		Audience:            audience,
		AccessTokenTTL:      accessTTL,
		RefreshTokenTTL:     refreshTTL,
		AuthCodeTTL:         codeTTL,
		DCRMode:             dcrMode,
		DCRAccessToken:      os.Getenv("OAUTH_DCR_ACCESS_TOKEN"),
		ClerkPublishableKey: clerkPublishableKey,
		ClerkJSURL:          clerkJSURL,
	}, nil
}

func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if dur, err := time.ParseDuration(val); err == nil {
			return dur
		}
	}
	return fallback
}
