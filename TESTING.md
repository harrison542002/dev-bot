# DevBot Feature Test Plan

End-to-end test scenarios for every command, state transition, and error path.
Run against a real config (bot + GitHub + AI credentials) unless marked **unit**.

---

## 1. Setup Checklist

Before running any scenario, verify:

- [ ] `config.yaml` has valid Telegram/Discord token and at least one allowed user ID
- [ ] GitHub token has Contents + Pull requests Read & Write
- [ ] AI provider is configured and reachable (`/status` returns no errors)
- [ ] Bot is running (`go run ./cmd/devbot` or `./devbot`)
- [ ] You can message the bot and receive `/help` back
- [ ] Test GitHub repo exists and the base branch (e.g. `main`) is present

For scheduler tests, also set:

```yaml
schedule:
  enabled: true
  timezone: "UTC"
  work_start: "00:00"
  work_end: "23:59"
  check_interval_minutes: 1
```

For budget tests, also set:

```yaml
budget:
  monthly_limit_usd: 0.01   # tiny limit so it trips immediately

local:
  base_url: "http://localhost:11434/v1"
  model: "llama3.2"          # must be pulled in Ollama
```

---

## 2. Task Management

### 2.1 `/task add` — basic

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/task add "Add hello-world endpoint"` | `Task 1 created: Add hello-world endpoint` + `Status: TODO` |
| 2 | `/task list` | Task 1 appears with status `TODO` |
| 3 | `/task show 1` | Full detail: title, status, created/updated timestamps |

### 2.2 `/task add` — empty title

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/task add` (no description) | Usage hint returned, no task created |
| 2 | `/task list` | Task count unchanged |

### 2.3 `/task add` — multi-repo routing

*Requires two repos in `github.repos` with names `backend` and `frontend`.*

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/task add backend "Add rate limit"` | Task created, repo label shows `[owner/backend]` |
| 2 | `/task add frontend "Fix login button"` | Task created, repo label shows `[owner/frontend]` |
| 3 | `/task add "Orphan task"` | Task routed to first repo (default) |
| 4 | `/task list` | Each task shows its repo label |

### 2.4 `/task list` — empty

| Step | Action | Expected |
|------|--------|----------|
| 1 | (fresh DB with no tasks) `/task list` | "No tasks yet. Add one with /task add <description>" |

### 2.5 `/task do` — happy path

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/task add "Add a README badge"` → note ID | Task N created |
| 2 | `/task do N` | "Starting agent for task N…" message, then agent progress |
| 3 | Wait for agent | "PR opened for task N: https://github.com/…/pull/…" |
| 4 | `/task show N` | Status `IN_REVIEW`, Branch and PR URL populated |
| 5 | Check GitHub | PR exists on correct branch with correct base branch |

### 2.6 `/task do` — not in TODO state

| Step | Action | Expected |
|------|--------|----------|
| 1 | Take a task that is `IN_REVIEW` | — |
| 2 | `/task do <id>` | "Task N is in IN_REVIEW state. Only TODO tasks can be started." |

### 2.7 `/task do` — invalid ID

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/task do abc` | "invalid task ID" |
| 2 | `/task do 99999` | "Task 99999 not found" |
| 3 | `/task do` (no id) | Usage hint |

### 2.8 `/task done`

| Step | Action | Expected |
|------|--------|----------|
| 1 | Have a task in `IN_REVIEW` | — |
| 2 | `/task done <id>` | "Task N marked as DONE: …" |
| 3 | `/task show <id>` | Status `DONE` |
| 4 | `/task done <id>` again | Error: wrong state |

### 2.9 `/task block`

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/task block 1 "Waiting for API spec"` | "Task 1 blocked: … Reason: Waiting for API spec" |
| 2 | `/task show 1` | Status `BLOCKED`, Error shows reason |
| 3 | `/task block 1` (no reason) | "Usage: /task block <id> <reason>" |

### 2.10 `/task show` — all fields

| Step | Action | Expected |
|------|--------|----------|
| 1 | Have a task in `IN_REVIEW` with PR | `/task show <id>` |
| 2 | — | Shows: title, status, repo, branch, PR URL, created, updated |
| 3 | Have a BLOCKED task | `/task show <id>` shows Error field |

---

## 3. PR Commands

### 3.1 `/pr <id>` — show PR

