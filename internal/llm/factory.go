package llm

import (
	"fmt"
	"strings"

	"github.com/harrison542002/dev-bot/internal/config"
)

func New(cfg *config.Config) (Client, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.AI.Provider))
	if provider == "" {
		return nil, fmt.Errorf("ai.provider is required")
	}

	switch provider {
	case "claude":
		if cfg.Claude.APIKey == "" {
			return nil, fmt.Errorf("claude.api_key is required when ai.provider is claude")
		}
		return NewClaudeClient(&cfg.Claude), nil

	case "openai":
		if cfg.OpenAI.APIKey == "" {
			return nil, fmt.Errorf("openai.api_key is required when ai.provider is openai")
		}
		return NewOpenAIClient(&cfg.OpenAI), nil

	case "gemini":
		if cfg.Gemini.APIKey == "" {
			return nil, fmt.Errorf("gemini.api_key is required when ai.provider is gemini")
		}
		return NewGeminiClient(&cfg.Gemini), nil

	case "local":
		return NewLocal(&cfg.Local)

	case "codex":
		return NewCodexClient(&cfg.Codex)

	default:
		return nil, fmt.Errorf("unknown ai.provider %q — valid values: claude, openai, gemini, local, codex", provider)
	}
}
