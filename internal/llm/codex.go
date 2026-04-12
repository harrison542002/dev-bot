package llm

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/harrison542002/dev-bot/internal/config"
)

type codexClient struct {
	model string
}

// NewCodexClient creates an LLM client that delegates to the `codex` CLI.
// The CLI reads ~/.codex/auth.json automatically — no API key is needed here.
func NewCodexClient(cfg *config.CodexConfig) (Client, error) {
	return &codexClient{model: cfg.Model}, nil
}

func (c *codexClient) ProviderName() string {
	return fmt.Sprintf("Codex (%s)", c.model)
}

// Complete runs the prompt through the `codex` CLI and returns its output.
// The CLI wraps the model response in header/footer chrome; extractJSON strips
// that and returns only the last JSON object found in the output.
func (c *codexClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	prompt := system + "\n\n" + user

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

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

	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", nil, fmt.Errorf("codex: %w", err)
		}
		return "", nil, fmt.Errorf("codex: %w\n%s", err, msg)
	}

	result := extractJSON(string(out))
	if result == "" {
		return "", nil, fmt.Errorf("codex returned no JSON\nraw: %s", strings.TrimSpace(string(out)))
	}
	return result, nil, nil
}

// extractJSON finds the last top-level JSON object in s, which is where the
// codex CLI places the model's response after stripping its header chrome.
func extractJSON(s string) string {
	last := strings.LastIndex(s, "{")
	if last == -1 {
		return ""
	}
	depth := 0
	for i := last; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[last : i+1]
			}
		}
	}
	return ""
}
