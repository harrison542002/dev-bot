package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/harrison542002/dev-bot/internal/store"
)

func handleTask(ctx context.Context, b *Bot, sessionKey string, args []string, notify func(string)) {
	if len(args) == 0 {
		notify("Usage: /task create|list|do|done|block|show|status")
		return
	}

	switch args[0] {
	case "create":
		b.startCreateWizard(ctx, sessionKey, notify)

	case "list":
		tasks, err := b.taskSvc.List(ctx)
		if err != nil {
			notify(fmt.Sprintf("Error: %v", err))
			return
		}
		if len(tasks) == 0 {
			notify("No tasks yet. Create one with /task create")
			return
		}
		var sb strings.Builder
		sb.WriteString("Tasks:\n")
		for _, t := range tasks {
			repoLabel := ""
			if b.pool.IsMultiRepo() && t.RepoOwner != "" {
				repoLabel = fmt.Sprintf(" [%s/%s]", t.RepoOwner, t.RepoName)
			}
			sb.WriteString(fmt.Sprintf("\n[%d]%s %s — %s", t.ID, repoLabel, t.Title, t.Status))
			if t.PRUrl != "" {
				sb.WriteString(fmt.Sprintf("\n    PR: %s", t.PRUrl))
			}
			if t.Error != "" && (t.Status == store.StatusFailed || t.Status == store.StatusBlocked) {
				sb.WriteString(fmt.Sprintf("\n    Error: %s", t.Error))
			}
		}
		notify(sb.String())

	case "do":
		id, err := parseID(args)
		if err != nil {
			notify(err.Error())
			return
		}
		t, err := b.taskSvc.Get(ctx, id)
		if err != nil {
			notify(fmt.Sprintf("Task %d not found", id))
			return
		}
		if t.Status != store.StatusTodo {
			notify(fmt.Sprintf("Task %d is in %s state. Only TODO tasks can be started.\nTo retry a failed task, use /pr retry %d", id, t.Status, id))
			return
		}
		notify(fmt.Sprintf("Starting agent for task %d: %s", id, t.Title))
		go b.ag.Run(ctx, id, notify)

	case "done":
		id, err := parseID(args)
		if err != nil {
			notify(err.Error())
			return
		}
		t, err := b.taskSvc.MarkDone(ctx, id)
		if err != nil {
			notify(fmt.Sprintf("Error: %v", err))
			return
		}
		notify(fmt.Sprintf("Task %d marked as DONE: %s", t.ID, t.Title))

	case "block":
		if len(args) < 3 {
			notify("Usage: /task block <id> <reason>")
			return
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			notify("Invalid task ID")
			return
		}
		reason := strings.Join(args[2:], " ")
		t, err := b.taskSvc.Block(ctx, id, reason)
		if err != nil {
			notify(fmt.Sprintf("Error: %v", err))
			return
		}
		notify(fmt.Sprintf("Task %d blocked: %s\nReason: %s", t.ID, t.Title, reason))

	case "show":
		id, err := parseID(args)
		if err != nil {
			notify(err.Error())
			return
		}
		t, err := b.taskSvc.Get(ctx, id)
		if err != nil {
			notify(fmt.Sprintf("Task %d not found", id))
			return
		}
		notify(formatTask(t))

	case "status":
		if len(args) < 3 {
			notify("Usage: /task status <id> <status>\nValid statuses: todo, in_progress, in_review, done, blocked, failed")
			return
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			notify("Invalid task ID")
			return
		}
		newStatus := store.Status(strings.ToUpper(args[2]))
		t, err := b.taskSvc.SetStatus(ctx, id, newStatus)
		if err != nil {
			notify(fmt.Sprintf("Error: %v", err))
			return
		}
		notify(fmt.Sprintf("Task %d status changed to %s: %s", t.ID, t.Status, t.Title))

	default:
		notify("Unknown subcommand. Use: /task create|list|do|done|block|show|status")
	}
}

func formatTask(t *store.Task) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Task %d: %s\n", t.ID, t.Title))
	sb.WriteString(fmt.Sprintf("Status: %s\n", t.Status))
	if t.RepoOwner != "" {
		sb.WriteString(fmt.Sprintf("Repo: %s/%s\n", t.RepoOwner, t.RepoName))
	}
	if t.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", t.Description))
	}
	if t.Branch != "" {
		sb.WriteString(fmt.Sprintf("Branch: %s\n", t.Branch))
	}
	if t.PRUrl != "" {
		sb.WriteString(fmt.Sprintf("PR: %s\n", t.PRUrl))
	}
	if t.Error != "" {
		sb.WriteString(fmt.Sprintf("Error: %s\n", t.Error))
	}
	sb.WriteString(fmt.Sprintf("Created: %s\n", t.CreatedAt.Format("2006-01-02 15:04 UTC")))
	sb.WriteString(fmt.Sprintf("Updated: %s", t.UpdatedAt.Format("2006-01-02 15:04 UTC")))
	return sb.String()
}

func parseID(args []string) (int64, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("usage requires a task ID")
	}
	id, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid task ID %q", args[1])
	}
	return id, nil
}
