package llm

import (
	"context"
	"fmt"

	"devbot/internal/config"
)

type localClient struct {
	inner *openaiClient
	model string
}

// newLocalClient wraps the OpenAI-compatible HTTP adapter for locally-hosted models.
// Most local servers (Ollama, LM Studio, LocalAI, Jan) expose an OpenAI-compatible
// endpoint, so no separate protocol implementation is needed.
func newLocalClient(cfg *config.LocalConfig) Client {
	return &localClient{
		model: cfg.Model,
		inner: &openaiClient{
			apiKey:  cfg.APIKey, // often empty or a dummy value for local servers
			model:   cfg.Model,
			baseURL: cfg.BaseURL,
		},
	}
}

func (c *localClient) ProviderName() string {
	return fmt.Sprintf("Local (%s)", c.model)
}

func (c *localClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, error) {
	return c.inner.Complete(ctx, system, user, maxTokens)
}
