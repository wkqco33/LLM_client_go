package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wkqco33/LLM_client_go/bots"
	slackbot "github.com/wkqco33/LLM_client_go/bots/slack"
	"github.com/wkqco33/LLM_client_go/examples/internal/dotenv"
)

func main() {
	if err := dotenv.Load(); err != nil {
		log.Fatalf("failed to load .env: %v", err)
	}

	// Configure backend: set BACKEND=openai or BACKEND=azure to use a hosted
	// provider. Defaults to Ollama (local, no API key required) so this
	// example works out of the box.
	var backend bots.Backend
	switch os.Getenv("BACKEND") {
	case "azure":
		backend = bots.NewAzureBackend(
			os.Getenv("AZURE_OPENAI_ENDPOINT"),
			os.Getenv("AZURE_OPENAI_API_KEY"),
			os.Getenv("AZURE_OPENAI_DEPLOYMENT"),
		)
	case "openai":
		backend = bots.NewOpenAIBackend(
			os.Getenv("OPENAI_API_KEY"),
			getEnvOrDefault("OPENAI_MODEL", "gpt-4o"),
		)
	default:
		backend = bots.NewOllamaBackend(
			os.Getenv("OLLAMA_BASE_URL"),
			getEnvOrDefault("OLLAMA_MODEL", "llama3.2"),
		)
	}

	sessions := bots.NewSessionManager(
		bots.WithSystemPrompt("You are a helpful assistant."),
		bots.WithMaxHistory(20),
	)

	bot, err := slackbot.New(slackbot.Config{
		BotToken: os.Getenv("SLACK_BOT_TOKEN"), // xoxb-...
		AppToken: os.Getenv("SLACK_APP_TOKEN"), // xapp-... (Socket Mode)
		Backend:  backend,
		Sessions: sessions,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Println("Starting Slack bot... (Ctrl+C to stop)")
	if err := bot.Start(ctx); err != nil {
		log.Fatal(err)
	}
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
