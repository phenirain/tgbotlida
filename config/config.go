
package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the bot
type Config struct {
	BotToken       string
	WelcomeMessage string
	PolicyText     string
	FinalMessage   string
	DatabaseURL    string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		return nil, fmt.Errorf("BOT_TOKEN environment variable is required")
	}

	// Get database URL (required for PostgreSQL)
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	cfg := &Config{
		BotToken:       botToken,
		WelcomeMessage: getEnvOrDefault("WELCOME_MESSAGE", "Welcome to our bot!"),
		PolicyText:     getEnvOrDefault("POLICY_TEXT", "Please accept our privacy policy to continue."),
		FinalMessage:   getEnvOrDefault("FINAL_MESSAGE", "Thank you! Your email has been saved."),
		DatabaseURL:    databaseURL,
	}

	return cfg, nil
}

// getEnvOrDefault returns the environment variable value or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
