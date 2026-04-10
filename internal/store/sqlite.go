package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type sqliteStore struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'TODO',
    branch      TEXT NOT NULL DEFAULT '',
    pr_url      TEXT NOT NULL DEFAULT '',
    pr_number   INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    error       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS budget_usage (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    month         TEXT NOT NULL,
    provider      TEXT NOT NULL,
    input_tokens  INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd      REAL NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS budget_usage_month ON budget_usage(month);
`

func NewSQLite(path string) (Store, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return &sqliteStore{db: db}, nil
}

func (s *sqliteStore) CreateTask(ctx context.Context, title string) (*Task, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO tasks (title, status, created_at, updated_at) VALUES (?, 'TODO', ?, ?)`,
		title, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetTask(ctx, id)
}

func (s *sqliteStore) GetTask(ctx context.Context, id int64) (*Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, description, status, branch, pr_url, pr_number, created_at, updated_at, error
		 FROM tasks WHERE id = ?`, id,
	)
	return scanTask(row)
}

func (s *sqliteStore) ListTasks(ctx context.Context) ([]*Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, description, status, branch, pr_url, pr_number, created_at, updated_at, error
		 FROM tasks ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *sqliteStore) UpdateTask(ctx context.Context, t *Task) error {
	t.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET title=?, description=?, status=?, branch=?, pr_url=?, pr_number=?,
		 updated_at=?, error=? WHERE id=?`,
		t.Title, t.Description, t.Status, t.Branch, t.PRUrl, t.PRNumber,
		t.UpdatedAt, t.Error, t.ID,
	)
	return err
}

func (s *sqliteStore) AddBudgetUsage(ctx context.Context, month, provider string, inputTokens, outputTokens int64, costUSD float64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO budget_usage (month, provider, input_tokens, output_tokens, cost_usd) VALUES (?, ?, ?, ?, ?)`,
		month, provider, inputTokens, outputTokens, costUSD,
	)
	return err
}

func (s *sqliteStore) GetMonthlySpend(ctx context.Context, month string) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(cost_usd), 0) FROM budget_usage WHERE month = ?`, month,
	).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

func (s *sqliteStore) GetMonthlyBreakdown(ctx context.Context, month string) ([]BudgetRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT provider, SUM(input_tokens), SUM(output_tokens), SUM(cost_usd)
		 FROM budget_usage WHERE month = ?
		 GROUP BY provider ORDER BY SUM(cost_usd) DESC`,
		month,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []BudgetRecord
	for rows.Next() {
		var r BudgetRecord
		if err := rows.Scan(&r.Provider, &r.InputTokens, &r.OutputTokens, &r.CostUSD); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *sqliteStore) Close() error {
	return s.db.Close()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTask(s scanner) (*Task, error) {
	var t Task
	var createdAt, updatedAt string
	err := s.Scan(
		&t.ID, &t.Title, &t.Description, &t.Status,
		&t.Branch, &t.PRUrl, &t.PRNumber,
		&createdAt, &updatedAt, &t.Error,
	)
	if err != nil {
		return nil, err
	}
	t.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	if t.CreatedAt.IsZero() {
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	}
	t.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", updatedAt)
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	}
	return &t, nil
}
