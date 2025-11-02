package main

import (
	"log"
	"regexp"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/phenirain/LidaTgBot/config"
	"github.com/phenirain/LidaTgBot/database"
)

// ConversationState represents the state of a user's conversation
type ConversationState int

const (
	StateNone ConversationState = iota
	StateAwaitingEmail
)

// Bot represents the Telegram bot instance
type Bot struct {
	api    *tgbotapi.BotAPI
	db     *database.Database
	cfg    *config.Config
	states map[int64]ConversationState
	mu     sync.RWMutex
}

// NewBot creates a new Bot instance
func NewBot(cfg *config.Config, db *database.Database) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, err
	}

	api.Debug = false
	log.Printf("Authorized on account %s", api.Self.UserName)

	return &Bot{
		api:    api,
		db:     db,
		cfg:    cfg,
		states: make(map[int64]ConversationState),
	}, nil
}

// getState retrieves the conversation state for a user
func (b *Bot) getState(userID int64) ConversationState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.states[userID]
}

// setState sets the conversation state for a user
func (b *Bot) setState(userID int64, state ConversationState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if state == StateNone {
		delete(b.states, userID)
	} else {
		b.states[userID] = state
	}
}

// isValidEmail validates email format
func isValidEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(pattern, email)
	return matched
}

// handleStart handles the /start command
func (b *Bot) handleStart(update tgbotapi.Update) {
	userID := update.Message.From.ID

	// Check if user already submitted email (async)
	exists, err := b.db.UserExists(userID)
	if err != nil {
		log.Printf("Error checking user existence: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "An error occurred. Please try again later.")
		b.api.Send(msg)
		return
	}

	if exists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You have already submitted your email. Thank you!")
		b.api.Send(msg)
		return
	}

	// Send welcome message
	welcomeMsg := tgbotapi.NewMessage(update.Message.Chat.ID, b.cfg.WelcomeMessage)
	b.api.Send(welcomeMsg)

	// Show policy acceptance button
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Accept", "accept_policy"),
		),
	)

	policyMsg := tgbotapi.NewMessage(update.Message.Chat.ID, b.cfg.PolicyText)
	policyMsg.ReplyMarkup = keyboard
	b.api.Send(policyMsg)

	b.setState(userID, StateAwaitingEmail)
}

// handleCallbackQuery handles callback queries (button presses)
func (b *Bot) handleCallbackQuery(update tgbotapi.Update) {
	query := update.CallbackQuery
	userID := query.From.ID

	// Answer the callback query to remove loading state
	callback := tgbotapi.NewCallback(query.ID, "")
	b.api.Request(callback)

	if query.Data == "accept_policy" {
		// Check if user already submitted (async)
		exists, err := b.db.UserExists(userID)
		if err != nil {
			log.Printf("Error checking user existence: %v", err)
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				"An error occurred. Please try again later.",
			)
			b.api.Send(editMsg)
			return
		}

		if exists {
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				"You have already submitted your email. Thank you!",
			)
			b.api.Send(editMsg)
			b.setState(userID, StateNone)
			return
		}

		// Update message to show policy accepted
		editMsg := tgbotapi.NewEditMessageText(
			query.Message.Chat.ID,
			query.Message.MessageID,
			"Policy accepted!",
		)
		b.api.Send(editMsg)

		// Ask for email
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, "Please provide your email address:")
		b.api.Send(msg)

		b.setState(userID, StateAwaitingEmail)
	}
}

// handleMessage handles regular text messages
func (b *Bot) handleMessage(update tgbotapi.Update) {
	userID := update.Message.From.ID
	state := b.getState(userID)

	if state == StateAwaitingEmail {
		b.handleEmailSubmission(update)
	}
}

// handleEmailSubmission handles email submission
func (b *Bot) handleEmailSubmission(update tgbotapi.Update) {
	userID := update.Message.From.ID
	email := update.Message.Text

	// Check if user already submitted (async)
	exists, err := b.db.UserExists(userID)
	if err != nil {
		log.Printf("Error checking user existence: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "An error occurred. Please try again later.")
		b.api.Send(msg)
		b.setState(userID, StateNone)
		return
	}

	if exists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You have already submitted your email. Thank you!")
		b.api.Send(msg)
		b.setState(userID, StateNone)
		return
	}

	// Validate email
	if !isValidEmail(email) {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid email format. Please provide a valid email address:")
		b.api.Send(msg)
		return
	}

	// Check if email already exists (async)
	emailExists, err := b.db.EmailExists(email)
	if err != nil {
		log.Printf("Error checking email existence: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "An error occurred. Please try again later.")
		b.api.Send(msg)
		b.setState(userID, StateNone)
		return
	}

	if emailExists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "This email is already registered. Please use a different email address.")
		b.api.Send(msg)
		return
	}

	// Save email (async)
	err = b.db.SaveUserEmail(userID, email)
	if err != nil {
		log.Printf("Error saving email: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "An error occurred. Please try again later.")
		b.api.Send(msg)
		b.setState(userID, StateNone)
		return
	}

	// Send final message
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, b.cfg.FinalMessage)
	b.api.Send(msg)
	b.setState(userID, StateNone)
}

// handleCancel handles the /cancel command
func (b *Bot) handleCancel(update tgbotapi.Update) {
	userID := update.Message.From.ID
	b.setState(userID, StateNone)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Operation cancelled.")
	b.api.Send(msg)
}

// Run starts the bot and handles updates
func (b *Bot) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	log.Println("Bot started")

	for update := range updates {
		// Handle different update types
		if update.Message != nil {
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "start":
					b.handleStart(update)
				case "cancel":
					b.handleCancel(update)
				}
			} else {
				b.handleMessage(update)
			}
		} else if update.CallbackQuery != nil {
			b.handleCallbackQuery(update)
		}
	}
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database with async operations
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create and start bot
	bot, err := NewBot(cfg, db)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	bot.Run()
}
