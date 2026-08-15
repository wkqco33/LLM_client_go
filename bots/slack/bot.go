// Package slack provides a Slack bot that forwards messages to an LLM backend.
// It uses the Events API with Socket Mode, which requires no public endpoint.
package slack

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/wkqco33/LLM_client_go/bots"
)

const resetCommand = "!reset"

// Bot is a Slack bot powered by an LLM backend.
type Bot struct {
	api       *slack.Client
	sm        *socketmode.Client
	backend   bots.Backend
	sessions  *bots.SessionManager
	botUserID string
	log       *log.Logger
}

// Config holds configuration for the Slack bot.
type Config struct {
	// BotToken is the Slack bot OAuth token (xoxb-...) (required).
	BotToken string

	// AppToken is the Slack app-level token for Socket Mode (xapp-...) (required).
	AppToken string

	// Backend is the LLM provider to use (required).
	Backend bots.Backend

	// Sessions is the session manager. A default one is created if nil.
	Sessions *bots.SessionManager

	// Logger defaults to the standard logger.
	Logger *log.Logger

	// Debug enables verbose Slack API logging.
	Debug bool

	// APIURL overrides the Slack API base URL (useful for testing).
	APIURL string
}

// New creates a new Slack Bot using Socket Mode.
func New(cfg Config) (*Bot, error) {
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("slack: BotToken is required")
	}
	if cfg.AppToken == "" {
		return nil, fmt.Errorf("slack: AppToken is required (Socket Mode requires xapp- token)")
	}
	if cfg.Backend == nil {
		return nil, fmt.Errorf("slack: backend is required")
	}

	sessions := cfg.Sessions
	if sessions == nil {
		sessions = bots.NewSessionManager()
	}

	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	apiOpts := []slack.Option{
		slack.OptionAppLevelToken(cfg.AppToken),
		slack.OptionDebug(cfg.Debug),
	}
	if cfg.APIURL != "" {
		apiOpts = append(apiOpts, slack.OptionAPIURL(cfg.APIURL))
	}

	api := slack.New(cfg.BotToken, apiOpts...)

	sm := socketmode.New(api,
		socketmode.OptionDebug(cfg.Debug),
	)

	// Fetch the bot's own user ID to avoid self-replies.
	authResp, err := api.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("slack: auth test: %w", err)
	}

	return &Bot{
		api:       api,
		sm:        sm,
		backend:   cfg.Backend,
		sessions:  sessions,
		botUserID: authResp.UserID,
		log:       logger,
	}, nil
}

// Start connects via Socket Mode and processes events until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	b.log.Println("[INFO] slack: bot started (Socket Mode)")

	go func() {
		<-ctx.Done()
		b.Stop()
	}()

	go b.handleEvents(ctx)
	return b.sm.Run()
}

// Stop disconnects the Socket Mode client.
func (b *Bot) Stop() error {
	b.log.Println("[INFO] slack: bot stopping")
	return nil
}

func (b *Bot) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-b.sm.Events:
			if !ok {
				return
			}
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}
				b.sm.Ack(*evt.Request)
				b.dispatchEvent(ctx, eventsAPIEvent)
			}
		}
	}
}

func (b *Bot) dispatchEvent(ctx context.Context, event slackevents.EventsAPIEvent) {
	switch event.InnerEvent.Type {
	case "app_mention":
		ev, ok := event.InnerEvent.Data.(*slackevents.AppMentionEvent)
		if !ok {
			return
		}
		// Strip the @BotMention prefix from the message text.
		text := strings.TrimSpace(stripMention(ev.Text))
		go b.handleMessage(ctx, ev.User, ev.Channel, text)

	case "message":
		ev, ok := event.InnerEvent.Data.(*slackevents.MessageEvent)
		if !ok {
			return
		}
		// Only handle direct messages (channel type "im") and ignore bot messages.
		if ev.BotID != "" || ev.User == b.botUserID {
			return
		}
		go b.handleMessage(ctx, ev.User, ev.Channel, strings.TrimSpace(ev.Text))
	}
}

func (b *Bot) handleMessage(ctx context.Context, userID, channelID, text string) {
	if text == "" {
		return
	}

	reply, _, err := bots.HandleTurn(ctx, b.sessions, b.backend, userID, text, resetCommand)
	if err != nil {
		b.log.Printf("[ERROR] slack: backend error: %v", err)
		b.send(channelID, "⚠️ Error getting response. Please try again.")
		return
	}
	b.send(channelID, reply)
}

// slackMaxMessageLen is a conservative chunk size for Slack messages. Slack
// has no small hard limit like Discord/Telegram, but very long single
// messages are still worth splitting for readability and to stay well under
// its block/text size limits.
const slackMaxMessageLen = 4000

func (b *Bot) send(channelID, text string) {
	for _, chunk := range bots.SplitMessage(text, slackMaxMessageLen) {
		if _, _, err := b.api.PostMessage(channelID, slack.MsgOptionText(chunk, false)); err != nil {
			b.log.Printf("[ERROR] slack: post message: %v", err)
		}
	}
}

// stripMention removes the leading <@USERID> mention from a Slack message.
func stripMention(text string) string {
	if strings.HasPrefix(text, "<@") {
		if idx := strings.Index(text, ">"); idx != -1 {
			return strings.TrimSpace(text[idx+1:])
		}
	}
	return text
}
