package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/harrison542002/dev-bot/internal/config"
)

// LocalClient calls the native Ollama /api/chat endpoint.
// This is distinct from the OpenAI-compatible /v1/chat/completions path,
// which Ollama supports but with subtle incompatibilities (streaming default,
// max_tokens vs max_completion_tokens, etc.).
type LocalClient struct {
	baseURL string // e.g. http://localhost:11434
	model   string
}

// --- request types ---

type OllamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []OpenAITool    `json:"tools,omitempty"`
	Options  OllamaOptions   `json:"options,omitempty"`
}

type OllamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"`
}

type OllamaOptions struct {
	NumPredict int `json:"num_predict,omitempty"`
}

// --- response types ---

type OllamaResponse struct {
	Message    OllamaMessage `json:"message"`
	DoneReason string        `json:"done_reason"`
	Error      string        `json:"error"`
}

type OllamaToolCall struct {
	Function OllamaToolCallFunc `json:"function"`
}

type OllamaToolCallFunc struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"` // object, not a JSON string like OpenAI
}

func NewLocal(cfg *config.LocalConfig) (Client, error) {
	base := strings.TrimSuffix(strings.TrimSuffix(cfg.BaseURL, "/v1"), "/")
	if base == "" {
		base = "http://localhost:11434"
	}
	return &LocalClient{
		model:   cfg.Model,
		baseURL: base,
	}, nil
}

func (c *LocalClient) ProviderName() string {
	return fmt.Sprintf("Local (%s)", c.model)
}

func (c *LocalClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	req := OllamaRequest{
		Model: c.model,
		Messages: []OllamaMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Stream:  false,
		Options: OllamaOptions{NumPredict: maxTokens},
	}
	resp, err := c.post(ctx, req)
	if err != nil {
		return "", nil, err
	}
	return resp.Message.Content, nil, nil
}

func (c *LocalClient) CompleteWithTools(ctx context.Context, system string, messages []Message, tools []Tool, maxTokens int) (Message, *Usage, error) {
	req := OllamaRequest{
		Model:    c.model,
		Messages: OllamaConvertMessages(system, messages),
		Stream:   false,
		Tools:    OpenAIConvertTools(tools),
		Options:  OllamaOptions{NumPredict: maxTokens},
	}
	resp, err := c.post(ctx, req)
	if err != nil {
		return Message{}, nil, err
	}

	reply := Message{Role: "assistant", Text: resp.Message.Content}

	raw2, _ := json.Marshal(resp.Message)
	slog.Debug("Ollama raw message", "message", string(raw2))

	for i, tc := range resp.Message.ToolCalls {
		reply.ToolUses = append(reply.ToolUses, ToolUse{
			ID:    fmt.Sprintf("call_%d", i),
			Name:  tc.Function.Name,
			Input: tc.Function.Arguments,
		})
	}
	return reply, nil, nil
}

func (c *LocalClient) post(ctx context.Context, body any) (*OllamaResponse, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("build Ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama HTTP: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result OllamaResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("Ollama response parse: %w\nraw: %.500s", err, raw)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("Ollama error: %s", result.Error)
	}
	return &result, nil
}

// OllamaConvertMessages converts the internal Message slice into Ollama's
// flat message format. Tool results are mapped to "tool" role messages.
func OllamaConvertMessages(system string, messages []Message) []OllamaMessage {
	out := []OllamaMessage{{Role: "system", Content: system}}
	for _, m := range messages {
		switch m.Role {
		case "user":
			if len(m.ToolResults) > 0 {
				for _, tr := range m.ToolResults {
					out = append(out, OllamaMessage{Role: "tool", Content: tr.Content})
				}
			} else if m.Text != "" {
				out = append(out, OllamaMessage{Role: "user", Content: m.Text})
			}
		case "assistant":
			msg := OllamaMessage{Role: "assistant", Content: m.Text}
			for _, tu := range m.ToolUses {
				msg.ToolCalls = append(msg.ToolCalls, OllamaToolCall{
					Function: OllamaToolCallFunc{Name: tu.Name, Arguments: tu.Input},
				})
			}
			out = append(out, msg)
		}
	}
	return out
}
