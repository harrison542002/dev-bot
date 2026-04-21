package llm

import (
	"context"
	"encoding/json"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/harrison542002/dev-bot/internal/config"
)

type ClaudeClient struct {
	client *anthropic.Client
	model  string
}

func NewClaudeClient(cfg *config.ClaudeConfig) Client {
	c := anthropic.NewClient(option.WithAPIKey(cfg.APIKey))
	return &ClaudeClient{client: &c, model: cfg.Model}
}

func (c *ClaudeClient) ProviderName() string { return "Claude" }

func (c *ClaudeClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
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

func (c *ClaudeClient) CompleteWithTools(ctx context.Context, system string, messages []Message, tools []Tool, maxTokens int) (Message, *Usage, error) {
	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: int64(maxTokens),
		System:    []anthropic.TextBlockParam{{Text: system}},
		Tools:     claudeToolParams(tools),
		Messages:  claudeMessageParams(messages),
	})
	if err != nil {
		return Message{}, nil, fmt.Errorf("claude API: %w", err)
	}

	reply := Message{Role: "assistant"}
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			reply.Text = block.AsText().Text
		case "tool_use":
			tu := block.AsToolUse()
			var input map[string]any
			if err := json.Unmarshal(tu.Input, &input); err != nil {
				return Message{}, nil, fmt.Errorf("parse tool input for %q: %w", tu.Name, err)
			}
			reply.ToolUses = append(reply.ToolUses, ToolUse{
				ID:    tu.ID,
				Name:  tu.Name,
				Input: input,
			})
		}
	}

	usage := &Usage{
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}
	return reply, usage, nil
}

func claudeToolParams(tools []Tool) []anthropic.ToolUnionParam {
	params := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		props := make(map[string]interface{}, len(t.Parameters.Properties))
		for name, p := range t.Parameters.Properties {
			props[name] = map[string]interface{}{
				"type":        p.Type,
				"description": p.Description,
			}
		}
		params = append(params, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: props,
					Required:   t.Parameters.Required,
				},
			},
		})
	}
	return params
}

func claudeMessageParams(messages []Message) []anthropic.MessageParam {
	params := make([]anthropic.MessageParam, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case "user":
			if len(m.ToolResults) > 0 {
				blocks := make([]anthropic.ContentBlockParamUnion, 0, len(m.ToolResults))
				for _, tr := range m.ToolResults {
					blocks = append(blocks, anthropic.NewToolResultBlock(tr.ToolUseID, tr.Content, tr.IsError))
				}
				params = append(params, anthropic.NewUserMessage(blocks...))
			} else {
				params = append(params, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Text)))
			}
		case "assistant":
			blocks := make([]anthropic.ContentBlockParamUnion, 0)
			if m.Text != "" {
				blocks = append(blocks, anthropic.NewTextBlock(m.Text))
			}
			for _, tu := range m.ToolUses {
				blocks = append(blocks, anthropic.NewToolUseBlock(tu.ID, tu.Input, tu.Name))
			}
			params = append(params, anthropic.NewAssistantMessage(blocks...))
		}
	}
	return params
}
