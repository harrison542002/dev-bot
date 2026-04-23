package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Discord  DiscordConfig  `yaml:"discord"`
	Bot      BotConfig      `yaml:"bot"`
	GitHub   GitHubConfig   `yaml:"github"`
	Git      GitIdentity    `yaml:"git"`
	AI       AIConfig       `yaml:"ai"`
	Claude   ClaudeConfig   `yaml:"claude"`
	OpenAI   OpenAIConfig   `yaml:"openai"`
	Gemini   GeminiConfig   `yaml:"gemini"`
	Local    LocalConfig    `yaml:"local"`
	Codex    CodexConfig    `yaml:"codex"`
	Budget   BudgetConfig   `yaml:"budget"`
	Database DatabaseConfig `yaml:"database"`
	Schedule ScheduleConfig `yaml:"schedule"`
}

// GitIdentity sets the author name and email used when DevBot commits code.
// Set these to your GitHub-verified email so commits appear as "Verified" on GitHub.
type GitIdentity struct {
	Name  string `yaml:"name"`  // e.g. "Harrison"
	Email string `yaml:"email"` // e.g. "you@example.com"
}

// AIConfig selects which provider powers the agent.
// If omitted, the provider defaults to "claude" for backward compatibility.
type AIConfig struct {
	Provider string `yaml:"provider"` // claude | openai | gemini | local | codex
}

// CodexConfig configures the Codex CLI provider.
// Authentication is handled by the `codex` CLI itself via ~/.codex/auth.json.
// Run `codex login` once before starting DevBot.
type CodexConfig struct {
	Model string `yaml:"model"` // defaults to "codex-mini-latest"
}

// BudgetConfig controls monthly spend limits and automatic fallback.
type BudgetConfig struct {
	// MonthlyLimitUSD is the maximum USD to spend on commercial AI per calendar month.
	// 0 means unlimited (tracking only).
	MonthlyLimitUSD float64 `yaml:"monthly_limit_usd"`
}

type ScheduleConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Timezone      string `yaml:"timezone"`       // IANA tz, e.g. "Asia/Bangkok"
	WorkStart     string `yaml:"work_start"`     // "09:00"
	WorkEnd       string `yaml:"work_end"`       // "17:00"
	CheckInterval string `yaml:"check_interval"` // Go duration, e.g. "10s", "1m", "500ms"
	EnableWeekend bool   `yaml:"enable_weekend"` // process tasks on Sat/Sun too
}

func (s ScheduleConfig) CheckIntervalDuration() time.Duration {
	if strings.TrimSpace(s.CheckInterval) != "" {
		d, err := time.ParseDuration(strings.TrimSpace(s.CheckInterval))
		if err == nil {
			return d
		}
	}
	return 10 * time.Minute
}

// BotConfig selects the messaging platform DevBot listens on.
type BotConfig struct {
	// Platform is the messaging backend: "telegram" (default) or "discord".
	Platform string `yaml:"platform"`
}

type TelegramConfig struct {
	Token          string  `yaml:"token"`
	AllowedUserIDs []int64 `yaml:"allowed_user_ids"`
}

// DiscordConfig configures the Discord bot backend.
type DiscordConfig struct {
	// Token is the Discord bot token from the Developer Portal.
	Token string `yaml:"token"`
	// AllowedUserIDs is a list of Discord user snowflake IDs (as strings) that
	// may issue commands. All other users are silently ignored.
	AllowedUserIDs []string `yaml:"allowed_user_ids"`
	// CommandPrefix is the character(s) that prefix commands (default "!").
	// e.g. prefix "!" → type "!task add ..." in a channel or DM.
	CommandPrefix string `yaml:"command_prefix"`
}

type GitHubConfig struct {
	Token      string       `yaml:"token"`
	Owner      string       `yaml:"owner"`       // legacy single-repo shorthand
	Repo       string       `yaml:"repo"`        // legacy single-repo shorthand
	BaseBranch string       `yaml:"base_branch"` // default base branch for all repos
	Repos      []RepoConfig `yaml:"repos"`       // multi-repo list (takes precedence)
}

