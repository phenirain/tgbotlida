package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/phenirain/LidaTgBot/config"
	"github.com/phenirain/LidaTgBot/database"
	"golang.org/x/net/proxy"
)

// ConversationState represents the state of a user's conversation
type ConversationState int

const (
	StateNone ConversationState = iota
	StateAwaitingListenConfirm
	StateAwaitingPolicyAccept
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
	httpClient := &http.Client{}

	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid PROXY_URL: %w", err)
		}
		dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy dialer: %w", err)
		}
		httpClient.Transport = &http.Transport{
			Dial: dialer.Dial,
		}
	}

	api, err := tgbotapi.NewBotAPIWithClient(cfg.BotToken, tgbotapi.APIEndpoint, httpClient)
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
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Произошла ошибка. Пожалуйста, попробуйте позже.")
		b.api.Send(msg)
		return
	}

	if exists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Вы уже отправили свой email. Спасибо!")
		b.api.Send(msg)
		b.setState(userID, StateNone) // Reset state
		return
	}

	// Create "Слушаю" button
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Слушаю", "listen_confirm"),
		),
	)

	// Send photo with welcome message and "Слушаю" button
	if b.cfg.PhotoURL != "" {
		var photoMsg tgbotapi.PhotoConfig

		// Check if PhotoURL is a local file path or URL
		if strings.HasPrefix(b.cfg.PhotoURL, "http://") || strings.HasPrefix(b.cfg.PhotoURL, "https://") {
			// URL
			photoMsg = tgbotapi.NewPhoto(update.Message.Chat.ID, tgbotapi.FileURL(b.cfg.PhotoURL))
		} else {
			// Local file path
			photoMsg = tgbotapi.NewPhoto(update.Message.Chat.ID, tgbotapi.FilePath(b.cfg.PhotoURL))
		}

		photoMsg.Caption = b.cfg.WelcomeMessage
		photoMsg.ReplyMarkup = keyboard
		b.api.Send(photoMsg)
	} else {
		// If no photo URL, send as text message
		textMsg := tgbotapi.NewMessage(update.Message.Chat.ID, b.cfg.WelcomeMessage)
		textMsg.ReplyMarkup = keyboard
		b.api.Send(textMsg)
	}

	b.setState(userID, StateAwaitingListenConfirm)
}

// handleCallbackQuery handles callback queries (button presses)
func (b *Bot) handleCallbackQuery(update tgbotapi.Update) {
	query := update.CallbackQuery
	userID := query.From.ID

	// Answer the callback query to remove loading state
	callback := tgbotapi.NewCallback(query.ID, "")
	b.api.Request(callback)

	switch query.Data {
	case "listen_confirm":
		// User clicked "Слушаю" button
		// Remove "Слушаю" button from previous message
		if query.Message.Photo != nil {
			// Edit photo caption to remove button
			editCaption := tgbotapi.NewEditMessageCaption(
				query.Message.Chat.ID,
				query.Message.MessageID,
				b.cfg.WelcomeMessage,
			)
			b.api.Send(editCaption)
		} else {
			// Edit text message to remove button
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				b.cfg.WelcomeMessage,
			)
			b.api.Send(editMsg)
		}

		// Send policy text with "Соглашаюсь" button
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Соглашаюсь", "accept_policy"),
			),
		)
		policyMsg := tgbotapi.NewMessage(query.Message.Chat.ID, b.cfg.PolicyText)
		policyMsg.ReplyMarkup = keyboard
		policyMsg.ParseMode = "markdown"
		b.api.Send(policyMsg)

		b.setState(userID, StateAwaitingPolicyAccept)

	case "accept_policy":
		// User clicked "Соглашаюсь" button
		// Check if user already submitted (async)
		exists, err := b.db.UserExists(userID)
		if err != nil {
			log.Printf("Error checking user existence: %v", err)
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				"Произошла ошибка. Пожалуйста, попробуйте позже.",
			)
			b.api.Send(editMsg)
			return
		}

		if exists {
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				"Вы уже отправили свой email. Спасибо!",
			)
			b.api.Send(editMsg)
			b.setState(userID, StateNone)
			return
		}

		// Update message to show policy accepted (remove button)
		editMsg := tgbotapi.NewEditMessageText(
			query.Message.Chat.ID,
			query.Message.MessageID,
			b.cfg.PolicyText,
		)
		editMsg.ParseMode = "markdown" // Enable HTML formatting
		b.api.Send(editMsg)

		// Send second message asking for email
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, b.cfg.SecondMessage)
		msg.ParseMode = "markdown" // Enable HTML formatting
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
	username := update.Message.From.UserName

	// Check if user already submitted (async)
	exists, err := b.db.UserExists(userID)
	if err != nil {
		log.Printf("Error checking user existence: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Произошла ошибка. Пожалуйста, попробуйте позже.")
		b.api.Send(msg)
		b.setState(userID, StateNone)
		return
	}

	if exists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Вы уже отправили свой email. Спасибо!")
		b.api.Send(msg)
		b.setState(userID, StateNone)
		return
	}

	// Validate email
	if !isValidEmail(email) {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Неверный формат email. Пожалуйста, укажите корректный email адрес:")
		b.api.Send(msg)
		return
	}

	// Check if email already exists (async)
	emailExists, err := b.db.EmailExists(email)
	if err != nil {
		log.Printf("Error checking email existence: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Произошла ошибка. Пожалуйста, попробуйте позже.")
		b.api.Send(msg)
		b.setState(userID, StateNone)
		return
	}

	if emailExists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Этот email уже зарегистрирован. Пожалуйста, используйте другой email адрес.")
		b.api.Send(msg)
		return
	}

	// Save email and username (async)
	err = b.db.SaveUserEmail(userID, username, email)
	if err != nil {
		log.Printf("Error saving email: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Произошла ошибка. Пожалуйста, попробуйте позже.")
		b.api.Send(msg)
		b.setState(userID, StateNone)
		return
	}

	// Send final message with album link button
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Слушать альбом", b.cfg.AlbumLink),
		),
	)

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, b.cfg.FinalMessage)
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
	b.setState(userID, StateNone)
}

// handleCancel handles the /cancel command
func (b *Bot) handleCancel(update tgbotapi.Update) {
	userID := update.Message.From.ID
	b.setState(userID, StateNone)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Операция отменена.")
	b.api.Send(msg)
}

// Run starts the bot and handles updates
func (b *Bot) Run() {
	var updates tgbotapi.UpdatesChannel

	if b.cfg.WebhookURL != "" {
		wh, err := tgbotapi.NewWebhook(b.cfg.WebhookURL + "/" + b.cfg.BotToken)
		if err != nil {
			log.Fatalf("Failed to create webhook config: %v", err)
		}
		if _, err := b.api.Request(wh); err != nil {
			log.Fatalf("Failed to set webhook: %v", err)
		}
		updates = b.api.ListenForWebhook("/" + b.cfg.BotToken)
		go func() {
			log.Println("Bot started (webhook mode)")
			if err := http.ListenAndServe(":8080", nil); err != nil {
				log.Fatalf("HTTP server error: %v", err)
			}
		}()
	} else {
		if _, err := b.api.Request(tgbotapi.DeleteWebhookConfig{}); err != nil {
			log.Printf("Failed to delete webhook: %v", err)
		}
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates = b.api.GetUpdatesChan(u)
		log.Println("Bot started (polling mode)")
	}

	for update := range updates {
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
