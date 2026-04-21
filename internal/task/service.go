package task

import (
	"context"
	"fmt"

	"github.com/harrison542002/dev-bot/internal/entities"
	"github.com/harrison542002/dev-bot/internal/store"
)

type Service struct {
	store store.Store
}

func NewService(s store.Store) *Service {
	return &Service{store: s}
}

func (s *Service) Add(ctx context.Context, title, description, repoOwner, repoName string) (*entities.Task, error) {
	if title == "" {
		return nil, fmt.Errorf("task title cannot be empty")
	}
	return s.store.CreateTask(ctx, title, description, repoOwner, repoName)
}

func (s *Service) Get(ctx context.Context, id int64) (*entities.Task, error) {
	return s.store.GetTask(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]*entities.Task, error) {
	return s.store.ListTasks(ctx)
}

func (s *Service) MarkDone(ctx context.Context, id int64) (*entities.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.Status != entities.StatusInReview && t.Status != entities.StatusInProgress {
		return nil, fmt.Errorf("task %d is in %s state, expected IN_REVIEW or IN_PROGRESS", id, t.Status)
	}
	t.Status = entities.StatusDone
	t.Error = ""
	return t, s.store.UpdateTask(ctx, t)
}

func (s *Service) Block(ctx context.Context, id int64, reason string) (*entities.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = entities.StatusBlocked
	t.Error = reason
	return t, s.store.UpdateTask(ctx, t)
}

func (s *Service) ResetToTodo(ctx context.Context, id int64) (*entities.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = entities.StatusTodo
	t.Branch = ""
	t.PRUrl = ""
	t.PRNumber = 0
	t.Error = ""
	return t, s.store.UpdateTask(ctx, t)
}

// RevertToTodo resets a task back to TODO status after an agent failure,
// preserving the error message so the user can inspect it with /task show.
func (s *Service) RevertToTodo(ctx context.Context, id int64, errMsg string) (*entities.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = entities.StatusTodo
	t.Branch = ""
	t.PRUrl = ""
	t.PRNumber = 0
	t.Error = errMsg
	return t, s.store.UpdateTask(ctx, t)
}

func (s *Service) SetInProgress(ctx context.Context, id int64) (*entities.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.Status != entities.StatusTodo {
		return nil, fmt.Errorf("task %d is in %s state, expected TODO", id, t.Status)
	}
	t.Status = entities.StatusInProgress
	t.Error = ""
	return t, s.store.UpdateTask(ctx, t)
}

func (s *Service) SetFailed(ctx context.Context, id int64, errMsg string) (*entities.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = entities.StatusFailed
	t.Error = errMsg
	return t, s.store.UpdateTask(ctx, t)
}

func (s *Service) SetStatus(ctx context.Context, id int64, status entities.TaskStatus) (*entities.Task, error) {
	switch status {
	case entities.StatusTodo, entities.StatusInProgress, entities.StatusInReview,
		entities.StatusDone, entities.StatusBlocked, entities.StatusFailed:
	default:
		return nil, fmt.Errorf("unknown status %q — valid values: todo, in_progress, in_review, done, blocked, failed", status)
	}
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = status
	switch status {
	case entities.StatusTodo:
		t.Branch, t.PRUrl, t.PRNumber, t.Error = "", "", 0, ""
	case entities.StatusDone, entities.StatusInProgress, entities.StatusInReview:
		t.Error = ""
	}
	return t, s.store.UpdateTask(ctx, t)
}

func (s *Service) SetInReview(ctx context.Context, id int64, branch, prURL string, prNumber int) (*entities.Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Status = entities.StatusInReview
	t.Branch = branch
	t.PRUrl = prURL
	t.PRNumber = prNumber
	t.Error = ""
	return t, s.store.UpdateTask(ctx, t)
}
