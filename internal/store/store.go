package store

import (
	"context"
	"time"
)

type Status string

const (
	StatusTodo       Status = "TODO"
	StatusInProgress Status = "IN_PROGRESS"
	StatusInReview   Status = "IN_REVIEW"
	StatusDone       Status = "DONE"
	StatusBlocked    Status = "BLOCKED"
	StatusFailed     Status = "FAILED"
)

type Task struct {
	ID          int64
	Title       string
	Description string
	Status      Status
	Branch      string
	PRUrl       string
	PRNumber    int
	RepoOwner   string
	RepoName    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Error       string
}

// BudgetRecord is one row from budget_usage aggregated by provider.
type BudgetRecord struct {
	Provider     string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}

type Store interface {
	// Task operations
	CreateTask(ctx context.Context, title, repoOwner, repoName string) (*Task, error)
	GetTask(ctx context.Context, id int64) (*Task, error)
	ListTasks(ctx context.Context) ([]*Task, error)
	UpdateTask(ctx context.Context, t *Task) error

	// Budget operations
	// AddBudgetUsage records one API call's token consumption.
	// month is in "2006-01" format.
	AddBudgetUsage(ctx context.Context, month, provider string, inputTokens, outputTokens int64, costUSD float64) error
	// GetMonthlySpend returns total USD spent in the given month.
	GetMonthlySpend(ctx context.Context, month string) (float64, error)
	// GetMonthlyBreakdown returns per-provider cost totals for the given month.
	GetMonthlyBreakdown(ctx context.Context, month string) ([]BudgetRecord, error)

	Close() error
}
