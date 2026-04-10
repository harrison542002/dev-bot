package llm

import (
	"fmt"
	"strings"

	"devbot/internal/config"
)

// New returns the LLM client for the provider specified in cfg.AI.Provider.
// If provider is empty, "claude" is used for backward compatibility.
func New(cfg *config.Config) (Client, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.AI.Provider))
	if provider == "" {
		provider = "claude"
	}

	switch provider {
	case "claude":
		if cfg.Claude.APIKey == "" {
			return nil, fmt.Errorf("claude.api_key is required when ai.provider is claude")
		}
		return newClaudeClient(&cfg.Claude), nil

	case "openai":
		if cfg.OpenAI.APIKey == "" {
			return nil, fmt.Errorf("openai.api_key is required when ai.provider is openai")
		}
		return newOpenAIClient(&cfg.OpenAI), nil

	case "gemini":
		if cfg.Gemini.APIKey == "" {
			return nil, fmt.Errorf("gemini.api_key is required when ai.provider is gemini")
		}
		return newGeminiClient(&cfg.Gemini), nil

	case "local":
		return NewLocal(&cfg.Local)

	case "codex":
		return NewCodexClient(&cfg.Codex)

	default:
		return nil, fmt.Errorf("unknown ai.provider %q — valid values: claude, openai, gemini, local, codex", provider)
	}
}
