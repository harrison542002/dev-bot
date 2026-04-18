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

func NewLocal(cfg *config.LocalConfig) (Client, error) {
	return &localClient{
		model: cfg.Model,
		inner: &openaiClient{
			apiKey:  cfg.APIKey,
			model:   cfg.Model,
			baseURL: cfg.BaseURL,
		},
	}, nil
}

func (c *localClient) ProviderName() string {
	return fmt.Sprintf("Local (%s)", c.model)
}

func (c *localClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	return c.inner.Complete(ctx, system, user, maxTokens)
}

func (c *localClient) CompleteWithTools(ctx context.Context, system string, messages []Message, tools []Tool, maxTokens int) (Message, *Usage, error) {
	return c.inner.CompleteWithTools(ctx, system, messages, tools, maxTokens)
}
