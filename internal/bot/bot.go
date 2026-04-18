package bot

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/harrison542002/dev-bot/internal/agent"
	"github.com/harrison542002/dev-bot/internal/budget"
	"github.com/harrison542002/dev-bot/internal/config"
	ghclient "github.com/harrison542002/dev-bot/internal/github"
	"github.com/harrison542002/dev-bot/internal/scheduler"
	"github.com/harrison542002/dev-bot/internal/task"
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
	pool    *ghclient.ClientPool
	ag      *agent.Agent
	sched   *scheduler.Scheduler // nil when schedule.enabled=false
	budget  *budget.Manager      // nil when budget is not configured
	pl      Platform
	readyCh chan struct{}

	wizardMu     sync.Mutex
	wizards      map[string]*wizardSession      // sessionKey → active task wizard
	schedWizards map[string]*schedWizardSession // sessionKey → active schedule wizard
}

func New(cfg *config.Config, taskSvc *task.Service, pool *ghclient.ClientPool, ag *agent.Agent, sched *scheduler.Scheduler, bm *budget.Manager) (*Bot, error) {
	b := &Bot{
		cfg:          cfg,
		taskSvc:      taskSvc,
		pool:         pool,
		ag:           ag,
		sched:        sched,
		budget:       bm,
		readyCh:      make(chan struct{}),
		wizards:      make(map[string]*wizardSession),
		schedWizards: make(map[string]*schedWizardSession),
	}

	platform := strings.ToLower(strings.TrimSpace(cfg.Bot.Platform))
	if platform == "" {
		platform = "telegram"
	}

	switch platform {
	case "telegram":
		pl, err := newTelegramPlatform(cfg, b.handleMessage)
		if err != nil {
			return nil, fmt.Errorf("telegram: %w", err)
		}
		b.pl = pl
	case "discord":
		pl, err := newDiscordPlatform(cfg, b.handleMessage)
		if err != nil {
			return nil, fmt.Errorf("discord: %w", err)
		}
		b.pl = pl
	default:
		return nil, fmt.Errorf("unknown bot.platform %q — valid values: telegram, discord", platform)
	}

	return b, nil
}

// Ready returns a channel that is closed once the bot is connected and ready
// to send messages. Use this to delay dependent components (e.g. the scheduler)
// until the bot can actually deliver broadcasts.
func (b *Bot) Ready() <-chan struct{} {
	return b.readyCh
}

func (b *Bot) Start(ctx context.Context) {
	close(b.readyCh)
	b.pl.Start(ctx)
}

// BroadcastMessage sends msg to every allowed user via the active platform.
// Used by the scheduler and budget manager to push notifications.
func (b *Bot) BroadcastMessage(msg string) {
	b.pl.BroadcastMessage(msg)
}

// handleMessage is the single entry point called by platforms for every
// message from an allowed user. It routes active wizard sessions first,
// then falls through to command dispatch.
func (b *Bot) handleMessage(ctx context.Context, sessionKey, text string, notify func(string)) {
	b.wizardMu.Lock()
	wiz, inWizard := b.wizards[sessionKey]
	swiz, inSchedWizard := b.schedWizards[sessionKey]
	b.wizardMu.Unlock()

	if inWizard {
		b.stepWizard(ctx, sessionKey, wiz, text, notify)
		return
	}
	if inSchedWizard {
		b.stepSchedWizard(ctx, sessionKey, swiz, text, notify)
		return
	}

	parts := strings.Fields(text)
	if len(parts) == 0 || !strings.HasPrefix(parts[0], "/") {
		return
	}
	b.dispatch(ctx, sessionKey, parts, notify)
}

// dispatch routes a parsed command (parts[0] must be "/cmd") to the handler.
func (b *Bot) dispatch(ctx context.Context, sessionKey string, parts []string, notify func(string)) {
	if len(parts) == 0 {
		return
	}
	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "/task":
		handleTask(ctx, b, sessionKey, args, notify)
	case "/pr":
		handlePR(ctx, b, 0, args, notify)
	case "/schedule":
		handleSchedule(ctx, b, sessionKey, args, notify)
	case "/budget":
		handleBudget(ctx, b, args, notify)
	case "/status":
		handleStatus(ctx, b, notify)
	case "/timezone":
		handleTimezone(args, notify)
	case "/help":
		handleHelp(notify)
	default:
		notify("Unknown command. Send /help to see available commands.")
	}
}

// startWizard registers an active wizard session for the given key.
func (b *Bot) startWizard(sessionKey string, wiz *wizardSession) {
	b.wizardMu.Lock()
	b.wizards[sessionKey] = wiz
	b.wizardMu.Unlock()
}

// endWizard removes the active wizard session for the given key.
func (b *Bot) endWizard(sessionKey string) {
	b.wizardMu.Lock()
	delete(b.wizards, sessionKey)
	b.wizardMu.Unlock()
}

// startSchedWizard registers an active schedule wizard for the given key.
func (b *Bot) startSchedWizard(sessionKey string, wiz *schedWizardSession) {
	b.wizardMu.Lock()
	b.schedWizards[sessionKey] = wiz
	b.wizardMu.Unlock()
}

// endSchedWizard removes the active schedule wizard for the given key.
func (b *Bot) endSchedWizard(sessionKey string) {
	b.wizardMu.Lock()
	delete(b.schedWizards, sessionKey)
	b.wizardMu.Unlock()
}
