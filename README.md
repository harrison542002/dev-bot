# DevBot

A self-hosted AI agent that accepts tasks via Telegram, writes the code, opens a pull request, and reports back — so you can review and merge at your own pace.

## How it works

Send a task description to your Telegram bot. DevBot picks it up, asks Claude to write the implementation on a new feature branch, and opens a pull request on your GitHub repository. It then messages you with a plain-English summary and the PR link. You review on your own schedule and merge when satisfied — nothing ever lands on main automatically.

## Features

- **17 Telegram commands** covering task management, PR review, and system status
- **Claude-powered code generation** — structured JSON output with path-safety validation and prompt injection mitigation
- **Automatic branch naming** — prefix (`feat/`, `fix/`, `chore/`) inferred from the task description
- **PR review helpers** — ask Claude to explain a diff, list changed tests, or retry with a fresh branch
- **SQLite by default**, Postgres via `DATABASE_URL` for multi-user VPS deployments
- **No auto-merge** — every merge is a deliberate human action on GitHub
- **No open ports** — outbound Telegram polling only; zero inbound attack surface
- **Allowlist authentication** — commands from unknown Telegram user IDs are silently dropped

---

## Quick Start

### Prerequisites

| Requirement | Minimum version | Notes |
|-------------|----------------|-------|
| Go | 1.24+ | `go version` |
| gcc | any | Required for SQLite CGO driver |
| libsqlite3-dev | any | `apt install libsqlite3-dev` / `brew install sqlite` |
| git | 2.x | Must be in `$PATH` — used by the agent for cloning and pushing |

### 1. Clone and build

```bash
git clone https://github.com/harrison542002/dev-bot
cd dev-bot
go build ./cmd/devbot
```

### 2. Create a Telegram bot

