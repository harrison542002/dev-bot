package store

import (
	"context"

	"github.com/harrison542002/dev-bot/internal/entities"
)

type Store interface {
	// Task operations
	CreateTask(ctx context.Context, title, description, repoOwner, repoName string) (*entities.Task, error)
	GetTask(ctx context.Context, id int64) (*entities.Task, error)
	ListTasks(ctx context.Context) ([]*entities.Task, error)
	UpdateTask(ctx context.Context, t *entities.Task) error

	// Budget operations
	// AddBudgetUsage records one API call's token consumption.
	// month is in "2006-01" format.
	AddBudgetUsage(ctx context.Context, month, provider string, inputTokens, outputTokens int64, costUSD float64) error
	// GetMonthlySpend returns total USD spent in the given month.
	GetMonthlySpend(ctx context.Context, month string) (float64, error)
	// GetMonthlyBreakdown returns per-provider cost totals for the given month.
	GetMonthlyBreakdown(ctx context.Context, month string) ([]entities.BudgetRecord, error)

	Close() error
}
