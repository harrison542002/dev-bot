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
	ag := agent.New(cfg, s, gh, svc)

	b, err := bot.New(cfg, svc, gh, ag)
	if err != nil {
		slog.Error("failed to create bot", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("DevBot starting",
		"repo", cfg.GitHub.Owner+"/"+cfg.GitHub.Repo,
		"model", cfg.Claude.Model,
	)
	b.Start(ctx)
	slog.Info("DevBot stopped")
}
