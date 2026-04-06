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
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Error       string
}

type Store interface {
	CreateTask(ctx context.Context, title string) (*Task, error)
	GetTask(ctx context.Context, id int64) (*Task, error)
	ListTasks(ctx context.Context) ([]*Task, error)
	UpdateTask(ctx context.Context, t *Task) error
	Close() error
}
