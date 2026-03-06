// Package bots provides common types and interfaces for LLM-powered chat bots.
package bots

import "context"

// Bot is implemented by each platform adapter (Discord, Telegram, Slack).
type Bot interface {
	// Start begins listening for messages. It blocks until ctx is cancelled or a fatal error occurs.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the bot.
	Stop() error
}
