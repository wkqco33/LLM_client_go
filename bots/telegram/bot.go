// Package telegram provides a Telegram bot that forwards messages to an LLM backend.
package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/wkqco33/LLM_client_go/bots"
)

const resetCommand = "/reset"

// Bot is a Telegram bot powered by an LLM backend.
type Bot struct {
	api      *tgbotapi.BotAPI
	backend  bots.Backend
	sessions *bots.SessionManager
	log      *log.Logger
}

// Config holds configuration for the Telegram bot.
type Config struct {
	// Token is the Telegram bot token from BotFather (required).
	Token string

	// Backend is the LLM provider to use (required).
	Backend bots.Backend

	// Sessions is the session manager. A default one is created if nil.
	Sessions *bots.SessionManager

	// Logger defaults to the standard logger.
	Logger *log.Logger

	// Debug enables verbose Telegram API logging.
	Debug bool
}

// New creates a new Telegram Bot.
func New(cfg Config) (*Bot, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram: token is required")
	}
	if cfg.Backend == nil {
		return nil, fmt.Errorf("telegram: backend is required")
	}

	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("telegram: create bot api: %w", err)
	}
	api.Debug = cfg.Debug

	sessions := cfg.Sessions
	if sessions == nil {
		sessions = bots.NewSessionManager()
	}

	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	return &Bot{
		api:      api,
		backend:  cfg.Backend,
		sessions: sessions,
		log:      logger,
	}, nil
}

// Start begins polling for updates and blocks until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	b.log.Printf("[INFO] telegram: bot started (@%s)", b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return b.Stop()
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if update.Message != nil {
				go b.handleMessage(ctx, update.Message)
			}
		}
	}
}

// Stop shuts down the bot.
func (b *Bot) Stop() error {
	b.log.Println("[INFO] telegram: bot stopping")
	b.api.StopReceivingUpdates()
	return nil
}

func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	userID := fmt.Sprintf("%d", msg.From.ID)
	text := strings.TrimSpace(msg.Text)

	if text == "" {
		return
	}

	// reply already holds the right text whether this was a reset or a
	// normal turn; only the error path needs distinct handling.
	reply, _, err := bots.HandleTurn(ctx, b.sessions, b.backend, userID, text, resetCommand)
	if err != nil {
		b.log.Printf("[ERROR] telegram: backend error: %v", err)
		b.send(msg.Chat.ID, "⚠️ Error getting response. Please try again.")
		return
	}
	b.send(msg.Chat.ID, reply)
}

// Telegram message limit is 4096 characters.
const telegramMaxMessageLen = 4096

func (b *Bot) send(chatID int64, text string) {
	for _, chunk := range bots.SplitMessage(text, telegramMaxMessageLen) {
		msg := tgbotapi.NewMessage(chatID, chunk)
		msg.ParseMode = tgbotapi.ModeMarkdown
		if _, err := b.api.Send(msg); err != nil {
			b.log.Printf("[ERROR] telegram: send message: %v", err)
		}
	}
}
