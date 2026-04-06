package llm

import (
	"context"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"devbot/internal/config"
)

type claudeClient struct {
	client *anthropic.Client
	model  string
}

func newClaudeClient(cfg *config.ClaudeConfig) Client {
	c := anthropic.NewClient(option.WithAPIKey(cfg.APIKey))
	return &claudeClient{client: &c, model: cfg.Model}
}

func (c *claudeClient) ProviderName() string { return "Claude" }

func (c *claudeClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: int64(maxTokens),
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
	})
	if err != nil {
		return "", nil, fmt.Errorf("claude API: %w", err)
	}
	if len(resp.Content) == 0 {
		return "", nil, fmt.Errorf("claude returned empty response")
	}
	usage := &Usage{
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}
	return resp.Content[0].Text, usage, nil
}
