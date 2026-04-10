package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"devbot/internal/store"
)

func handleTask(ctx context.Context, b *Bot, chatID int64, args []string, notify func(string)) {
	if len(args) == 0 {
		notify("Usage: /task add|list|do|done|block|show")
		return
	}

	switch args[0] {
	case "add":
		if len(args) < 2 {
			if b.pool.IsMultiRepo() {
				notify("Usage: /task add [repo-name] <description>")
			} else {
				notify("Usage: /task add <description>")
			}
			return
		}
		// If multi-repo and the first arg matches a configured repo name/alias,
		// use that repo; otherwise treat all args as the task title.
		repoOwner, repoName, titleArgs := resolveRepo(b, args[1:])
		if len(titleArgs) == 0 {
			notify("Task title cannot be empty")
			return
		}
		title := strings.Join(titleArgs, " ")
		t, err := b.taskSvc.Add(ctx, title, repoOwner, repoName)
		if err != nil {
			notify(fmt.Sprintf("Error: %v", err))
			return
		}
		repoLabel := ""
		if b.pool.IsMultiRepo() {
			repoLabel = fmt.Sprintf(" [%s/%s]", t.RepoOwner, t.RepoName)
		}
		notify(fmt.Sprintf("Task %d created%s: %s\nStatus: TODO\n\nStart it with: /task do %d", t.ID, repoLabel, t.Title, t.ID))

	case "list":
		tasks, err := b.taskSvc.List(ctx)
		if err != nil {
			notify(fmt.Sprintf("Error: %v", err))
			return
		}
		if len(tasks) == 0 {
			notify("No tasks yet. Add one with /task add <description>")
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
		// Validate task exists and is in TODO state before launching goroutine
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

	default:
		notify("Unknown subcommand. Use: /task add|list|do|done|block|show")
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

// resolveRepo checks whether the first element of args is a known repo alias.
// If so, it returns the owner/name for that repo and the remaining args as the title.
// If not, it returns the default repo's owner/name and args unchanged.
func resolveRepo(b *Bot, args []string) (owner, name string, titleArgs []string) {
	if len(args) > 0 && b.pool.IsMultiRepo() {
		if c := b.pool.Lookup(args[0]); c != nil {
			return c.Owner(), c.Repo(), args[1:]
		}
	}
	def := b.pool.Default()
	if def != nil {
		return def.Owner(), def.Repo(), args
	}
	return "", "", args
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
