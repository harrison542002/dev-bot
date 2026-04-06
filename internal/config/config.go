package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
	GitHub   GitHubConfig   `yaml:"github"`
	AI       AIConfig       `yaml:"ai"`
	Claude   ClaudeConfig   `yaml:"claude"`
	OpenAI   OpenAIConfig   `yaml:"openai"`
	Gemini   GeminiConfig   `yaml:"gemini"`
	Local    LocalConfig    `yaml:"local"`
	Database DatabaseConfig `yaml:"database"`
	Schedule ScheduleConfig `yaml:"schedule"`
}

// AIConfig selects which provider powers the agent.
// If omitted, the provider defaults to "claude" for backward compatibility.
type AIConfig struct {
	Provider string `yaml:"provider"` // claude | openai | gemini
}

type ScheduleConfig struct {
	Enabled              bool   `yaml:"enabled"`
	Timezone             string `yaml:"timezone"`               // IANA tz, e.g. "Asia/Bangkok"
	WorkStart            string `yaml:"work_start"`             // "09:00"
	WorkEnd              string `yaml:"work_end"`               // "17:00"
	CheckIntervalMinutes int    `yaml:"check_interval_minutes"` // default 10
}

type TelegramConfig struct {
	Token          string  `yaml:"token"`
	AllowedUserIDs []int64 `yaml:"allowed_user_ids"`
}

type GitHubConfig struct {
	Token      string `yaml:"token"`
	Owner      string `yaml:"owner"`
	Repo       string `yaml:"repo"`
	BaseBranch string `yaml:"base_branch"`
}

type ClaudeConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type OpenAIConfig struct {
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url"` // optional; defaults to https://api.openai.com/v1
}

type GeminiConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

// LocalConfig targets any OpenAI-compatible local inference server
// (Ollama, LM Studio, LocalAI, Jan, etc.).
type LocalConfig struct {
	BaseURL string `yaml:"base_url"` // e.g. http://localhost:11434/v1
	Model   string `yaml:"model"`    // e.g. llama3.2, mistral, gemma3
	APIKey  string `yaml:"api_key"`  // usually empty; some servers accept a dummy value
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %q: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Apply defaults
	if cfg.GitHub.BaseBranch == "" {
		cfg.GitHub.BaseBranch = "main"
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "./devbot.db"
	}

	// AI provider defaults
	provider := strings.ToLower(strings.TrimSpace(cfg.AI.Provider))
	if provider == "" {
		provider = "claude"
		cfg.AI.Provider = "claude"
	}
	switch provider {
	case "claude":
		if cfg.Claude.Model == "" {
			cfg.Claude.Model = "claude-sonnet-4-6"
		}
	case "openai":
		if cfg.OpenAI.Model == "" {
			cfg.OpenAI.Model = "gpt-4o"
		}
	case "gemini":
		if cfg.Gemini.Model == "" {
			cfg.Gemini.Model = "gemini-1.5-pro"
		}
	case "local":
		if cfg.Local.BaseURL == "" {
			cfg.Local.BaseURL = "http://localhost:11434/v1" // Ollama default
		}
	}

	// Schedule defaults
	if cfg.Schedule.Timezone == "" {
		cfg.Schedule.Timezone = "UTC"
	}
	if cfg.Schedule.WorkStart == "" {
		cfg.Schedule.WorkStart = "09:00"
	}
	if cfg.Schedule.WorkEnd == "" {
		cfg.Schedule.WorkEnd = "17:00"
	}
	if cfg.Schedule.CheckIntervalMinutes == 0 {
		cfg.Schedule.CheckIntervalMinutes = 10
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required")
	}
	if len(c.Telegram.AllowedUserIDs) == 0 {
		return fmt.Errorf("telegram.allowed_user_ids must contain at least one user ID")
	}
	if c.GitHub.Token == "" {
		return fmt.Errorf("github.token is required")
	}
	if c.GitHub.Owner == "" {
		return fmt.Errorf("github.owner is required")
	}
	if c.GitHub.Repo == "" {
		return fmt.Errorf("github.repo is required")
	}

	// Validate the selected provider has its API key set.
	// Provider defaults to "claude" when empty.
	provider := strings.ToLower(strings.TrimSpace(c.AI.Provider))
	if provider == "" {
		provider = "claude"
	}
	switch provider {
	case "claude":
		if c.Claude.APIKey == "" {
			return fmt.Errorf("claude.api_key is required when ai.provider is claude (or when ai.provider is not set)")
		}
	case "openai":
		if c.OpenAI.APIKey == "" {
			return fmt.Errorf("openai.api_key is required when ai.provider is openai")
		}
	case "gemini":
		if c.Gemini.APIKey == "" {
			return fmt.Errorf("gemini.api_key is required when ai.provider is gemini")
		}
	case "local":
		if c.Local.Model == "" {
			return fmt.Errorf("local.model is required when ai.provider is local (e.g. llama3.2, mistral)")
		}
	default:
		return fmt.Errorf("unknown ai.provider %q — valid values: claude, openai, gemini, local", provider)
	}

	return nil
}
