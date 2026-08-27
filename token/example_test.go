package token_test

import (
	"fmt"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/token"
)

func ExampleEstimate() {
	tokens := token.Estimate("Hello world")
	fmt.Printf("Estimated tokens > 0: %t\n", tokens > 0)
	// Output:
	// Estimated tokens > 0: true
}

func ExampleHeuristicCounter_CountMessages() {
	counter := token.HeuristicCounter{}
	messages := []llm.Message{
		llm.NewSystemMessage("You are a helpful assistant."),
		llm.NewUserMessage("What is Go?"),
	}
	total := counter.CountMessages(messages)
	fmt.Printf("Total message tokens > 0: %t\n", total > 0)
	// Output:
	// Total message tokens > 0: true
}
