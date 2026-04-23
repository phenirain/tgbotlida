package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the bot
type Config struct {
	BotToken       string
	AdminUserID    int64
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

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	var adminUserID int64
	if s := os.Getenv("ADMIN_USER_ID"); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("ADMIN_USER_ID must be a number: %w", err)
		}
		adminUserID = id
	}

	cfg := &Config{
		BotToken:       botToken,
		AdminUserID:    adminUserID,
		WelcomeMessage: getEnvOrDefault("WELCOME_MESSAGE", "Welcome to our bot!"),
		PhotoURL:       getEnvOrDefault("PHOTO_URL", ""),
		PolicyText:     getEnvOrDefault("POLICY_TEXT", "Please accept our privacy policy to continue."),
		SecondMessage:  getEnvOrDefault("SECOND_MESSAGE", "Please provide your email address:"),
		FinalMessage:   getEnvOrDefault("FINAL_MESSAGE", "Thank you! Your email has been saved."),
		AlbumLink:      getEnvOrDefault("ALBUM_LINK", "https://example.com"),
		DatabaseURL:    databaseURL,
	}

	log.Printf("Config loaded: WELCOME_MESSAGE=%v, PHOTO_URL=%v, POLICY_TEXT=%v, SECOND_MESSAGE=%v, FINAL_MESSAGE=%v, ALBUM_LINK=%v",
		cfg.WelcomeMessage != "",
		cfg.PhotoURL != "",
		cfg.PolicyText != "",
		cfg.SecondMessage != "",
		cfg.FinalMessage != "",
		cfg.AlbumLink != "",
	)

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
