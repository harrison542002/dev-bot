package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type postgresStore struct {
	db *sql.DB
}

const postgresSchema = `
CREATE TABLE IF NOT EXISTS tasks (
    id          SERIAL PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'TODO',
    branch      TEXT NOT NULL DEFAULT '',
    pr_url      TEXT NOT NULL DEFAULT '',
    pr_number   INTEGER NOT NULL DEFAULT 0,
    repo_owner  TEXT NOT NULL DEFAULT '',
    repo_name   TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    error       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS budget_usage (
    id            SERIAL PRIMARY KEY,
    month         TEXT NOT NULL,
    provider      TEXT NOT NULL,
    input_tokens  BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cost_usd      DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS budget_usage_month ON budget_usage(month);
`

var postgresMigrations = []string{
	`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS repo_owner TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS repo_name  TEXT NOT NULL DEFAULT ''`,
}

func NewPostgres(dsn string) (Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if _, err := db.Exec(postgresSchema); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	for _, m := range postgresMigrations {
		if _, err := db.Exec(m); err != nil {
			return nil, fmt.Errorf("migration %q: %w", m, err)
		}
	}
	return &postgresStore{db: db}, nil
}

func (s *postgresStore) CreateTask(ctx context.Context, title, repoOwner, repoName string) (*Task, error) {
	now := time.Now().UTC()
	var id int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO tasks (title, status, repo_owner, repo_name, created_at, updated_at) VALUES ($1, 'TODO', $2, $3, $4, $5) RETURNING id`,
		title, repoOwner, repoName, now, now,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}
	return s.GetTask(ctx, id)
}

func (s *postgresStore) GetTask(ctx context.Context, id int64) (*Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, description, status, branch, pr_url, pr_number, repo_owner, repo_name, created_at, updated_at, error
		 FROM tasks WHERE id = $1`, id,
	)
	return scanPostgresTask(row)
}

func (s *postgresStore) ListTasks(ctx context.Context) ([]*Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, description, status, branch, pr_url, pr_number, repo_owner, repo_name, created_at, updated_at, error
		 FROM tasks ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := scanPostgresTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *postgresStore) UpdateTask(ctx context.Context, t *Task) error {
	t.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET title=$1, description=$2, status=$3, branch=$4, pr_url=$5, pr_number=$6,
		 repo_owner=$7, repo_name=$8, updated_at=$9, error=$10 WHERE id=$11`,
		t.Title, t.Description, t.Status, t.Branch, t.PRUrl, t.PRNumber,
		t.RepoOwner, t.RepoName, t.UpdatedAt, t.Error, t.ID,
	)
	return err
}

func (s *postgresStore) AddBudgetUsage(ctx context.Context, month, provider string, inputTokens, outputTokens int64, costUSD float64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO budget_usage (month, provider, input_tokens, output_tokens, cost_usd) VALUES ($1, $2, $3, $4, $5)`,
		month, provider, inputTokens, outputTokens, costUSD,
	)
	return err
}

func (s *postgresStore) GetMonthlySpend(ctx context.Context, month string) (float64, error) {
	var total float64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(cost_usd), 0) FROM budget_usage WHERE month = $1`, month,
	).Scan(&total)
	return total, err
}

func (s *postgresStore) GetMonthlyBreakdown(ctx context.Context, month string) ([]BudgetRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT provider, SUM(input_tokens), SUM(output_tokens), SUM(cost_usd)
		 FROM budget_usage WHERE month = $1
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

func (s *postgresStore) Close() error {
	return s.db.Close()
}

func scanPostgresTask(s scanner) (*Task, error) {
	var t Task
	err := s.Scan(
		&t.ID, &t.Title, &t.Description, &t.Status,
		&t.Branch, &t.PRUrl, &t.PRNumber,
		&t.RepoOwner, &t.RepoName,
		&t.CreatedAt, &t.UpdatedAt, &t.Error,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
