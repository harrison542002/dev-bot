// Package llm provides a unified interface for multiple AI providers.
// Supported providers: claude (Anthropic), openai (OpenAI / any OpenAI-compatible endpoint), gemini (Google).
package llm

import "context"

// Client is the common interface all AI provider adapters implement.
type Client interface {
	// Complete sends a system prompt and a user message, and returns the model's text response.
	Complete(ctx context.Context, system, user string, maxTokens int) (string, error)
	// ProviderName returns a human-readable label used in status messages (e.g. "Claude", "OpenAI").
	ProviderName() string
}
