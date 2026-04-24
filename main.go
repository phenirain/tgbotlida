package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/phenirain/LidaTgBot/config"
	"github.com/phenirain/LidaTgBot/database"
	"github.com/xuri/excelize/v2"
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

func (b *Bot) send(c tgbotapi.Chattable) {
	if _, err := b.api.Send(c); err != nil {
		log.Printf("send error: %v", err)
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
		b.send(msg)
		return
	}

	if exists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Вы уже отправили свой email. Спасибо!")
		b.send(msg)
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
		photoMsg.ParseMode = "html"
		photoMsg.ReplyMarkup = keyboard
		b.send(photoMsg)
	} else {
		textMsg := tgbotapi.NewMessage(update.Message.Chat.ID, b.cfg.WelcomeMessage)
		textMsg.ParseMode = "html"
		textMsg.ReplyMarkup = keyboard
		b.send(textMsg)
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
			b.send(editCaption)
		} else {
			// Edit text message to remove button
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				b.cfg.WelcomeMessage,
			)
			editMsg.ParseMode = "html"
			b.send(editMsg)
		}

		// Send policy text with "Соглашаюсь" button
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Соглашаюсь", "accept_policy"),
			),
		)
		policyMsg := tgbotapi.NewMessage(query.Message.Chat.ID, b.cfg.PolicyText)
		policyMsg.ReplyMarkup = keyboard
		policyMsg.ParseMode = "html"
		b.send(policyMsg)

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
			b.send(editMsg)
			return
		}

		if exists {
			editMsg := tgbotapi.NewEditMessageText(
				query.Message.Chat.ID,
				query.Message.MessageID,
				"Вы уже отправили свой email. Спасибо!",
			)
			b.send(editMsg)
			b.setState(userID, StateNone)
			return
		}

		// Update message to show policy accepted (remove button)
		editMsg := tgbotapi.NewEditMessageText(
			query.Message.Chat.ID,
			query.Message.MessageID,
			b.cfg.PolicyText,
		)
		editMsg.ParseMode = "html" // Enable HTML formatting
		b.send(editMsg)

		// Send second message asking for email
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, b.cfg.SecondMessage)
		msg.ParseMode = "html" // Enable HTML formatting
		b.send(msg)

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
		b.send(msg)
		b.setState(userID, StateNone)
		return
	}

	if exists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Вы уже отправили свой email. Спасибо!")
		b.send(msg)
		b.setState(userID, StateNone)
		return
	}

	// Validate email
	if !isValidEmail(email) {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Неверный формат email. Пожалуйста, укажите корректный email адрес:")
		b.send(msg)
		return
	}

	// Check if email already exists (async)
	emailExists, err := b.db.EmailExists(email)
	if err != nil {
		log.Printf("Error checking email existence: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Произошла ошибка. Пожалуйста, попробуйте позже.")
		b.send(msg)
		b.setState(userID, StateNone)
		return
	}

	if emailExists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Этот email уже зарегистрирован. Пожалуйста, используйте другой email адрес.")
		b.send(msg)
		return
	}

	// Save email and username (async)
	err = b.db.SaveUserEmail(userID, username, email)
	if err != nil {
		log.Printf("Error saving email: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Произошла ошибка. Пожалуйста, попробуйте позже.")
		b.send(msg)
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
	msg.ParseMode = "html"
	b.send(msg)
	b.setState(userID, StateNone)
}

// handleXlsx exports all users to xlsx and sends the file (admin only)
func (b *Bot) handleXlsx(update tgbotapi.Update) {
	if update.Message.From.ID != b.cfg.AdminUserID {
		return
	}

	users, err := b.db.GetAllUsers()
	if err != nil {
		log.Printf("xlsx: get users error: %v", err)
		b.send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при получении данных."))
		return
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Users"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"user_id", "username", "email", "created_at"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for row, u := range users {
		vals := []any{u.UserID, u.Username, u.Email, u.CreatedAt.Format("2006-01-02 15:04:05")}
		for col, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(col+1, row+2)
			f.SetCellValue(sheet, cell, v)
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		log.Printf("xlsx: write buffer error: %v", err)
		b.send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при создании файла."))
		return
	}

	doc := tgbotapi.NewDocument(update.Message.Chat.ID, tgbotapi.FileReader{Name: "users.xlsx", Reader: buf})
	doc.Caption = fmt.Sprintf("Всего записей: %d", len(users))
	b.send(doc)
}

// handleCancel handles the /cancel command
func (b *Bot) handleCancel(update tgbotapi.Update) {
	userID := update.Message.From.ID
	b.setState(userID, StateNone)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Операция отменена.")
	b.send(msg)
}

// Run starts the bot and handles updates
func (b *Bot) Run() {
	if _, err := b.api.Request(tgbotapi.DeleteWebhookConfig{}); err != nil {
		log.Printf("Failed to delete webhook: %v", err)
	}
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)
	log.Println("Bot started (polling mode)")

	for update := range updates {
		if update.Message != nil {
			log.Printf("update from @%s (id=%d): %q", update.Message.From.UserName, update.Message.From.ID, update.Message.Text)
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "start":
					b.handleStart(update)
				case "cancel":
					b.handleCancel(update)
				case "xlsx":
					b.handleXlsx(update)
				}
			} else {
				b.handleMessage(update)
			}
		} else if update.CallbackQuery != nil {
			log.Printf("callback from @%s (id=%d): %q", update.CallbackQuery.From.UserName, update.CallbackQuery.From.ID, update.CallbackQuery.Data)
			b.handleCallbackQuery(update)
		}
	}
}

func main() {
	logFile, err := os.OpenFile("bot.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))

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