| Step | Action | Expected |
|------|--------|----------|
| 1 | Have task in `IN_REVIEW` | — |
| 2 | `/pr <id>` | PR URL, title, branch, status |

### 3.2 `/pr <id>` — no PR yet

| Step | Action | Expected |
|------|--------|----------|
| 1 | Have task in `TODO` | — |
| 2 | `/pr <id>` | "Task N has no PR yet (status: TODO)" |

### 3.3 `/pr diff <id>`

| Step | Action | Expected |
|------|--------|----------|
| 1 | Have task in `IN_REVIEW` | — |
| 2 | `/pr diff <id>` | Truncated unified diff in chat (≤ 3000 chars) |
| 3 | `/pr diff 99999` | "Task 99999 not found" |

### 3.4 `/pr explain <id>`

| Step | Action | Expected |
|------|--------|----------|
| 1 | Have task in `IN_REVIEW` | — |
| 2 | `/pr explain <id>` | 3–5 sentence plain-English explanation from AI |
| 3 | Empty diff scenario (no changes) | "(no diff available)" or graceful fallback |

### 3.5 `/pr tests <id>`

| Step | Action | Expected |
|------|--------|----------|
| 1 | Have a PR that modified test files | — |
| 2 | `/pr tests <id>` | Bulleted list of test files/functions changed |
| 3 | Have a PR with no test changes | "No tests were changed" (or similar) |

### 3.6 `/pr retry <id>`

| Step | Action | Expected |
|------|--------|----------|
| 1 | Have task in `IN_REVIEW` | — |
| 2 | `/pr retry <id>` | "Task N reset to TODO. Starting agent again…" |
| 3 | Check GitHub | Original branch deleted |
| 4 | `/task show <id>` | Branch and PR URL cleared; status `TODO` then `IN_PROGRESS` |
| 5 | Wait for agent | New PR opened on new branch |

### 3.7 `/pr retry` — no existing branch

| Step | Action | Expected |
|------|--------|----------|
| 1 | Have a `FAILED` task with no branch set | — |
| 2 | `/pr retry <id>` | Task reset to TODO, agent starts (no branch delete error) |

### 3.8 `/pr` — no args

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/pr` | "Usage: /pr <id> or /pr diff|explain|tests|retry <id>" |

---

## 4. System Commands

### 4.1 `/status`

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/status` with mix of task states | Shows counts per state: TODO, IN_PROGRESS, IN_REVIEW, DONE, BLOCKED, FAILED |
| 2 | Scheduler enabled, not paused | "Scheduler: active" |
| 3 | Scheduler disabled | "Scheduler: disabled" |
| 4 | Budget configured | Shows limit and active provider |
| 5 | Budget not configured | "Budget: disabled" |

### 4.2 `/help`

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/help` | Full command reference including all sections (Task, PR, Schedule, Budget, System) |
| 2 | Scheduler disabled | `/schedule` section still listed (static text) |

### 4.3 Authentication — unknown user

| Step | Action | Expected |
|------|--------|----------|
| 1 | Send any command from a user ID **not** in `allowed_user_ids` | No response (silently dropped) |
| 2 | Check logs | No error logged for the unknown user |

---

## 5. Auto-Scheduler

*Requires `schedule.enabled: true`, `timezone: "UTC"`, `work_start: "00:00"`, `work_end: "23:59"`, `check_interval_minutes: 1`.*

### 5.1 `/schedule` — status display

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/schedule` | Shows: Enabled, Paused, work window, interval, current state (ACTIVE/INACTIVE), agent running, TODO queue count |

### 5.2 `/schedule off` / `/schedule on`

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/schedule off` | "Scheduler paused…" |
| 2 | `/schedule` | Shows `Paused: Yes`, `Right now: PAUSED` |
| 3 | Add a TODO task and wait > check_interval | Agent does **not** auto-start |
| 4 | `/schedule on` | "Scheduler resumed…" |
| 5 | Wait up to check_interval | Agent auto-starts the TODO task |

### 5.3 `/schedule next`

| Step | Action | Expected |
|------|--------|----------|
| 1 | Add two tasks | — |
| 2 | `/schedule next` | Shows first (lowest ID) TODO task |
| 3 | Mark all tasks done | `/schedule next` → "No TODO tasks queued." |

### 5.4 Auto-start task

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/schedule off` first (clean state) | — |
| 2 | `/task add "Auto-start test"` | Task created |
| 3 | `/schedule on` | Scheduler resumes |
| 4 | Wait up to `check_interval_minutes` | Broadcast: "Auto-starting task N: Auto-start test" |
| 5 | Wait for agent | PR opened broadcast received |

