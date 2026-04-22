package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/harrison542002/dev-bot/internal/llm"
)

// writeOnlyTools returns the subset of tools available after the read limit is
func writeOnlyTools() []llm.Tool {
	all := agentTools()
	var out []llm.Tool
	for _, t := range all {
		switch t.Name {
		case "write_file", "delete_file", "finish_task":
			out = append(out, t)
		}
	}
	return out
}

// agentTools returns the tool definitions sent to the model each turn.
func agentTools() []llm.Tool {
	return []llm.Tool{
		{
			Name:        "read_file",
			Description: "Read the full contents of a file in the repository.",
			Parameters: llm.ToolParameters{
				Properties: map[string]llm.ToolProperty{
					"path": {Type: "string", Description: "File path relative to the repository root"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "list_directory",
			Description: "List files and subdirectories in a directory. Use '.' for the repository root.",
			Parameters: llm.ToolParameters{
				Properties: map[string]llm.ToolProperty{
					"path": {Type: "string", Description: "Directory path relative to the repository root (use '.' for root)"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "search_code",
			Description: "Search for a text pattern across all source files. Returns matching file paths and lines.",
			Parameters: llm.ToolParameters{
				Properties: map[string]llm.ToolProperty{
					"pattern": {Type: "string", Description: "Text or regular expression to search for"},
				},
				Required: []string{"pattern"},
			},
		},
		{
			Name:        "write_file",
			Description: "Create or completely overwrite a file. Intermediate directories are created automatically.",
			Parameters: llm.ToolParameters{
				Properties: map[string]llm.ToolProperty{
					"path":    {Type: "string", Description: "File path relative to the repository root"},
					"content": {Type: "string", Description: "Complete new file content"},
				},
				Required: []string{"path", "content"},
			},
		},
		{
			Name:        "delete_file",
			Description: "Delete a file from the repository.",
			Parameters: llm.ToolParameters{
				Properties: map[string]llm.ToolProperty{
					"path": {Type: "string", Description: "File path relative to the repository root"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name: "finish_task",
			Description: "Signal that all code changes are complete. " +
				"Call this ONLY when every required file has been written and you are ready to commit.",
			Parameters: llm.ToolParameters{
				Properties: map[string]llm.ToolProperty{
					"branch_prefix": {Type: "string", Description: "feat, fix, or chore"},
					"pr_title":      {Type: "string", Description: "Pull request title — imperative mood, max 72 chars"},
					"pr_body":       {Type: "string", Description: "Pull request description in markdown"},
					"summary":       {Type: "string", Description: "2-3 sentence plain-English summary of what was changed"},
				},
				Required: []string{"branch_prefix", "pr_title", "pr_body", "summary"},
			},
		},
	}
}

// taskResult holds the metadata returned by the finish_task tool call.
type taskResult struct {
	BranchPrefix string
	PRTitle      string
	PRBody       string
	Summary      string
}

type ToolExecutor struct {
	workDir string
	result  *taskResult
}

func (e *ToolExecutor) run(ctx context.Context, call llm.ToolUse) llm.ToolResult {
	content, err := e.dispatch(ctx, call)
	if err != nil {
		return llm.ToolResult{ToolUseID: call.ID, Content: "Error: " + err.Error(), IsError: true}
	}
	return llm.ToolResult{ToolUseID: call.ID, Content: content}
}

func (e *ToolExecutor) dispatch(ctx context.Context, call llm.ToolUse) (string, error) {
	switch call.Name {
	case "read_file":
		return e.readFile(call.Input)
	case "list_directory":
		return e.listDirectory(call.Input)
	case "search_code":
		return e.searchCode(ctx, call.Input)
	case "write_file":
		return e.writeFile(call.Input)
	case "delete_file":
		return e.deleteFile(call.Input)
	case "finish_task":
		return e.finishTask(call.Input)
	default:
		return "", fmt.Errorf("unknown tool %q", call.Name)
	}
}

func (e *ToolExecutor) readFile(args map[string]any) (string, error) {
	path, err := stringArg(args, "path")
	if err != nil {
		return "", err
	}
	full, err := e.safePath(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}
	return string(data), nil
}

func (e *ToolExecutor) listDirectory(args map[string]any) (string, error) {
	path, err := stringArg(args, "path")
	if err != nil {
		return "", err
	}
	full, err := e.safePath(path)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return "", fmt.Errorf("list %q: %w", path, err)
	}
	var sb strings.Builder
	for _, entry := range entries {
		if entry.IsDir() {
			sb.WriteString(entry.Name() + "/\n")
		} else {
			sb.WriteString(entry.Name() + "\n")
		}
	}
	return sb.String(), nil
}

func (e *ToolExecutor) searchCode(ctx context.Context, args map[string]any) (string, error) {
	pattern, err := stringArg(args, "pattern")
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "grep", "-r", "-n", "--include=*.go",
		"--include=*.ts", "--include=*.js", "--include=*.py",
		"--include=*.java", "--include=*.rb", "--include=*.rs",
		"-l", pattern, e.workDir)
	filesOut, _ := cmd.Output()

	if len(bytes.TrimSpace(filesOut)) == 0 {
		// Try a broader search
		cmd2 := exec.CommandContext(ctx, "grep", "-r", "-n", "-l", pattern, e.workDir)
		filesOut, _ = cmd2.Output()
	}

	if len(bytes.TrimSpace(filesOut)) == 0 {
		return "No matches found.", nil
	}

	// Now get the actual matching lines (limit output)
	cmd3 := exec.CommandContext(ctx, "grep", "-r", "-n", "--max-count=5", pattern, e.workDir)
	out, _ := cmd3.Output()
	result := strings.ReplaceAll(string(out), e.workDir+string(filepath.Separator), "")
	if len(result) > 4000 {
		result = result[:4000] + "\n... (truncated)"
	}
	return result, nil
}

func (e *ToolExecutor) writeFile(args map[string]any) (string, error) {
	path, err := stringArg(args, "path")
	if err != nil {
		return "", err
	}
	content, err := stringArg(args, "content")
	if err != nil {
		return "", err
	}
	full, err := e.safePath(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return "", fmt.Errorf("mkdir for %q: %w", path, err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write %q: %w", path, err)
	}
	return fmt.Sprintf("wrote %s (%d bytes)", path, len(content)), nil
}

func (e *ToolExecutor) deleteFile(args map[string]any) (string, error) {
	path, err := stringArg(args, "path")
	if err != nil {
		return "", err
	}
	full, err := e.safePath(path)
	if err != nil {
		return "", err
	}
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("delete %q: %w", path, err)
	}
	return "deleted " + path, nil
}

func (e *ToolExecutor) finishTask(args map[string]any) (string, error) {
	prefix, err := stringArg(args, "branch_prefix")
	if err != nil {
		return "", err
	}
	if prefix == "" {
		return "", fmt.Errorf("branch_prefix is required and cannot be empty")
	}
	title, err := stringArg(args, "pr_title")
	if err != nil {
		return "", err
	}
	if title == "" {
		return "", fmt.Errorf("pr_title is required and cannot be empty")
	}
	body, _ := stringArg(args, "pr_body")
	summary, _ := stringArg(args, "summary")

	e.result = &taskResult{
		BranchPrefix: prefix,
		PRTitle:      title,
		PRBody:       body,
		Summary:      summary,
	}
	return "task marked complete", nil
}

// safePath resolves a relative path inside workDir and rejects both lexical
// path traversal and symlink-based escapes (e.g. a checked-in symlink that
// points outside the cloned workspace).
func (e *ToolExecutor) safePath(rel string) (string, error) {
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("path %q contains '..' which is not allowed", rel)
	}
	full := filepath.Join(e.workDir, filepath.FromSlash(rel))
	// Lexical prefix check — fast rejection of obvious escapes.
	if !strings.HasPrefix(full, e.workDir+string(filepath.Separator)) && full != e.workDir {
		return "", fmt.Errorf("path %q escapes repository root", rel)
	}

	// Symlink-aware check: walk each path component from workDir to the
	// target, resolving symlinks at every step. This catches symlinks like
	// "repo/link -> /etc" before os.ReadFile/os.WriteFile follows them.
	realWork, err := filepath.EvalSymlinks(e.workDir)
	if err != nil {
		realWork = e.workDir
	}
	realWorkSlash := realWork + string(filepath.Separator)

	relFromWork, _ := filepath.Rel(e.workDir, full)
	parts := strings.Split(filepath.ToSlash(relFromWork), "/")
	current := realWork
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		next := filepath.Join(current, part)
		resolved, err := filepath.EvalSymlinks(next)
		if err != nil {
			// Component doesn't exist yet (new file/dir being created).
			// All existing ancestors were clean — the path is safe.
			break
		}
		if !strings.HasPrefix(resolved, realWorkSlash) && resolved != realWork {
			return "", fmt.Errorf("path %q escapes repository root via symlink", rel)
		}
		current = resolved
	}
	return full, nil
}

func stringArg(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing argument %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("argument %q must be a string", key)
	}
	return s, nil
}
