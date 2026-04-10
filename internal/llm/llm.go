// Package llm provides a unified interface for multiple AI providers.
// Supported providers: claude, openai, gemini, local.
package llm

import "context"

// Usage holds token consumption for a single API call.
// It is nil for providers that do not report usage (e.g. local servers).
type Usage struct {
	InputTokens  int64
	OutputTokens int64
}

// Client is the common interface all AI provider adapters implement.
type Client interface {
	// Complete sends a system prompt and a user message, and returns the model's
	// text response plus optional token usage (nil when unavailable).
	Complete(ctx context.Context, system, user string, maxTokens int) (text string, usage *Usage, err error)

	// ProviderName returns a human-readable label used in status messages.
	ProviderName() string
}