// RepoConfig describes one target repository.
// Token and BaseBranch fall back to the parent GitHubConfig values when empty.
type RepoConfig struct {
	Owner string `yaml:"owner"`
	Repo  string `yaml:"repo"`
	// Name is an optional short alias used in commands: /task add <name> "description"
	Name       string `yaml:"name"`
	BaseBranch string `yaml:"base_branch"` // optional override
	Token      string `yaml:"token"`       // optional per-repo token override
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
	BaseURL string `yaml:"base_url"` // e.g. http://localhost:11434
	Model   string `yaml:"model"`    // e.g. llama3.2, mistral, gemma3
	APIKey  string `yaml:"api_key"`  // usually empty; some servers accept a dummy value
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
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
	if cfg.Git.Name == "" {
		cfg.Git.Name = "DevBot"
	}
	if cfg.Git.Email == "" {
		cfg.Git.Email = "devbot@users.noreply.github.com"
	}

	// Normalize repos: if no repos list, promote legacy owner/repo to a single entry.
	if len(cfg.GitHub.Repos) == 0 && cfg.GitHub.Owner != "" {
		cfg.GitHub.Repos = []RepoConfig{{
			Owner: cfg.GitHub.Owner,
			Repo:  cfg.GitHub.Repo,
		}}
	}
	// Fill in inherited defaults for each repo entry.
	// base_branch is only inherited from the global value for single-repo configs;
	// when multiple repos are listed each must declare its own (enforced by validate).
	multiRepo := len(cfg.GitHub.Repos) > 1
	for i := range cfg.GitHub.Repos {
		if cfg.GitHub.Repos[i].Token == "" {
			cfg.GitHub.Repos[i].Token = cfg.GitHub.Token
		}
		if !multiRepo && cfg.GitHub.Repos[i].BaseBranch == "" {
			cfg.GitHub.Repos[i].BaseBranch = cfg.GitHub.BaseBranch
		}
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
			cfg.Local.BaseURL = "http://localhost:11434"
		}
	case "codex":
		if cfg.Codex.Model == "" {
			cfg.Codex.Model = "codex-mini-latest"
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
	if strings.TrimSpace(cfg.Schedule.CheckInterval) == "" {
		cfg.Schedule.CheckInterval = "10m"
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	platform := strings.ToLower(strings.TrimSpace(c.Bot.Platform))
	if platform == "" {
		platform = "telegram"
	}
	switch platform {
	case "telegram":
		if c.Telegram.Token == "" {
			return fmt.Errorf("telegram.token is required when bot.platform is telegram (or not set)")
		}
		if len(c.Telegram.AllowedUserIDs) == 0 {
			return fmt.Errorf("telegram.allowed_user_ids must contain at least one user ID")
		}
	case "discord":
		if c.Discord.Token == "" {
			return fmt.Errorf("discord.token is required when bot.platform is discord")
		}
		if len(c.Discord.AllowedUserIDs) == 0 {
			return fmt.Errorf("discord.allowed_user_ids must contain at least one user ID")
		}
	default:
		return fmt.Errorf("unknown bot.platform %q — valid values: telegram, discord", platform)
	}

	if c.GitHub.Token == "" {
		return fmt.Errorf("github.token is required")
	}
	// Accept either legacy owner+repo or a repos list (or both)
	if len(c.GitHub.Repos) == 0 && (c.GitHub.Owner == "" || c.GitHub.Repo == "") {
		return fmt.Errorf("github.owner+github.repo (single repo) or github.repos list is required")
	}
	for i, r := range c.GitHub.Repos {
		if r.Owner == "" || r.Repo == "" {
			return fmt.Errorf("github.repos[%d]: owner and repo are required", i)
		}
		// When multiple repos are listed each must declare its own base_branch
		// so branches don't silently collide across repositories.
		if len(c.GitHub.Repos) > 1 && r.BaseBranch == "" {
			return fmt.Errorf("github.repos[%d] (%s/%s): base_branch is required when configuring multiple repos", i, r.Owner, r.Repo)
		}
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
	case "codex":
		// No credentials needed here — the codex CLI reads ~/.codex/auth.json directly.
	default:
		return fmt.Errorf("unknown ai.provider %q — valid values: claude, openai, gemini, local, codex", provider)
	}

	if intervalStr := strings.TrimSpace(c.Schedule.CheckInterval); intervalStr != "" {
		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			return fmt.Errorf("invalid schedule.check_interval %q: %w", intervalStr, err)
		}
		if interval < time.Second {
			return fmt.Errorf("schedule.check_interval must be at least 1s")
		}
	} else if interval := c.Schedule.CheckIntervalDuration(); interval < time.Second {
		return fmt.Errorf("schedule.check_interval must be at least 1s")
	}
	return nil
}