### 5.5 Morning briefing

| Step | Action | Expected |
|------|--------|----------|
| 1 | Add 2 TODO tasks | — |
| 2 | Restart bot (resets `lastRunDate`) | — |
| 3 | Wait for first tick | Broadcast: "Good morning! Work day started. 2 task(s) in queue." |
| 4 | Wait for next tick | Morning briefing **not** sent again (once per day) |

### 5.6 Scheduler disabled in config

| Step | Action | Expected |
|------|--------|----------|
| 1 | Set `schedule.enabled: false`, restart | — |
| 2 | `/schedule` | "Auto-scheduler is not enabled. Set schedule.enabled: true…" |
| 3 | `/schedule on` | Same "not enabled" message |

### 5.7 Work window respected

| Step | Action | Expected |
|------|--------|----------|
| 1 | Set `work_start: "23:58"`, `work_end: "23:59"` (1-min window) | — |
| 2 | Add TODO task | — |
| 3 | Wait well outside the window | Agent does **not** auto-start |
| 4 | Set window to cover current UTC time | Agent auto-starts within one interval |

---

## 6. Budget Manager

*Requires `budget.monthly_limit_usd: 0.01` and a local model running.*

### 6.1 `/budget` — display

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/budget` | Shows: month, spent/limit, remaining, enforcement status, breakdown by provider |
| 2 | Budget not configured | "Budget tracking is not configured." |

### 6.2 `/budget pause` / `/budget resume`

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/budget pause` | "Budget enforcement paused. Commercial provider will be used regardless of spend." |
| 2 | `/budget` | Shows `PAUSED` enforcement |
| 3 | `/budget resume` | "Budget enforcement resumed." |
| 4 | `/budget` | Shows active enforcement |

### 6.3 Automatic provider switch

| Step | Action | Expected |
|------|--------|----------|
| 1 | Set limit to $0.01, run one task to exhaust budget | — |
| 2 | Broadcast received | "Budget limit of $0.01 reached… Switching to Local (model-name)…" |
| 3 | `/budget` | Active provider shown as local model |
| 4 | Run another task | Agent uses local model (check `/status` or logs) |
| 5 | `/budget pause` then run a task | Agent uses commercial provider |

### 6.4 `budget.monthly_limit_usd: 0` — tracking only

| Step | Action | Expected |
|------|--------|----------|
| 1 | Set limit to 0, run a task | — |
| 2 | `/budget` | Shows spend but no enforcement line / "no limit" |
| 3 | No provider switch | Commercial provider always used |

---

## 7. Agent Workflow (end-to-end)

### 7.1 Full happy path

| Step | Action | Expected |
|------|--------|----------|
| 1 | `/task add "Add a CONTRIBUTING.md file"` | Task 1: TODO |
| 2 | `/task do 1` | Agent starts |
| 3 | Bot sends progress messages | "Cloning repository…", "Running tool loop with Claude…" |
| 4 | Agent finishes | "PR opened: https://github.com/…/pull/N" + summary |
| 5 | `/task show 1` | Status: IN_REVIEW, Branch: feat/…-1, PR URL set |
| 6 | Visit GitHub | PR exists on correct base branch; diff contains CONTRIBUTING.md |
| 7 | `/pr explain 1` | Coherent explanation |
| 8 | Merge PR on GitHub | — |
| 9 | `/task done 1` | Status: DONE |

### 7.2 Agent error → reset to TODO

| Step | Action | Expected |
|------|--------|----------|
| 1 | Misconfigure GitHub token (wrong permissions) | — |
| 2 | `/task add "Test"` → `/task do 1` | Agent starts |
| 3 | Push/PR step fails | "Task 1 failed and was reset to TODO: …" |
| 4 | `/task show 1` | Status: TODO, Error field contains message |
| 5 | Fix token in config, restart | — |
| 6 | `/task do 1` | Agent retries successfully |

### 7.3 Path traversal blocked (unit)

Confirm the agent rejects malicious LLM output paths:
- `../../etc/passwd` → `applyChanges` returns "unsafe file path"
- `../outside-repo/file.go` → same rejection
- `valid/path/file.go` → accepted

