package bot

import (
	"context"
	"fmt"
	"strings"

	"devbot/internal/agent"
	"devbot/internal/budget"
	"devbot/internal/config"
	ghclient "devbot/internal/github"
	"devbot/internal/scheduler"
	"devbot/internal/task"
)

// Platform abstracts the messaging transport so the same command logic works
// across Telegram, Discord, and any future backend.
type Platform interface {
	Start(ctx context.Context)
	BroadcastMessage(msg string)
}

// Bot holds platform-agnostic state shared by all command handlers.
type Bot struct {
	cfg     *config.Config
	taskSvc *task.Service
	gh      *ghclient.Client
	ag      *agent.Agent
	sched   *scheduler.Scheduler // nil when schedule.enabled=false
	budget  *budget.Manager      // nil when budget is not configured
	pl      Platform
}

func New(cfg *config.Config, taskSvc *task.Service, gh *ghclient.Client, ag *agent.Agent, sched *scheduler.Scheduler, bm *budget.Manager) (*Bot, error) {
	b := &Bot{
		cfg:     cfg,
		taskSvc: taskSvc,
		gh:      gh,
		ag:      ag,
		sched:   sched,
		budget:  bm,
	}

	platform := strings.ToLower(strings.TrimSpace(cfg.Bot.Platform))
	if platform == "" {
		platform = "telegram"
	}

	switch platform {
	case "telegram":
		pl, err := newTelegramPlatform(cfg, b.dispatch)
		if err != nil {
			return nil, fmt.Errorf("telegram: %w", err)
		}
		b.pl = pl
	case "discord":
		pl, err := newDiscordPlatform(cfg, b.dispatch)
		if err != nil {
			return nil, fmt.Errorf("discord: %w", err)
		}
		b.pl = pl
	default:
		return nil, fmt.Errorf("unknown bot.platform %q — valid values: telegram, discord", platform)
	}

	return b, nil
}

func (b *Bot) Start(ctx context.Context) {
	b.pl.Start(ctx)
}

// BroadcastMessage sends msg to every allowed user via the active platform.
// Used by the scheduler and budget manager to push notifications.
func (b *Bot) BroadcastMessage(msg string) {
	b.pl.BroadcastMessage(msg)
}

// dispatch routes a parsed command (parts[0] must be "/cmd") to the handler.
func (b *Bot) dispatch(ctx context.Context, parts []string, notify func(string)) {
	if len(parts) == 0 {
		return
	}
	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "/task":
		handleTask(ctx, b, 0, args, notify)
	case "/pr":
		handlePR(ctx, b, 0, args, notify)
	case "/schedule":
		handleSchedule(ctx, b, args, notify)
	case "/budget":
		handleBudget(ctx, b, args, notify)
	case "/status":
		handleStatus(ctx, b, notify)
	case "/help":
		handleHelp(notify)
	default:
		notify("Unknown command. Send /help to see available commands.")
	}
}