1. Open Telegram and message [@BotFather](https://t.me/BotFather)
2. Send `/newbot` and follow the prompts
3. Copy the HTTP API token (looks like `123456789:ABCdef...`)
4. Find your own Telegram user ID: message [@userinfobot](https://t.me/userinfobot)

### 3. Generate a GitHub Personal Access Token

1. Go to **GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens**
2. Select your target repository
3. Grant **Contents: Read & Write** and **Pull requests: Read & Write**
4. Copy the token (starts with `ghp_` or `github_pat_`)

### 4. Get an Anthropic API key

Sign in at [console.anthropic.com](https://console.anthropic.com) and create an API key.

### 5. Configure

```bash
cp config.example.yaml config.yaml
chmod 600 config.yaml   # restrict to owner only
```

Edit `config.yaml` with your values (see [Configuration Reference](#configuration-reference) below).

### 6. Run

```bash
./devbot
# or without building first:
go run ./cmd/devbot
```

### 7. Verify

Send `/help` to your bot on Telegram. You should receive the full command reference within a few seconds.

---

## Configuration Reference

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `telegram.token` | Yes | — | Bot token from @BotFather |
| `telegram.allowed_user_ids` | Yes | — | List of Telegram user IDs permitted to send commands |
| `github.token` | Yes | — | GitHub PAT with `repo` (or Contents + Pull requests) scope |
| `github.owner` | Yes | — | GitHub username or organisation that owns the target repo |
| `github.repo` | Yes | — | Repository name (without owner prefix) |
| `github.base_branch` | No | `main` | Branch that PRs are opened against |
| `claude.api_key` | Yes | — | Anthropic API key |
| `claude.model` | No | `claude-sonnet-4-6` | Claude model ID — use `claude-opus-4-6` for more complex tasks |
| `database.path` | No | `./devbot.db` | SQLite file path; ignored when `DATABASE_URL` env var is set |

**Example:**

```yaml
telegram:
  token: "7123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"
  allowed_user_ids:
    - 123456789

github:
  token: "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  owner: "alice"
  repo: "my-project"
  base_branch: "main"

claude:
  api_key: "sk-ant-api03-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  model: "claude-sonnet-4-6"

database:
  path: "./devbot.db"
```

---

## Command Reference

### Task Management

| Command | What it does | Example |
|---------|--------------|---------|
| `/task add <description>` | Create a new task in TODO state | `/task add "Add rate limiting to /api/login"` |
| `/task list` | Show all tasks and their current status | `/task list` |
| `/task do <id>` | Trigger the agent to start work on a task | `/task do 14` |
| `/task done <id>` | Mark a task complete after merging the PR | `/task done 14` |
| `/task block <id> <reason>` | Block a task with a reason | `/task block 7 "Waiting for API spec"` |
| `/task show <id>` | Show full details for a single task | `/task show 14` |

### PR & Review

| Command | What it does | Example |
|---------|--------------|---------|
| `/pr <id>` | Show the PR link and status for a task | `/pr 14` |
| `/pr diff <id>` | Show an abbreviated diff in chat | `/pr diff 14` |
| `/pr explain <id>` | Ask Claude to explain the changes in plain English | `/pr explain 14` |
| `/pr tests <id>` | List the tests added or modified | `/pr tests 14` |
| `/pr retry <id>` | Discard the current branch and start again | `/pr retry 14` |

### System

| Command | What it does |
|---------|--------------|
| `/status` | Show agent health and task counts by status |
| `/help` | List all commands with short descriptions |

---

## Task Lifecycle

```
                   /task do <id>
  ┌───────┐  ─────────────────────►  ┌─────────────┐
  │  TODO │                          │ IN_PROGRESS │
  └───────┘  ◄─────────────────────  └─────────────┘
      ▲         error / /pr retry          │
      │                                    │ agent opens PR
      │                                    ▼
      │                           ┌──────────────┐
      │     /task done <id>       │  IN_REVIEW   │
      │  ◄────────────────────────┘──────────────┘
      │                                    │
      │                                    ▼
      │                            ┌──────────────┐
      │                            │     DONE     │
      │                            └──────────────┘
      │
      │  /task block <id> <reason>
      └──────────────────────────► BLOCKED (any state)
```

**States:**

| Status | Meaning |
|--------|---------|
| `TODO` | Ready to work on; accepts `/task do` |
| `IN_PROGRESS` | Agent is running — cloning, generating, pushing |
| `IN_REVIEW` | PR is open on GitHub; awaiting your review |
| `DONE` | PR merged and task manually marked complete |
| `BLOCKED` | Waiting on something external; reason stored in task |
| `FAILED` | Agent encountered an error; inspect with `/task show <id>`, retry with `/pr retry <id>` |

---

## Typical Workflow

```
1. /task add "Add pagination to the /users endpoint"
   → Task 1 created.

2. /task do 1
   → Agent starts: generating code, pushing branch, opening PR…
   → PR opened: https://github.com/alice/my-project/pull/42
     Added cursor-based pagination using a `next_cursor` query parameter…

3. (Review PR on GitHub — read the diff, run CI, leave comments)

4. (Merge on GitHub when satisfied)

5. /task done 1
   → Task 1 marked as DONE.
```

If something goes wrong:

```
/pr retry 1
→ Branch deleted, task reset to TODO, agent restarting…
```

---

## Deployment

### Local — single user, SQLite

```bash
./devbot -config config.yaml
```

SQLite creates `devbot.db` in the current directory automatically. No other infrastructure required.

### VPS — systemd service

Create `/etc/systemd/system/devbot.service`:

```ini
[Unit]
Description=DevBot AI agent
After=network.target

[Service]
Type=simple
User=devbot
WorkingDirectory=/opt/devbot
ExecStart=/opt/devbot/devbot -config /opt/devbot/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now devbot
sudo journalctl -u devbot -f
```

### VPS — Postgres

Set the `DATABASE_URL` environment variable and DevBot switches from SQLite to Postgres automatically:

```bash
export DATABASE_URL="postgres://user:pass@localhost/devbot?sslmode=require"
./devbot
```

Add to the systemd unit:

```ini
[Service]
Environment="DATABASE_URL=postgres://user:pass@localhost/devbot?sslmode=require"
```

### Multi-user access

Add additional Telegram user IDs to the allowlist in `config.yaml`:

```yaml
telegram:
  allowed_user_ids:
    - 123456789   # alice
    - 987654321   # bob
```

---

## Security Design

| Attack surface | DevBot | Why |
|---------------|--------|-----|
| Web UI / browser surface | None | No HTTP server at any port |
| Open inbound ports | None | Outbound polling to Telegram API only |
| Authentication | Telegram user ID allowlist | Commands from unknown IDs are silently dropped |
| Credentials stored | `config.yaml` only | Two tokens total; `chmod 600` on the file |
| GitHub token scope | `repo` only | Cannot delete repos, manage org, etc. |
| Auto-merge | Not possible | Token lacks merge permissions by design |
| Plugin system | None | No dynamic code loading, no community registry |
| Filesystem access | Per-task temp dir | Cloned to `os.MkdirTemp`, deleted with `defer os.RemoveAll` |
| Prompt injection | JSON schema + system prompt | AI output must match a strict schema; rogue instructions in repo content are rejected by the output parser |

---

## PR Review Checklist

Use this checklist when reviewing an agent-generated PR before merging:

- [ ] Does the diff make sense given the task description?
- [ ] Are there tests covering the new behaviour?
- [ ] Does it touch files unrelated to the task? (scope creep)
- [ ] Are there any hardcoded secrets or credentials?
- [ ] Does the branch name follow the convention (`feat/*`, `fix/*`, `chore/*`)?
- [ ] Is the PR description clear enough to understand without extra context?
- [ ] Ready to merge — no further changes needed?

If any item is **No**, comment on the PR explaining what needs fixing, then run `/pr retry <id>` to trigger a second attempt.