### 7.4 Token redaction in errors

| Step | Action | Expected |
|------|--------|----------|
| 1 | Set invalid GitHub repo (404) | Agent fails on clone |
| 2 | Error message sent to user | GitHub PAT (`ghp_…`) **not** visible in the error |
| 3 | Check logs | Token replaced with `***` |

---

## 8. Configuration Edge Cases

### 8.1 Missing config file → setup wizard

| Step | Action | Expected |
|------|--------|----------|
| 1 | Run `./devbot` with no `config.yaml` | Interactive setup wizard launches |
| 2 | Complete wizard | `config.yaml` created, bot starts |

### 8.2 Invalid config fields

| Step | Action | Expected |
|------|--------|----------|
| 1 | Set `schedule.timezone: "Invalid/Zone"` | Startup error: "invalid schedule.timezone" |
| 2 | Set `schedule.work_start: "25:00"` | Startup error: "time out of range" |
| 3 | Set `schedule.work_start: "17:00"`, `work_end: "09:00"` | Startup error: "work_start must be before work_end" |

### 8.3 `DATABASE_URL` override

| Step | Action | Expected |
|------|--------|----------|
| 1 | Set `DATABASE_URL=postgres://…` env var | Logs: "using Postgres database" |
| 2 | Unset env var | Logs: "using SQLite database" |

### 8.4 `--version` flag

| Step | Action | Expected |
|------|--------|----------|
| 1 | `./devbot --version` | Prints `devbot dev` (or tag if built with `-ldflags`) and exits |

---

## 9. Multi-Platform

### 9.1 Telegram

| Step | Action | Expected |
|------|--------|----------|
| 1 | `bot.platform: "telegram"` (or unset) | Bot responds to `/task`, `/pr`, etc. |
| 2 | Send from allowed user | Commands execute normally |
| 3 | Send from unknown user | No response |

### 9.2 Discord

| Step | Action | Expected |
|------|--------|----------|
| 1 | `bot.platform: "discord"`, set token + allowed_user_ids | — |
| 2 | Type `!help` in a channel the bot can read | Full help text returned |
| 3 | `!task add "Discord test"` | Task created |
| 4 | `!task do <id>` | Agent runs, PR notification arrives in same channel |
| 5 | DM the bot with `!status` | Responds in DM |

### 9.3 Invalid platform

| Step | Action | Expected |
|------|--------|----------|
| 1 | `bot.platform: "slack"` | Startup error: "unknown bot.platform "slack"" |

---

## 10. State Machine — Transition Matrix

Verify every valid and invalid transition explicitly:

| From | Command | Expected To | Expected Error if Invalid |
|------|---------|-------------|--------------------------|
| `TODO` | `/task do` | `IN_PROGRESS` | — |
| `TODO` | `/task block <r>` | `BLOCKED` | — |
| `TODO` | `/task done` | — | "expected IN_REVIEW or IN_PROGRESS" |
| `IN_PROGRESS` | (agent success) | `IN_REVIEW` | — |
| `IN_PROGRESS` | (agent failure) | `TODO` | Error stored |
| `IN_PROGRESS` | `/task done` | `DONE` | — |
| `IN_REVIEW` | `/task done` | `DONE` | — |
| `IN_REVIEW` | `/pr retry` | `TODO` | — |
| `DONE` | `/task done` | — | "expected IN_REVIEW or IN_PROGRESS" |
| `BLOCKED` | `/task do` | — | "Only TODO tasks can be started" |
| `FAILED` | `/pr retry` | `TODO` | — |

---

## 11. Regression Checks

After any code change, verify these do not regress:

- [ ] `/help` lists all commands (task, pr, schedule, budget, system)
- [ ] `/status` shows correct counts after adding tasks in each state
- [ ] Agent temp directory is cleaned up after task completion (check `/tmp` for stale `devbot-*` dirs)
- [ ] `/pr retry` deletes the old GitHub branch before creating a new one
- [ ] Scheduler does not launch a second agent while one is already running (`agentBusy` lock)
- [ ] Budget notification is sent only once per calendar month, not on every over-budget request
- [ ] Unknown user IDs receive zero response (no message, no log entry with user content)
- [ ] `./devbot --version` exits cleanly with no panic
