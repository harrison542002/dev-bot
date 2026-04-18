package task

import (
	"context"
	"fmt"

	"github.com/harrison542002/dev-bot/internal/store"
)

type Service struct {
	store store.Store
}

func NewService(s store.Store) *Service {
	return &Service{store: s}
}

func (s *Service) Add(ctx context.Context, title, description, repoOwner, repoName string) (*store.Task, error) {
	if title == "" {
		return nil, fmt.Errorf("task title cannot be empty")
	}
	return s.store.CreateTask(ctx, title, description, repoOwner, repoName)
}

func (s *Service) Get(ctx context.Context, id int64) (*store.Task, error) {
	return s.store.GetTask(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]*store.Task, error) {
	return s.store.ListTasks(ctx)
}

func (s *Service) MarkDone(ctx context.Context, id int64) (*store.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.Status != store.StatusInReview && t.Status != store.StatusInProgress {
		return nil, fmt.Errorf("task %d is in %s state, expected IN_REVIEW or IN_PROGRESS", id, t.Status)
	}
	t.Status = store.StatusDone
	t.Error = ""
	return t, s.store.UpdateTask(ctx, t)
}

func (s *Service) Block(ctx context.Context, id int64, reason string) (*store.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = store.StatusBlocked
	t.Error = reason
	return t, s.store.UpdateTask(ctx, t)
}

func (s *Service) ResetToTodo(ctx context.Context, id int64) (*store.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = store.StatusTodo
	t.Branch = ""
	t.PRUrl = ""
	t.PRNumber = 0
	t.Error = ""
	return t, s.store.UpdateTask(ctx, t)
}

// RevertToTodo resets a task back to TODO status after an agent failure,
// preserving the error message so the user can inspect it with /task show.
// Unlike SetFailed, this keeps the task actionable — /task do can be used again.
func (s *Service) RevertToTodo(ctx context.Context, id int64, errMsg string) (*store.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = store.StatusTodo
	t.Branch = ""
	t.PRUrl = ""
	t.PRNumber = 0
	t.Error = errMsg
	return t, s.store.UpdateTask(ctx, t)
}

func (s *Service) SetInProgress(ctx context.Context, id int64) (*store.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.Status != store.StatusTodo {
		return nil, fmt.Errorf("task %d is in %s state, expected TODO", id, t.Status)
	}
	t.Status = store.StatusInProgress
	t.Error = ""
	return t, s.store.UpdateTask(ctx, t)
}

func (s *Service) SetFailed(ctx context.Context, id int64, errMsg string) (*store.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = store.StatusFailed
	t.Error = errMsg
	return t, s.store.UpdateTask(ctx, t)
}

// SetStatus transitions a task to any valid status. Fields that are no longer
// relevant to the new status (branch, PR, error) are cleared automatically.
func (s *Service) SetStatus(ctx context.Context, id int64, status store.Status) (*store.Task, error) {
	switch status {
	case store.StatusTodo, store.StatusInProgress, store.StatusInReview,
		store.StatusDone, store.StatusBlocked, store.StatusFailed:
	default:
		return nil, fmt.Errorf("unknown status %q — valid values: todo, in_progress, in_review, done, blocked, failed", status)
	}
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = status
	switch status {
	case store.StatusTodo:
		t.Branch, t.PRUrl, t.PRNumber, t.Error = "", "", 0, ""
	case store.StatusDone, store.StatusInProgress, store.StatusInReview:
		t.Error = ""
	}
	return t, s.store.UpdateTask(ctx, t)
}

func (s *Service) SetInReview(ctx context.Context, id int64, branch, prURL string, prNumber int) (*store.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = store.StatusInReview
	t.Branch = branch
	t.PRUrl = prURL
	t.PRNumber = prNumber
	t.Error = ""
	return t, s.store.UpdateTask(ctx, t)
}
