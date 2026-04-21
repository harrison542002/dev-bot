package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/harrison542002/dev-bot/internal/config"
)

type OpenAIClient struct {
	apiKey  string
	model   string
	baseURL string
}

func NewOpenAIClient(cfg *config.OpenAIConfig) Client {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return &OpenAIClient{apiKey: cfg.APIKey, model: cfg.Model, baseURL: base}
}

func (c *OpenAIClient) ProviderName() string { return "OpenAI" }

type OpenAIRequest struct {
	Model     string          `json:"model"`
	Messages  []OpenAIMessage `json:"messages"`
	MaxTokens int             `json:"max_completion_tokens"`
}

type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content   *string          `json:"content"`
			ToolCalls []OpenAIToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (c *OpenAIClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	reqBody := OpenAIRequest{
		Model: c.model,
		Messages: []OpenAIMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxTokens: maxTokens,
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", nil, fmt.Errorf("build OpenAI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("OpenAI HTTP: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result OpenAIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", nil, fmt.Errorf("OpenAI response parse: %w", err)
	}
	if result.Error != nil {
		return "", nil, fmt.Errorf("OpenAI error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", nil, fmt.Errorf("OpenAI returned empty choices")
	}

	var usage *Usage
	if result.Usage != nil {
		usage = &Usage{
			InputTokens:  result.Usage.PromptTokens,
			OutputTokens: result.Usage.CompletionTokens,
		}
	}
	content := ""
	if result.Choices[0].Message.Content != nil {
		content = *result.Choices[0].Message.Content
	}
	return content, usage, nil
}

type OpenAIToolRequest struct {
	Model     string          `json:"model"`
	Messages  []OpenAIMessage `json:"messages"`
	Tools     []OpenAITool    `json:"tools,omitempty"`
	MaxTokens int             `json:"max_completion_tokens"`
}

type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIToolFunc `json:"function"`
}

type OpenAIToolFunc struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Parameters  OpenAIToolSchema `json:"parameters"`
}

type OpenAIToolSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]OpenAIToolProp `json:"properties"`
	Required   []string                  `json:"required"`
}

type OpenAIToolProp struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

func (c *OpenAIClient) CompleteWithTools(ctx context.Context, system string, messages []Message, tools []Tool, maxTokens int) (Message, *Usage, error) {
	reqBody := OpenAIToolRequest{
		Model:     c.model,
		Messages:  OpenAIConvertMessages(system, messages),
		Tools:     OpenAIConvertTools(tools),
		MaxTokens: maxTokens,
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return Message{}, nil, fmt.Errorf("build OpenAI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Message{}, nil, fmt.Errorf("OpenAI HTTP: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result OpenAIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return Message{}, nil, fmt.Errorf("OpenAI response parse: %w", err)
	}
	if result.Error != nil {
		return Message{}, nil, fmt.Errorf("OpenAI error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return Message{}, nil, fmt.Errorf("OpenAI returned empty choices")
	}

	choice := result.Choices[0].Message
	reply := Message{Role: "assistant"}
	if choice.Content != nil {
		reply.Text = *choice.Content
	}
	for _, tc := range choice.ToolCalls {
		var input map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
			return Message{}, nil, fmt.Errorf("parse tool arguments for %q: %w", tc.Function.Name, err)
		}
		reply.ToolUses = append(reply.ToolUses, ToolUse{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	var usage *Usage
	if result.Usage != nil {
		usage = &Usage{
			InputTokens:  result.Usage.PromptTokens,
			OutputTokens: result.Usage.CompletionTokens,
		}
	}
	return reply, usage, nil
}

func OpenAIConvertMessages(system string, messages []Message) []OpenAIMessage {
	out := []OpenAIMessage{{Role: "system", Content: system}}
	for _, m := range messages {
		switch m.Role {
		case "user":
			if len(m.ToolResults) > 0 {
				// OpenAI requires one "tool" role message per result.
				for _, tr := range m.ToolResults {
					out = append(out, OpenAIMessage{
						Role:       "tool",
						Content:    tr.Content,
						ToolCallID: tr.ToolUseID,
					})
				}
			} else {
				out = append(out, OpenAIMessage{Role: "user", Content: m.Text})
			}
		case "assistant":
			msg := OpenAIMessage{Role: "assistant"}
			if m.Text != "" {
				msg.Content = m.Text
			}
			for _, tu := range m.ToolUses {
				argsJSON, _ := json.Marshal(tu.Input)
				msg.ToolCalls = append(msg.ToolCalls, OpenAIToolCall{
					ID:   tu.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: tu.Name, Arguments: string(argsJSON)},
				})
			}
			out = append(out, msg)
		}
	}
	return out
}

func OpenAIConvertTools(tools []Tool) []OpenAITool {
	out := make([]OpenAITool, 0, len(tools))
	for _, t := range tools {
		props := make(map[string]OpenAIToolProp, len(t.Parameters.Properties))
		for name, p := range t.Parameters.Properties {
			props[name] = OpenAIToolProp{Type: p.Type, Description: p.Description}
		}
		out = append(out, OpenAITool{
			Type: "function",
			Function: OpenAIToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters: OpenAIToolSchema{
					Type:       "object",
					Properties: props,
					Required:   t.Parameters.Required,
				},
			},
		})
	}
	return out
}
