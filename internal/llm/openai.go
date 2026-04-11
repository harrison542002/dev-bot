package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"devbot/internal/config"
)

type openaiClient struct {
	apiKey  string
	model   string
	baseURL string
}

func newOpenAIClient(cfg *config.OpenAIConfig) Client {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return &openaiClient{apiKey: cfg.APIKey, model: cfg.Model, baseURL: base}
}

func (c *openaiClient) ProviderName() string { return "OpenAI" }

// ── simple completion ──────────────────────────────────────────────────────────

type openaiRequest struct {
	Model     string          `json:"model"`
	Messages  []openaiMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"` // string or null
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content   *string          `json:"content"`
			ToolCalls []openaiToolCall `json:"tool_calls"`
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

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON-encoded string
	} `json:"function"`
}

func (c *openaiClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	reqBody := openaiRequest{
		Model: c.model,
		Messages: []openaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxTokens: maxTokens,
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", nil, fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("openai HTTP: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result openaiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", nil, fmt.Errorf("openai response parse: %w", err)
	}
	if result.Error != nil {
		return "", nil, fmt.Errorf("openai error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", nil, fmt.Errorf("openai returned empty choices")
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

// ── tool-use completion ────────────────────────────────────────────────────────

type openaiToolRequest struct {
	Model     string          `json:"model"`
	Messages  []openaiMessage `json:"messages"`
	Tools     []openaiTool    `json:"tools,omitempty"`
	MaxTokens int             `json:"max_tokens"`
}

type openaiTool struct {
	Type     string         `json:"type"` // "function"
	Function openaiToolFunc `json:"function"`
}

type openaiToolFunc struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  openaiToolSchema  `json:"parameters"`
}

type openaiToolSchema struct {
	Type       string                     `json:"type"` // "object"
	Properties map[string]openaiToolProp  `json:"properties"`
	Required   []string                   `json:"required"`
}

type openaiToolProp struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

func (c *openaiClient) CompleteWithTools(ctx context.Context, system string, messages []Message, tools []Tool, maxTokens int) (Message, *Usage, error) {
	reqBody := openaiToolRequest{
		Model:     c.model,
		Messages:  openaiConvertMessages(system, messages),
		Tools:     openaiConvertTools(tools),
		MaxTokens: maxTokens,
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return Message{}, nil, fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Message{}, nil, fmt.Errorf("openai HTTP: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result openaiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return Message{}, nil, fmt.Errorf("openai response parse: %w", err)
	}
	if result.Error != nil {
		return Message{}, nil, fmt.Errorf("openai error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return Message{}, nil, fmt.Errorf("openai returned empty choices")
	}

	choice := result.Choices[0].Message
	reply := Message{Role: "assistant"}
	if choice.Content != nil {
		reply.Text = *choice.Content
	}
	for _, tc := range choice.ToolCalls {
		var input map[string]any
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
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

func openaiConvertMessages(system string, messages []Message) []openaiMessage {
	out := []openaiMessage{{Role: "system", Content: system}}
	for _, m := range messages {
		switch m.Role {
		case "user":
			if len(m.ToolResults) > 0 {
				// OpenAI requires one "tool" role message per result.
				for _, tr := range m.ToolResults {
					out = append(out, openaiMessage{
						Role:       "tool",
						Content:    tr.Content,
						ToolCallID: tr.ToolUseID,
					})
				}
			} else {
				out = append(out, openaiMessage{Role: "user", Content: m.Text})
			}
		case "assistant":
			msg := openaiMessage{Role: "assistant"}
			if m.Text != "" {
				msg.Content = m.Text
			}
			for _, tu := range m.ToolUses {
				argsJSON, _ := json.Marshal(tu.Input)
				msg.ToolCalls = append(msg.ToolCalls, openaiToolCall{
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

func openaiConvertTools(tools []Tool) []openaiTool {
	out := make([]openaiTool, 0, len(tools))
	for _, t := range tools {
		props := make(map[string]openaiToolProp, len(t.Parameters.Properties))
		for name, p := range t.Parameters.Properties {
			props[name] = openaiToolProp{Type: p.Type, Description: p.Description}
		}
		out = append(out, openaiTool{
			Type: "function",
			Function: openaiToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters: openaiToolSchema{
					Type:       "object",
					Properties: props,
					Required:   t.Parameters.Required,
				},
			},
		})
	}
	return out
}
