package llm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/harrison542002/dev-bot/internal/config"
)

type CodexClient struct {
	model string
}

// NewCodexClient creates an LLM client that delegates to the `codex` CLI.
// The CLI reads ~/.codex/auth.json automatically — no API key is needed here.
func NewCodexClient(cfg *config.CodexConfig) (Client, error) {
	return &CodexClient{model: cfg.Model}, nil
}

func (c *CodexClient) ProviderName() string {
	return fmt.Sprintf("Codex (%s)", c.model)
}

// RunAgent delegates the repository workflow to the Codex CLI running inside
// the already-cloned work tree. DevBot creates the PR after the branch exists.
func (c *CodexClient) RunAgent(ctx context.Context, workDir, branch, baseBranch, title, description, ghToken string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	prompt := fmt.Sprintf(`You are operating inside an already-cloned Git repository.

Complete the requested task directly in the working tree.

Required workflow:
1. Inspect only the files needed for the task.
2. Create and switch to branch %q from %q.
3. Implement the requested code changes.
4. Run relevant tests or checks when practical.
5. Commit the changes with a concise commit message.
6. Push the branch to origin.
7. Do not create a pull request. DevBot will create the PR after the branch is pushed.

Rules:
- Stay inside the current repository.
- Do not change git remotes.
- Do not ask for confirmation.
- If you cannot finish, explain the blocker clearly.

Task title: %s

Task description:
%s
`, branch, baseBranch, title, description)

	args := []string{
		"exec",
		"--model", c.model,
		"--dangerously-bypass-approvals-and-sandbox",
		"-",
	}

	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = append(os.Environ(),
		"GH_TOKEN="+ghToken,
		"GITHUB_TOKEN="+ghToken,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("codex native agent: %w", err)
		}
		return fmt.Errorf("codex native agent: %w\n%s", err, msg)
	}
	return nil
}

// Complete runs the prompt through the `codex` CLI and returns its output.
// The CLI wraps the model response in header/footer chrome; extractJSON strips
// that and returns only the last JSON object found in the output.
func (c *CodexClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	prompt := system + "\n\n" + user

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	lastMsgFile, err := os.CreateTemp("", "devbot-codex-last-message-*.txt")
	if err != nil {
		return "", nil, fmt.Errorf("codex temp file: %w", err)
	}
	lastMsgPath := lastMsgFile.Name()
	lastMsgFile.Close()
	defer os.Remove(lastMsgPath)

	args := []string{"exec", "--model", c.model}
	if maxTokens > 0 {
		// Codex CLI accepts config overrides via -c key=value for request shaping.
		args = append(args, "-c", fmt.Sprintf("model_max_output_tokens=%d", maxTokens))
	}
	args = append(args, "-o", lastMsgPath)
	args = append(args, "-")

	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Stdin = strings.NewReader(prompt)

	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", nil, fmt.Errorf("codex: %w", err)
		}
		return "", nil, fmt.Errorf("codex: %w\n%s", err, msg)
	}

	if data, err := os.ReadFile(lastMsgPath); err == nil {
		text := strings.TrimSpace(string(data))
		if text != "" {
			return text, nil, nil
		}
	}

	result := extractJSON(string(out))
	if result != "" {
		return result, nil, nil
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return "", nil, fmt.Errorf("codex returned no output")
	}
	return raw, nil, nil
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
