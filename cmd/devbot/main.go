package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"devbot/internal/agent"
	"devbot/internal/bot"
	"devbot/internal/budget"
	"devbot/internal/config"
	ghclient "devbot/internal/github"
	"devbot/internal/llm"
	"devbot/internal/scheduler"
	"devbot/internal/store"
	"devbot/internal/task"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	// Select database: Postgres if DATABASE_URL is set, otherwise SQLite
	var s store.Store
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		slog.Info("using Postgres database")
		s, err = store.NewPostgres(dbURL)
	} else {
		slog.Info("using SQLite database", "path", cfg.Database.Path)
		s, err = store.NewSQLite(cfg.Database.Path)
	}
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer s.Close()

	gh := ghclient.NewClient(cfg.GitHub)
	svc := task.NewService(s)

	// Build primary LLM client from the configured provider.
	primaryLLM, err := llm.New(cfg)
	if err != nil {
		slog.Error("failed to create LLM client", "err", err)
		os.Exit(1)
	}
	slog.Info("AI provider", "provider", primaryLLM.ProviderName())

	// Build budget manager.
	// The local fallback is only wired when a local section is configured AND
	// the primary provider is not already local.
	var activeLLM llm.Client = primaryLLM
	var bm *budget.Manager

	if cfg.Budget.MonthlyLimitUSD > 0 || cfg.Local.Model != "" {
		var fallbackLLM llm.Client
		if cfg.Local.Model != "" && cfg.AI.Provider != "local" {
			// Build a local client as fallback
			localCfg := cfg.Local
			if localCfg.BaseURL == "" {
				localCfg.BaseURL = "http://localhost:11434/v1"
			}
			fallbackLLM, err = llm.NewLocal(&localCfg)
			if err != nil {
				slog.Error("failed to create local LLM client", "err", err)
				os.Exit(1)
			}
			slog.Info("local fallback configured", "model", cfg.Local.Model)
		}

		bm = budget.New(primaryLLM, fallbackLLM, s, cfg.Budget.MonthlyLimitUSD, nil)
		activeLLM = bm // Manager itself implements llm.Client
		slog.Info("budget manager active",
			"limit_usd", cfg.Budget.MonthlyLimitUSD,
			"fallback", fallbackLLM != nil,
		)
	}

	ag := agent.New(cfg, s, gh, svc, activeLLM)

	// Create scheduler if enabled; broadcast is wired after bot creation.
	var sched *scheduler.Scheduler
	if cfg.Schedule.Enabled {
		sched, err = scheduler.New(&cfg.Schedule, svc, ag, nil)
		if err != nil {
			slog.Error("failed to create scheduler", "err", err)
			os.Exit(1)
		}
		slog.Info("scheduler enabled",
			"timezone", cfg.Schedule.Timezone,
			"work_hours", cfg.Schedule.WorkStart+"-"+cfg.Schedule.WorkEnd,
		)
	}

	b, err := bot.New(cfg, svc, gh, ag, sched, bm)
	if err != nil {
		slog.Error("failed to create bot", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Wire broadcast callbacks after bot exists (breaks init cycles).
	if sched != nil {
		sched.SetBroadcast(b.BroadcastMessage)
		go sched.Start(ctx)
	}
	if bm != nil {
		bm.SetBroadcast(b.BroadcastMessage)
	}

	slog.Info("DevBot starting",
		"repo", cfg.GitHub.Owner+"/"+cfg.GitHub.Repo,
		"ai_provider", ag.ProviderName(),
		"scheduler", cfg.Schedule.Enabled,
		"budget_limit", cfg.Budget.MonthlyLimitUSD,
	)
	b.Start(ctx)
	slog.Info("DevBot stopped")
}
