package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"devbot/internal/config"
)

const (
	// codexClientID is the public OAuth2 client ID for the official OpenAI Codex CLI.
	codexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexTokenURL = "https://auth.openai.com/oauth/token"
	codexAPIBase  = "https://api.openai.com/v1"
)

// codexTokens mirrors the ~/.codex/auth.json credential file written by the
// official Codex CLI, so DevBot can read and write the same file.
type codexTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
}

type codexClient struct {
	model     string
	tokenFile string // path where tokens are persisted after refresh

	mu     sync.Mutex
	tokens codexTokens
}

// NewCodexClient creates an LLM client that authenticates with an OAuth2
// Bearer token from a ChatGPT Plus/Pro/Team subscription via the Codex flow.
//
// Priority for token loading:
//  1. Tokens explicitly set in config (codex.access_token / codex.refresh_token)
//  2. Credential file: codex.token_file if set, else ~/.codex/auth.json
func NewCodexClient(cfg *config.CodexConfig) (Client, error) {
	tokenFile := cfg.TokenFile
	if tokenFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		tokenFile = filepath.Join(home, ".codex", "auth.json")
	}

	c := &codexClient{
		model:     cfg.Model,
		tokenFile: tokenFile,
	}

	if cfg.AccessToken != "" {
		// Explicit tokens in config take precedence.
		c.tokens = codexTokens{
			AccessToken:  cfg.AccessToken,
			RefreshToken: cfg.RefreshToken,
		}
	} else {
		// Fall back to credential file (written by `codex login`).
		if err := c.loadTokenFile(); err != nil {
			return nil, fmt.Errorf(
				"codex: no access_token in config and could not read %s: %w\n"+
					"Run `codex login` to authenticate, or set codex.access_token manually.",
				tokenFile, err,
			)
		}
		slog.Info("codex: loaded credentials from file", "file", tokenFile)
	}

	if c.tokens.AccessToken == "" {
		return nil, fmt.Errorf("codex: access_token is empty — run `codex login` or set codex.access_token in config")
	}

	return c, nil
}

func (c *codexClient) ProviderName() string {
	return fmt.Sprintf("Codex (%s)", c.model)
}

// Complete calls the OpenAI chat/completions endpoint with an OAuth2 Bearer
// token. If the token is expired (HTTP 401), it refreshes automatically.
func (c *codexClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	text, usage, err := c.doRequest(ctx, system, user, maxTokens)
	if err == nil {
		return text, usage, nil
	}

	// On auth failure, refresh and retry once.
	if isAuthError(err) {
		slog.Info("codex: access token expired, refreshing…")
		if rerr := c.refresh(ctx); rerr != nil {
			return "", nil, fmt.Errorf("codex token refresh failed: %w (original: %v)", rerr, err)
		}
		return c.doRequest(ctx, system, user, maxTokens)
	}

	return "", nil, err
}

func (c *codexClient) doRequest(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	c.mu.Lock()
	accessToken := c.tokens.AccessToken
	c.mu.Unlock()

	reqBody := openaiRequest{
		Model: c.model,
		Messages: []openaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxTokens: maxTokens,
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexAPIBase+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", nil, fmt.Errorf("build codex request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("codex HTTP: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return "", nil, &authError{msg: "401 Unauthorized"}
	}

	var result openaiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", nil, fmt.Errorf("codex response parse: %w", err)
	}
	if result.Error != nil {
		return "", nil, fmt.Errorf("codex API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", nil, fmt.Errorf("codex returned empty choices")
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

// refresh exchanges the refresh_token for a new access_token.
func (c *codexClient) refresh(ctx context.Context) error {
	c.mu.Lock()
	refreshToken := c.tokens.RefreshToken
	c.mu.Unlock()

	if refreshToken == "" {
		return fmt.Errorf("no refresh_token available — re-run `codex login` or update codex.access_token in config")
	}

	formData := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {codexClientID},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexTokenURL,
		strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("refresh HTTP: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := string(body)
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return fmt.Errorf("refresh returned %d: %s", resp.StatusCode, msg)
	}

	var newTokens codexTokens
	if err := json.Unmarshal(body, &newTokens); err != nil {
		return fmt.Errorf("parse refresh response: %w", err)
	}
	if newTokens.AccessToken == "" {
		return fmt.Errorf("refresh response missing access_token")
	}
	// Preserve the existing refresh token if the server did not issue a new one.
	if newTokens.RefreshToken == "" {
		newTokens.RefreshToken = refreshToken
	}

	c.mu.Lock()
	c.tokens = newTokens
	c.mu.Unlock()

	if err := c.saveTokenFile(newTokens); err != nil {
		// Non-fatal: log a warning but don't fail the request.
		slog.Warn("codex: could not save refreshed tokens", "file", c.tokenFile, "err", err)
	} else {
		slog.Info("codex: tokens refreshed and saved", "file", c.tokenFile)
	}
	return nil
}

func (c *codexClient) loadTokenFile() error {
	data, err := os.ReadFile(c.tokenFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &c.tokens)
}

func (c *codexClient) saveTokenFile(t codexTokens) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.tokenFile), 0700); err != nil {
		return err
	}
	return os.WriteFile(c.tokenFile, data, 0600)
}

// authError is returned for HTTP 401 responses so the caller can detect and retry.
type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }

func isAuthError(err error) bool {
	_, ok := err.(*authError)
	return ok
}
