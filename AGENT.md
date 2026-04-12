# DevBot — Agent Coding Standards

All code in this repository must follow these standards. They exist to keep the
codebase secure, predictable, and easy to change.

---

## 1. Effective Go

Follow https://go.dev/doc/effective_go in full. The rules below call out the
points most relevant to this project.

### Naming

- Package names are lowercase single words: `setup`, `store`, `agent`. No underscores, no mixedCaps.
- Exported identifiers use PascalCase; unexported use camelCase.
- Avoid redundant package prefixes: `store.Task` not `store.StoreTask`.
- Acronyms follow the surrounding case: `prURL`, `ghToken`, `apiKey`, `PRUrl` (exported).
- Short variable names (`c`, `t`, `r`) are acceptable only when the scope is a few lines; use descriptive names across function boundaries.
- Error variables: `err` for the first, `ferr`/`err2` when a second is needed in the same scope.

### Error handling

- Every error must be handled explicitly. Do not use `_` to discard errors unless you have documented why it is safe.
- Wrap errors with context: `fmt.Errorf("open config: %w", err)`. The message describes the operation, not the symptom.
- Error strings are lowercase and have no trailing punctuation: `"clone repo"` not `"Clone Repo."`.
- Return errors to the caller; use `log/slog` only at the outermost layer (main, goroutine entry points).

### Functions

- Functions do one thing. If the name contains "and", split it.
- Prefer returning an error over panicking. Panic only for programmer errors that represent an impossible state.
- Avoid named return values except when they meaningfully document the output (e.g., `func divide(a, b float64) (result float64, err error)`).

### Interfaces

- Define interfaces where they are *used*, not where types are defined.
- Keep interfaces small. A one-method interface is ideal; more than three methods is a signal to reconsider.
- Accept interfaces, return concrete types.

### Goroutines

- Every goroutine must have a way to stop. Pass a `context.Context` and respect cancellation.
- Never start a goroutine without knowing how it ends. Document the exit condition.
- Do not use goroutines when a plain function call suffices.

### Comments

- Write godoc comments only on exported symbols, and only when the name alone does not fully describe the behaviour.
- Do not comment what the code does; comment *why* it does it, when that is not obvious.
- Delete commented-out code before committing.

### Formatting

- All code is formatted with `gofmt`. No exceptions.
- Group imports: standard library, then external, then internal (`devbot/internal/...`). Separate groups with a blank line.

---

## 2. Security

### Credentials

- No secrets, tokens, or API keys in source code or committed files.
- `config.yaml` is written with mode `0600`. Never loosen this permission.
- Log messages must not contain tokens, keys, or passwords. Truncate or redact before logging.
- Error messages returned to users must not expose internal paths, stack traces, or credential values.

### Input validation

- Validate all input at system boundaries: user messages, config file values, AI output, GitHub API responses.
- AI (LLM) output is parsed through a strict JSON schema before any file operation is performed. Reject responses that do not conform; never execute free-form text as code or shell commands.
- GitHub repository paths, file paths from AI output, and branch names are sanitised before use. Reject paths with `..` components.

### Process execution

- Use `exec.Command` with explicit argument slices. Never pass user-controlled data to a shell (`/bin/sh -c`). This prevents shell injection.
- Do not set `cmd.Env` to include secrets; pass them only through arguments when the subprocess API requires it.

### File operations

- Temporary directories are created with `os.MkdirTemp` and removed with `defer os.RemoveAll`. Never leave artefacts on disk.
- Write new files with `0644`; write config files with `0600`.

### Authentication

- Bot commands from unknown user IDs are silently dropped. Never send an error reply to an unauthenticated user — that leaks bot presence.
- The allowlist check happens before any command parsing.

### GitHub token scope

- The PAT needs only *Contents* and *Pull requests* (Read & Write). Document this wherever the token is referenced. Never request broader scopes.

---

## 3. Performance

### Context propagation

- Every function that performs I/O (network, disk, database) accepts a `context.Context` as its first parameter and passes it to all downstream calls.
- Do not use `context.Background()` inside library code; only in `main` and goroutine entry points.

### Allocations

- Avoid allocating inside loops when a pre-allocated slice or `strings.Builder` can be reused.
- Use `fmt.Fprintf(w, ...)` instead of building intermediate strings with `fmt.Sprintf` when writing to an `io.Writer`.
- Profile before optimising. The bottleneck is almost always an LLM API call or a git clone, not Go allocations.

### Database

- Reuse the single `store.Store` connection pool. Never open a second connection.
- Use `context`-aware query methods (`QueryRowContext`, `ExecContext`). This allows queries to be cancelled when the user cancels a task.

### Goroutines

- One agent task runs at a time (enforced by status check before launch). Do not add concurrency without a documented reason and a measured benefit.
- The scheduler ticker fires every N minutes; the handler must complete or hand off work before the next tick. If the agent is busy, skip the tick — do not queue work.

---

## 4. Project Conventions

### Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat:     new user-visible feature
fix:      bug fix
refactor: code restructure with no behaviour change
docs:     documentation only
chore:    build, config, dependency, tooling
add:      new file, asset, or configuration not covered by feat
```

- Subject line: imperative mood, lowercase after the prefix, no trailing period, ≤72 characters.
- Body: explain *why*, not *what*. Reference the problem the change solves.

### Branch names

```
feat/short-description
fix/what-is-broken
refactor/what-is-changing
docs/what-is-documented
chore/what-is-updated
```

No auto-generated names. Names must be readable and descriptive.

### Logging

- Use `log/slog` exclusively. No `fmt.Println`, `log.Printf`, or third-party loggers.
- Log levels:
  - `slog.Info` — startup events, successful major operations.
  - `slog.Warn` — recoverable errors, non-fatal anomalies.
  - `slog.Error` — failures that prevented an operation from completing.
- Structured fields over format strings: `slog.Error("clone failed", "repo", repo, "err", err)`.

### Package layout

```
cmd/devbot/         entry point and wiring only — no business logic
internal/agent/     AI workflow: clone → generate → commit → PR
internal/bot/       message routing and command handlers
internal/budget/    spend tracking and provider switching
internal/config/    config loading and validation
internal/github/    GitHub API client and pool
internal/llm/       LLM provider adapters and factory
internal/scheduler/ auto-scheduler
internal/setup/     first-run interactive wizard
internal/store/     persistence interface + SQLite/Postgres implementations
internal/task/      task state machine
```

Keep packages focused. If a new concern does not clearly belong in an existing package, create a new one rather than expanding an existing one beyond its stated purpose.

### Testing

- Use table-driven tests (`[]struct{ name, input, want string }`).
- Test function names: `TestFunctionName_Scenario` (e.g. `TestGet_UnknownRepo`).
- Do not write tests that depend on wall-clock time, network access, or the file system unless the test is explicitly an integration test and marked with `t.Skip` under `-short`.
- Prefer testing behaviour (what the function returns or does) over implementation (which internal functions it calls).
