package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/harrison542002/dev-bot/internal/store"
)

func handlePR(ctx context.Context, b *Bot, chatID int64, args []string, notify func(string)) {
	if len(args) == 0 {
		notify("Usage: /pr <id> or /pr diff|explain|tests|retry <id>")
		return
	}

	// Check if first arg is a subcommand or a task ID
	switch args[0] {
	case "diff":
		prSubcommand(ctx, b, args[1:], notify, func(t *store.Task) error {
			gh := b.pool.Get(t.RepoOwner, t.RepoName)
			if gh == nil {
				return fmt.Errorf("repo %s/%s is not in the current config", t.RepoOwner, t.RepoName)
			}
			diff, err := gh.GetPRDiff(ctx, t.PRNumber)
			if err != nil {
				return err
			}
			if diff == "" {
				notify("No diff available.")
				return nil
			}
			// Truncate to Telegram's message limit
			truncated := truncateDiff(diff, 3000)
			notify(fmt.Sprintf("Diff for PR #%d (%s):\n\n%s", t.PRNumber, t.Branch, truncated))
			return nil
		})

	case "explain":
		prSubcommand(ctx, b, args[1:], notify, func(t *store.Task) error {
			gh := b.pool.Get(t.RepoOwner, t.RepoName)
			if gh == nil {
				return fmt.Errorf("repo %s/%s is not in the current config", t.RepoOwner, t.RepoName)
			}
			diff, err := gh.GetPRDiff(ctx, t.PRNumber)
			if err != nil {
				return fmt.Errorf("fetch diff: %w", err)
			}
			notify("Asking Claude to explain the changes...")
			explanation, err := b.ag.ExplainDiff(ctx, diff)
			if err != nil {
				return err
			}
			notify(fmt.Sprintf("Explanation for task %d:\n\n%s", t.ID, explanation))
			return nil
		})

	case "tests":
		prSubcommand(ctx, b, args[1:], notify, func(t *store.Task) error {
			gh := b.pool.Get(t.RepoOwner, t.RepoName)
			if gh == nil {
				return fmt.Errorf("repo %s/%s is not in the current config", t.RepoOwner, t.RepoName)
			}
			diff, err := gh.GetPRDiff(ctx, t.PRNumber)
			if err != nil {
				return fmt.Errorf("fetch diff: %w", err)
			}
			notify("Analyzing tests in the diff...")
			result, err := b.ag.ListTests(ctx, diff)
			if err != nil {
				return err
			}
			notify(fmt.Sprintf("Tests in PR for task %d:\n\n%s", t.ID, result))
			return nil
		})

	case "retry":
		prSubcommand(ctx, b, args[1:], notify, func(t *store.Task) error {
			// Delete the branch on GitHub if it exists
			if t.Branch != "" {
				gh := b.pool.Get(t.RepoOwner, t.RepoName)
				if gh == nil {
					return fmt.Errorf("repo %s/%s is not in the current config; update the config or clear the task manually", t.RepoOwner, t.RepoName)
				}
				if err := gh.DeleteBranch(ctx, t.Branch); err != nil {
					// Log but don't fail — branch may already be gone
					notify(fmt.Sprintf("Warning: could not delete branch %q: %v", t.Branch, err))
				}
			}

			// Reset task to TODO
			resetTask, err := b.taskSvc.ResetToTodo(ctx, t.ID)
			if err != nil {
				return fmt.Errorf("reset task: %w", err)
			}
			notify(fmt.Sprintf("Task %d reset to TODO. Starting agent again...", resetTask.ID))
			go b.ag.Run(ctx, resetTask.ID, notify)
			return nil
		})

	default:
		// Treat first arg as task ID for /pr <id>
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			notify(fmt.Sprintf("Unknown subcommand %q. Use: /pr diff|explain|tests|retry <id>", args[0]))
			return
		}
		t, err := b.taskSvc.Get(ctx, id)
		if err != nil {
			notify(fmt.Sprintf("Task %d not found", id))
			return
		}
		if t.PRUrl == "" {
			notify(fmt.Sprintf("Task %d has no PR yet (status: %s)", id, t.Status))
			return
		}
		notify(fmt.Sprintf("PR for task %d: %s\n\nTitle: %s\nBranch: %s\nStatus: %s",
			t.ID, t.PRUrl, t.Title, t.Branch, t.Status))
	}
}

func prSubcommand(ctx context.Context, b *Bot, args []string, notify func(string), fn func(*store.Task) error) {
	if len(args) == 0 {
		notify("Please provide a task ID")
		return
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		notify(fmt.Sprintf("Invalid task ID %q", args[0]))
		return
	}
	t, err := b.taskSvc.Get(ctx, id)
	if err != nil {
		notify(fmt.Sprintf("Task %d not found", id))
		return
	}
	if t.PRNumber == 0 {
		notify(fmt.Sprintf("Task %d has no PR yet (status: %s)", id, t.Status))
		return
	}
	if err := fn(t); err != nil {
		notify(fmt.Sprintf("Error: %v", err))
	}
}

func truncateDiff(diff string, maxBytes int) string {
	if len(diff) <= maxBytes {
		return diff
	}
	lines := strings.Split(diff[:maxBytes], "\n")
	if len(lines) > 1 {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n") + "\n\n... (diff truncated)"
}
