package agent

import (
	"context"
)

// ExplainDiff asks the AI to explain a PR diff in plain English.
func (a *Agent) ExplainDiff(ctx context.Context, diff string) (string, error) {
	if diff == "" {
		return "(no diff available)", nil
	}
	text, _, err := a.llm.Complete(ctx,
		"You are a helpful code reviewer. Be concise and clear.",
		"Explain the following git diff in plain English, suitable for a Telegram message. Be concise (3-5 sentences).\n\n"+truncate(diff, 8000),
		1024,
	)
	return text, err
}

// ListTests asks the AI to list the test files and functions changed in a diff.
func (a *Agent) ListTests(ctx context.Context, diff string) (string, error) {
	if diff == "" {
		return "(no diff available)", nil
	}
	text, _, err := a.llm.Complete(ctx,
		"You are a helpful code reviewer. Be concise and clear.",
		"List the test files and test function names added or modified in the following diff. Format as a bulleted list. If no tests were changed, say so.\n\n"+truncate(diff, 8000),
		512,
	)
	return text, err
}
