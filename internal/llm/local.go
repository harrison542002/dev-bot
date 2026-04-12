package llm

import (
	"context"
	"fmt"

	"github.com/harrison542002/dev-bot/internal/config"
)

type localClient struct {
	inner *openaiClient
	model string
}

// NewLocal wraps the OpenAI-compatible HTTP adapter for locally-hosted models.
// Most local servers (Ollama, LM Studio, LocalAI, Jan) expose an OpenAI-compatible
// endpoint, so no separate protocol implementation is needed.
func NewLocal(cfg *config.LocalConfig) (Client, error) {
	return &localClient{
		model: cfg.Model,
		inner: &openaiClient{
			apiKey:  cfg.APIKey, // often empty or a dummy value for local servers
			model:   cfg.Model,
			baseURL: cfg.BaseURL,
		},
	}, nil
}

func (c *localClient) ProviderName() string {
	return fmt.Sprintf("Local (%s)", c.model)
}

// Complete delegates to the OpenAI-compatible adapter.
// Usage is returned when the local server reports token counts; otherwise nil.
func (c *localClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	return c.inner.Complete(ctx, system, user, maxTokens)
}

// CompleteWithTools delegates to the OpenAI-compatible adapter.
// Tool use works with any local server that supports the OpenAI function-calling API.
func (c *localClient) CompleteWithTools(ctx context.Context, system string, messages []Message, tools []Tool, maxTokens int) (Message, *Usage, error) {
	return c.inner.CompleteWithTools(ctx, system, messages, tools, maxTokens)
}
