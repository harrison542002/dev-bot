package bot

import (
	"context"
	"fmt"

	"github.com/harrison542002/dev-bot/internal/entities"
)

func handleSchedule(ctx context.Context, b *Bot, sessionKey string, args []string, notify func(string)) {
	if b.sched == nil {
		notify("Auto-scheduler is not enabled.\nSet schedule.enabled: true in config.yaml and restart DevBot.")
		return
	}

	if len(args) == 0 {
		notify(b.sched.Status(ctx))
		return
	}

	switch args[0] {
	case "on":
		b.sched.Resume()
		notify("Scheduler resumed. TODO tasks will be auto-started during work hours.")

	case "off":
		b.sched.Pause()
		notify("Scheduler paused. No tasks will be auto-started until you run /schedule on.")

	case "next":
		tasks, err := b.taskSvc.List(ctx)
		if err != nil {
			notify(fmt.Sprintf("Error: %v", err))
			return
		}
		for _, t := range tasks {
			if t.Status == entities.StatusTodo {
				notify(fmt.Sprintf("Next task: #%d %s", t.ID, t.Title))
				return
			}
		}
		notify("No TODO tasks queued. Add one with /task create.")

	case "setup":
		b.startScheduleWizard(sessionKey, notify)

	default:
		notify("Usage:\n  /schedule          Show scheduler status\n  /schedule on        Resume auto-processing\n  /schedule off       Pause auto-processing\n  /schedule next      Show next queued task\n  /schedule setup     Configure timezone and work hours")
	}
}

func handleBudget(ctx context.Context, b *Bot, args []string, notify func(string)) {
	if b.budget == nil {
		notify("Budget tracking is not configured.\nSet budget.monthly_limit_usd in config.yaml to enable it.")
		return
	}

	if len(args) == 0 {
		notify(b.budget.Status(ctx))
		return
	}

	switch args[0] {
	case "pause":
		b.budget.Pause()
		notify("Budget enforcement paused. Commercial provider will be used regardless of spend.\nUse /budget resume to re-enable automatic switching.")

	case "resume":
		b.budget.Resume()
		notify("Budget enforcement resumed. DevBot will switch to the local model when the monthly limit is reached.")

	default:
		notify("Usage:\n  /budget          Show spend for this month\n  /budget pause    Always use commercial provider (ignore limit)\n  /budget resume   Re-enable automatic fallback to local model")
	}
}
