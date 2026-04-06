package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
	GitHub   GitHubConfig   `yaml:"github"`
	Claude   ClaudeConfig   `yaml:"claude"`
	Database DatabaseConfig `yaml:"database"`
	Schedule ScheduleConfig `yaml:"schedule"`
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
	if cfg.Claude.Model == "" {
		cfg.Claude.Model = "claude-sonnet-4-6"
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "./devbot.db"
	}
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
	if c.Claude.APIKey == "" {
		return fmt.Errorf("claude.api_key is required")
	}
	return nil
}
