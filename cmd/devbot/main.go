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

	llmClient, err := llm.New(cfg)
	if err != nil {
		slog.Error("failed to create LLM client", "err", err)
		os.Exit(1)
	}
	slog.Info("AI provider", "provider", llmClient.ProviderName())

	ag := agent.New(cfg, s, gh, svc, llmClient)

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

	b, err := bot.New(cfg, svc, gh, ag, sched)
	if err != nil {
		slog.Error("failed to create bot", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Wire broadcast and start scheduler after bot exists.
	if sched != nil {
		sched.SetBroadcast(b.BroadcastMessage)
		go sched.Start(ctx)
	}

	slog.Info("DevBot starting",
		"repo", cfg.GitHub.Owner+"/"+cfg.GitHub.Repo,
		"ai_provider", llmClient.ProviderName(),
		"scheduler", cfg.Schedule.Enabled,
	)
	b.Start(ctx)
	slog.Info("DevBot stopped")
}
