// Package discord provides a Discord bot that forwards messages to an LLM backend.
package discord

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	llm "llm-client-go"
	"llm-client-go/bots"
)

const resetCommand = "!reset"

// Bot is a Discord bot powered by an LLM backend.
type Bot struct {
	session  *discordgo.Session
	backend  bots.Backend
	sessions *bots.SessionManager
	log      *log.Logger
}

// Config holds configuration for the Discord bot.
type Config struct {
	// Token is the Discord bot token (required).
	Token string

	// Backend is the LLM provider to use (required).
	Backend bots.Backend

	// Sessions is the session manager. A default one is created if nil.
	Sessions *bots.SessionManager

	// Logger is used for info/error messages. Defaults to the standard logger.
	Logger *log.Logger
}

// New creates a new Discord Bot.
func New(cfg Config) (*Bot, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("discord: token is required")
	}
	if cfg.Backend == nil {
		return nil, fmt.Errorf("discord: backend is required")
	}

	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("discord: create session: %w", err)
	}

	sessions := cfg.Sessions
	if sessions == nil {
		sessions = bots.NewSessionManager()
	}

	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	b := &Bot{
		session:  dg,
		backend:  cfg.Backend,
		sessions: sessions,
		log:      logger,
	}

	dg.AddHandler(b.onMessage)
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	return b, nil
}

// Start connects to Discord and begins processing messages.
// It blocks until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("discord: open connection: %w", err)
	}
	b.log.Println("Discord bot started")

	<-ctx.Done()
	return b.Stop()
}

// Stop disconnects from Discord.
func (b *Bot) Stop() error {
	b.log.Println("Discord bot stopping")
	return b.session.Close()
}

func (b *Bot) onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself.
	if m.Author.ID == s.State.User.ID {
		return
	}

	userID := m.Author.ID
	text := strings.TrimSpace(m.Content)

	if strings.EqualFold(text, resetCommand) {
		b.sessions.Reset(userID)
		s.ChannelMessageSend(m.ChannelID, "✅ Conversation reset.")
		return
	}

	// Append the user message and fetch full history.
	b.sessions.Append(userID, llm.Message{Role: llm.RoleUser, Content: text})
	history := b.sessions.GetHistory(userID)

	reply, err := b.backend.Complete(context.Background(), history)
	if err != nil {
		b.log.Printf("discord: backend error: %v", err)
		s.ChannelMessageSend(m.ChannelID, "⚠️ Error getting response. Please try again.")
		return
	}

	// Append the assistant reply to the session.
	b.sessions.Append(userID, llm.Message{Role: llm.RoleAssistant, Content: reply})

	// Discord has a 2000-char message limit; split if necessary.
	for _, chunk := range splitMessage(reply, 2000) {
		if _, err := s.ChannelMessageSend(m.ChannelID, chunk); err != nil {
			b.log.Printf("discord: send message: %v", err)
		}
	}
}

// splitMessage splits a long message into chunks of at most maxLen characters.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > maxLen {
		chunks = append(chunks, text[:maxLen])
		text = text[maxLen:]
	}
	if text != "" {
		chunks = append(chunks, text)
	}
	return chunks
}
