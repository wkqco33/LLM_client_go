// Package discord provides a Discord bot that forwards messages to an LLM backend.
package discord

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/wkqco33/LLM_client_go/bots"
)

const resetCommand = "!reset"

// Bot is a Discord bot powered by an LLM backend.
type Bot struct {
	session  *discordgo.Session
	backend  bots.Backend
	sessions *bots.SessionManager
	log      *log.Logger

	// ctx is the context passed to Start, used so in-flight backend calls
	// are cancelled on shutdown. It's set once before the session opens
	// (and therefore before onMessage can fire), so no synchronization is
	// needed to read it from the discordgo dispatch goroutines.
	ctx context.Context
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
	b.ctx = ctx
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("discord: open connection: %w", err)
	}
	b.log.Println("[INFO] discord: bot started")

	<-ctx.Done()
	return b.Stop()
}

// Stop disconnects from Discord.
func (b *Bot) Stop() error {
	b.log.Println("[INFO] discord: bot stopping")
	return b.session.Close()
}

func (b *Bot) onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself.
	if m.Author.ID == s.State.User.ID {
		return
	}

	userID := m.Author.ID
	text := strings.TrimSpace(m.Content)

	reply, wasReset, err := bots.HandleTurn(b.ctx, b.sessions, b.backend, userID, text, resetCommand)
	if err != nil {
		b.log.Printf("[ERROR] discord: backend error: %v", err)
		s.ChannelMessageSend(m.ChannelID, "⚠️ Error getting response. Please try again.")
		return
	}
	if wasReset {
		s.ChannelMessageSend(m.ChannelID, reply)
		return
	}

	// Discord has a 2000-char message limit; split if necessary.
	for _, chunk := range bots.SplitMessage(reply, 2000) {
		if _, err := s.ChannelMessageSend(m.ChannelID, chunk); err != nil {
			b.log.Printf("[ERROR] discord: send message: %v", err)
		}
	}
}
