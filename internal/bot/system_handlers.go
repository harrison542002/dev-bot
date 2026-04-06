package bot

import (
	"context"
	"fmt"
	"runtime"

	"devbot/internal/store"
)

func handleStatus(ctx context.Context, b *Bot, notify func(string)) {
	tasks, err := b.taskSvc.List(ctx)
	if err != nil {
		notify(fmt.Sprintf("Error fetching tasks: %v", err))
		return
	}

	counts := map[store.Status]int{}
	for _, t := range tasks {
		counts[t.Status]++
	}

	notify(fmt.Sprintf(
		"DevBot Status\n\nTasks:\n  TODO: %d\n  IN_PROGRESS: %d\n  IN_REVIEW: %d\n  DONE: %d\n  BLOCKED: %d\n  FAILED: %d\n\nGo goroutines: %d\nGitHub: %s/%s\nClaude model: %s",
		counts[store.StatusTodo],
		counts[store.StatusInProgress],
		counts[store.StatusInReview],
		counts[store.StatusDone],
		counts[store.StatusBlocked],
		counts[store.StatusFailed],
		runtime.NumGoroutine(),
		b.cfg.GitHub.Owner,
		b.cfg.GitHub.Repo,
		b.cfg.Claude.Model,
	))
}

func handleHelp(notify func(string)) {
	notify(`DevBot — AI-powered task & PR agent

Task Management:
  /task add <description>     Create a new task
  /task list                  Show all tasks
  /task do <id>               Start agent work on a task
  /task done <id>             Mark task complete (after merging PR)
  /task block <id> <reason>   Block a task with a reason
  /task show <id>             Show full task details

PR & Review:
  /pr <id>                    Show PR link and status
  /pr diff <id>               Show abbreviated diff
  /pr explain <id>            Explain changes in plain English
  /pr tests <id>              List tests added/modified
  /pr retry <id>              Discard branch and start over

System:
  /status                     Show agent health and task counts
  /help                       Show this message

Workflow:
  1. /task add "describe the work"
  2. /task do <id>  → agent writes code, opens PR
  3. Review PR on GitHub, merge when happy
  4. /task done <id>`)
}
