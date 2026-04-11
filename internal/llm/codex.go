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
func (c *codexClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	prompt := system + "\n\n" + user

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "codex", "exec",
		"--model", c.model,
		"--ask-for-approval", "never",
		prompt,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", nil, fmt.Errorf("codex: %w", err)
		}
		return "", nil, fmt.Errorf("codex: %w\n%s", err, msg)
	}

	return strings.TrimSpace(string(out)), nil, nil
}
