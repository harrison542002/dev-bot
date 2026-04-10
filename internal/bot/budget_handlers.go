package bot

import (
	"context"
)

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
