package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the bot
type Config struct {
	BotToken       string
	WebhookURL     string
	WelcomeMessage string
	PhotoURL       string
	PolicyText     string
	SecondMessage  string
	FinalMessage   string
	AlbumLink      string
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
		WebhookURL:     os.Getenv("WEBHOOK_URL"),
		WelcomeMessage: getEnvOrDefault("WELCOME_MESSAGE", "Welcome to our bot!"),
		PhotoURL:       getEnvOrDefault("PHOTO_URL", ""),
		PolicyText:     getEnvOrDefault("POLICY_TEXT", "Please accept our privacy policy to continue."),
		SecondMessage:  getEnvOrDefault("SECOND_MESSAGE", "Please provide your email address:"),
		FinalMessage:   getEnvOrDefault("FINAL_MESSAGE", "Thank you! Your email has been saved."),
		AlbumLink:      getEnvOrDefault("ALBUM_LINK", "https://example.com"),
		DatabaseURL:    databaseURL,
	}

	return cfg, nil
}

// getEnvOrDefault returns the environment variable value or a default value
// It also processes escape sequences like \n for multi-line strings
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return processEscapeSequences(value)
	}
	return defaultValue
}

// processEscapeSequences converts escape sequences like \n to actual newlines
func processEscapeSequences(s string) string {
	// Replace \n with actual newline
	// This handles both literal \n strings and actual newlines
	result := strings.ReplaceAll(s, "\\n", "\n")

	// Also handle other common escape sequences
	result = strings.ReplaceAll(result, "\\t", "\t")
	result = strings.ReplaceAll(result, "\\r", "\r")

	// Try to unquote the string if it's quoted (handles cases where shell might quote it)
	if len(result) >= 2 && result[0] == '"' && result[len(result)-1] == '"' {
		if unquoted, err := strconv.Unquote(result); err == nil {
			result = unquoted
		}
	}

	return result
}
