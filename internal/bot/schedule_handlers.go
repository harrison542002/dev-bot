package bot

import (
	"context"
	"fmt"

	"github.com/harrison542002/dev-bot/internal/store"
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
			if t.Status == store.StatusTodo {
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
