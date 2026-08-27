package retry_test

import (
	"fmt"
	"time"

	"github.com/wkqco33/LLM_client_go/retry"
)

func ExamplePolicy() {
	policy := retry.Policy{
		MaxRetries: 3,
		MinWait:    500 * time.Millisecond,
		MaxWait:    5 * time.Second,
	}
	fmt.Printf("MaxRetries: %d\n", policy.MaxRetries)
	// Output:
	// MaxRetries: 3
}
