package entities

import "time"

type TaskStatus string

// Status is kept as an alias so the rest of the codebase can use the shorter
// domain-oriented name while TaskStatus remains backward compatible.
type Status = TaskStatus

const (
	StatusTodo       TaskStatus = "TODO"
	StatusInProgress TaskStatus = "IN_PROGRESS"
	StatusInReview   TaskStatus = "IN_REVIEW"
	StatusDone       TaskStatus = "DONE"
	StatusBlocked    TaskStatus = "BLOCKED"
	StatusFailed     TaskStatus = "FAILED"
)

type Task struct {
	ID          int64
	Title       string
	Description string
	Status      TaskStatus
	Branch      string
	PRUrl       string
	PRNumber    int
	RepoOwner   string
	RepoName    string
	Error       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
