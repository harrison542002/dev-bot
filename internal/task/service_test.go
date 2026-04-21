package task

import (
	"context"
	"errors"
	"testing"

	"github.com/harrison542002/dev-bot/internal/entities"
	"github.com/harrison542002/dev-bot/internal/store"
)

type fakeStore struct {
	tasks map[int64]*entities.Task
}

func newFakeStore(tasks ...*entities.Task) *fakeStore {
	fs := &fakeStore{tasks: make(map[int64]*entities.Task, len(tasks))}
	for _, task := range tasks {
		cloned := *task
		fs.tasks[task.ID] = &cloned
	}
	return fs
}

func (f *fakeStore) CreateTask(ctx context.Context, title, description, repoOwner, repoName string) (*entities.Task, error) {
	id := int64(len(f.tasks) + 1)
	task := &entities.Task{
		ID:          id,
		Title:       title,
		Description: description,
		RepoOwner:   repoOwner,
		RepoName:    repoName,
		Status:      entities.StatusTodo,
	}
	f.tasks[id] = task
	return task, nil
}

func (f *fakeStore) GetTask(ctx context.Context, id int64) (*entities.Task, error) {
	task, ok := f.tasks[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return task, nil
}

func (f *fakeStore) ListTasks(ctx context.Context) ([]*entities.Task, error) {
	out := make([]*entities.Task, 0, len(f.tasks))
	for _, task := range f.tasks {
		out = append(out, task)
	}
	return out, nil
}

func (f *fakeStore) UpdateTask(ctx context.Context, task *entities.Task) error {
	f.tasks[task.ID] = task
	return nil
}

func (f *fakeStore) AddBudgetUsage(ctx context.Context, month, provider string, inputTokens, outputTokens int64, costUSD float64) error {
	return nil
}

func (f *fakeStore) GetMonthlySpend(ctx context.Context, month string) (float64, error) {
	return 0, nil
}

func (f *fakeStore) GetMonthlyBreakdown(ctx context.Context, month string) ([]entities.BudgetRecord, error) {
	return nil, nil
}

func (f *fakeStore) Close() error {
	return nil
}

var _ store.Store = (*fakeStore)(nil)

func TestMarkDoneRequiresReviewOrInProgress(t *testing.T) {
	t.Parallel()

	svc := NewService(newFakeStore(&entities.Task{
		ID:     1,
		Title:  "Example",
		Status: entities.StatusTodo,
	}))

	if _, err := svc.MarkDone(context.Background(), 1); err == nil {
		t.Fatal("expected error when marking TODO task as done")
	}
}

func TestResetToTodoClearsWorkflowFields(t *testing.T) {
	t.Parallel()

	svc := NewService(newFakeStore(&entities.Task{
		ID:       1,
		Title:    "Example",
		Status:   entities.StatusInReview,
		Branch:   "feat/example-1",
		PRUrl:    "https://example.com/pr/1",
		PRNumber: 1,
		Error:    "boom",
	}))

	task, err := svc.ResetToTodo(context.Background(), 1)
	if err != nil {
		t.Fatalf("ResetToTodo returned error: %v", err)
	}

	if task.Status != entities.StatusTodo {
		t.Fatalf("expected status TODO, got %s", task.Status)
	}
	if task.Branch != "" || task.PRUrl != "" || task.PRNumber != 0 || task.Error != "" {
		t.Fatalf("expected workflow fields cleared, got %+v", task)
	}
}

func TestSetStatusTodoClearsWorkflowFields(t *testing.T) {
	t.Parallel()

	svc := NewService(newFakeStore(&entities.Task{
		ID:       1,
		Title:    "Example",
		Status:   entities.StatusBlocked,
		Branch:   "feat/example-1",
		PRUrl:    "https://example.com/pr/1",
		PRNumber: 1,
		Error:    "blocked",
	}))

	task, err := svc.SetStatus(context.Background(), 1, entities.StatusTodo)
	if err != nil {
		t.Fatalf("SetStatus returned error: %v", err)
	}

	if task.Branch != "" || task.PRUrl != "" || task.PRNumber != 0 || task.Error != "" {
		t.Fatalf("expected TODO transition to clear workflow fields, got %+v", task)
	}
}

func TestSetInReviewPopulatesPRFields(t *testing.T) {
	t.Parallel()

	svc := NewService(newFakeStore(&entities.Task{
		ID:     1,
		Title:  "Example",
		Status: entities.StatusInProgress,
	}))

	task, err := svc.SetInReview(context.Background(), 1, "feat/example-1", "https://example.com/pr/1", 1)
	if err != nil {
		t.Fatalf("SetInReview returned error: %v", err)
	}

	if task.Status != entities.StatusInReview {
		t.Fatalf("expected status IN_REVIEW, got %s", task.Status)
	}
	if task.Branch != "feat/example-1" || task.PRUrl != "https://example.com/pr/1" || task.PRNumber != 1 {
		t.Fatalf("expected PR fields to be populated, got %+v", task)
	}
}
