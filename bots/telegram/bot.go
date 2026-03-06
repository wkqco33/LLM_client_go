// Package telegram provides a Telegram bot that forwards messages to an LLM backend.
package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	llm "llm-client-go"
	"llm-client-go/bots"
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
	b.log.Printf("Telegram bot started (@%s)", b.api.Self.UserName)

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
				go b.handleMessage(update.Message)
			}
		}
	}
}

// Stop shuts down the bot.
func (b *Bot) Stop() error {
	b.log.Println("Telegram bot stopping")
	b.api.StopReceivingUpdates()
	return nil
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	userID := fmt.Sprintf("%d", msg.From.ID)
	text := strings.TrimSpace(msg.Text)

	if text == "" {
		return
	}

	if strings.EqualFold(text, resetCommand) {
		b.sessions.Reset(userID)
		b.send(msg.Chat.ID, "✅ Conversation reset.")
		return
	}

	b.sessions.Append(userID, llm.Message{Role: llm.RoleUser, Content: text})
	history := b.sessions.GetHistory(userID)

	reply, err := b.backend.Complete(context.Background(), history)
	if err != nil {
		b.log.Printf("telegram: backend error: %v", err)
		b.send(msg.Chat.ID, "⚠️ Error getting response. Please try again.")
		return
	}

	b.sessions.Append(userID, llm.Message{Role: llm.RoleAssistant, Content: reply})
	b.send(msg.Chat.ID, reply)
}

func (b *Bot) send(chatID int64, text string) {
	// Telegram message limit is 4096 characters.
	const maxLen = 4096
	for len(text) > 0 {
		chunk := text
		if len(chunk) > maxLen {
			chunk = text[:maxLen]
		}
		text = text[len(chunk):]

		msg := tgbotapi.NewMessage(chatID, chunk)
		msg.ParseMode = tgbotapi.ModeMarkdown
		if _, err := b.api.Send(msg); err != nil {
			b.log.Printf("telegram: send message: %v", err)
		}
	}
}
