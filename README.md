# DevBot

A self-hosted AI agent that accepts tasks via Telegram, writes the code, opens a pull request, and reports back — so you can review and merge at your own pace.

## How it works

Send a task description to your Telegram bot. DevBot picks it up, asks the AI to write the implementation on a new feature branch, and opens a pull request on your GitHub repository. It then messages you with a plain-English summary and the PR link. You review on your own schedule and merge when satisfied — nothing ever lands on main automatically.

## Features

- **Telegram or Discord** — choose your messaging platform; all commands work on both
- **AI-powered code generation** — structured JSON output with path-safety validation and prompt injection mitigation
- **Multi-repository** — configure any number of repos; tasks are routed to the right one by name alias
- **Automatic branch naming** — prefix (`feat/`, `fix/`, `chore/`) inferred from the task description
- **PR review helpers** — ask the AI to explain a diff, list changed tests, or retry with a fresh branch
- **SQLite by default**, Postgres via `DATABASE_URL` for multi-user VPS deployments
- **Auto-scheduler** — processes TODO tasks automatically during configurable work hours; weekday-only by default, with an opt-in weekend mode for always-on setups
- **No auto-merge** — every merge is a deliberate human action on GitHub
- **No open ports** — outbound Telegram polling only; zero inbound attack surface
- **Allowlist authentication** — commands from unknown Telegram user IDs are silently dropped

---

## Quick Start

### Prerequisites

| Requirement | Minimum version | Notes                                                          |
| ----------- | --------------- | -------------------------------------------------------------- |
| Go          | 1.25+           | `go version`                                                   |
| git         | 2.x             | Must be in `$PATH` — used by the agent for cloning and pushing |

### 1. Clone and build

```bash
git clone https://github.com/harrison542002/dev-bot
cd dev-bot
go build ./cmd/devbot
```

### 2. Choose a messaging platform — Telegram or Discord

**Option A: Telegram** (default)

1. Open Telegram and message [@BotFather](https://t.me/BotFather)
2. Send `/newbot` and follow the prompts
3. Copy the HTTP API token (looks like `123456789:ABCdef...`)
4. Find your own Telegram user ID: message [@userinfobot](https://t.me/userinfobot)

Set `bot.platform: "telegram"` in `config.yaml` (or leave it unset — telegram is the default).

**Option B: Discord**

1. Go to the [Discord Developer Portal](https://discord.com/developers/applications) and click **New Application**
2. Go to **Bot** → click **Add Bot**
3. Under **Token**, click **Reset Token** and copy it
4. Under **Privileged Gateway Intents**, enable **Message Content Intent** (required to read command text)
5. Under **OAuth2 → URL Generator**, select scopes: `bot`; permissions: `Send Messages`, `Read Message History`
6. Open the generated URL in your browser to invite the bot to your server (or use it in DMs)
7. Find your own Discord user ID: in Discord, go to Settings → Advanced → enable **Developer Mode**, then right-click your username → **Copy User ID**

Set `bot.platform: "discord"` in `config.yaml`.

### 3. Generate a GitHub Personal Access Token

1. Go to **GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens**
2. Select your target repository
3. Grant **Contents: Read & Write** and **Pull requests: Read & Write**
4. Copy the token (starts with `ghp_` or `github_pat_`)

### 4. Get an AI provider key

DevBot supports several AI backends — pick one and get a key for it:

| Provider                         | Where to get a key                                                                   |
| -------------------------------- | ------------------------------------------------------------------------------------ |
| **Claude** (default)             | [console.anthropic.com](https://console.anthropic.com)                               |
| **OpenAI**                       | [platform.openai.com/api-keys](https://platform.openai.com/api-keys)                 |
| **Gemini**                       | [aistudio.google.com](https://aistudio.google.com/app/apikey)                        |
| **Local** (Ollama, LM Studio…)   | No key needed                                                                        |
| **Codex** (ChatGPT subscription) | Run `codex login` — see [Using Codex](#using-codex-chatgpt-subscription--no-api-key) |

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

- **Telegram:** Send `/help` to your bot. You should receive the full command reference within a few seconds.
- **Discord:** Type `!help` in any channel the bot can see, or in a DM to the bot.

---

## Configuration Reference

| Field                             | Required                    | Default                           | Description                                                                                                       |
| --------------------------------- | --------------------------- | --------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `bot.platform`                    | No                          | `telegram`                        | Messaging backend — `telegram` or `discord`                                                                       |
| `telegram.token`                  | If platform=telegram        | —                                 | Bot token from @BotFather                                                                                         |
| `telegram.allowed_user_ids`       | If platform=telegram        | —                                 | List of Telegram user IDs permitted to send commands                                                              |
| `discord.token`                   | If platform=discord         | —                                 | Discord bot token from the Developer Portal                                                                       |
| `discord.allowed_user_ids`        | If platform=discord         | —                                 | List of Discord user snowflake IDs (as quoted strings)                                                            |
| `discord.command_prefix`          | No                          | `!`                               | Prefix for bot commands in Discord (e.g. `!task list`)                                                            |
| `git.name`                        | No                          | `DevBot`                          | Author name used in git commits made by the agent                                                                 |
| `git.email`                       | No                          | `devbot@users.noreply.github.com` | Author email used in commits — set to your GitHub-verified email so commits show as **Verified**                  |
| `github.token`                    | Yes                         | —                                 | GitHub PAT with Contents + Pull requests Read & Write scope                                                       |
| `github.owner`                    | If not using `github.repos` | —                                 | GitHub username or org (single-repo shorthand)                                                                    |
| `github.repo`                     | If not using `github.repos` | —                                 | Repository name without owner prefix (single-repo shorthand)                                                      |
| `github.base_branch`              | No                          | `main`                            | Default base branch for PRs (single-repo mode only)                                                               |
| `github.repos[].owner`            | Yes (per repo)              | —                                 | Owner of this repository                                                                                          |
| `github.repos[].repo`             | Yes (per repo)              | —                                 | Repository name                                                                                                   |
| `github.repos[].name`             | No                          | —                                 | Short alias shown in the `/task create` repo selection prompt                                                     |
| `github.repos[].base_branch`      | **Yes in multi-repo**       | —                                 | Base branch for PRs in this repo; required when two or more repos are configured                                  |
| `github.repos[].token`            | No                          | inherits `github.token`           | Per-repo token override                                                                                           |
| `ai.provider`                     | No                          | `claude`                          | AI backend — `claude`, `openai`, `gemini`, `local`, or `codex`                                                    |
| `claude.api_key`                  | If provider=claude          | —                                 | Anthropic API key (console.anthropic.com)                                                                         |
| `claude.model`                    | No                          | `claude-sonnet-4-6`               | Claude model — e.g. `claude-opus-4-6` for harder tasks                                                            |
| `openai.api_key`                  | If provider=openai          | —                                 | OpenAI API key (platform.openai.com)                                                                              |
| `openai.model`                    | No                          | `gpt-4o`                          | OpenAI model — e.g. `o3`, `gpt-4-turbo`                                                                           |
| `openai.base_url`                 | No                          | `https://api.openai.com/v1`       | Override for OpenAI-compatible endpoints                                                                          |
| `gemini.api_key`                  | If provider=gemini          | —                                 | Google Gemini API key (aistudio.google.com)                                                                       |
| `gemini.model`                    | No                          | `gemini-1.5-pro`                  | Gemini model — e.g. `gemini-2.0-flash`, `gemini-1.5-flash`                                                        |
| `local.base_url`                  | No                          | `http://localhost:11434`          | Base URL of the Ollama server — uses the native `/api/chat` endpoint; omit the `/v1` suffix                       |
| `local.model`                     | If provider=local           | —                                 | Model name as loaded in the local server (e.g. `gemma4`, `llama3.2`, `mistral`)                                   |
| `local.api_key`                   | No                          | ``                                | Usually blank; set to `"ollama"` if your server requires a non-empty value                                        |
| `codex.model`                     | No                          | `codex-mini-latest`               | Codex model — e.g. `o4-mini`, `gpt-4o`                                                                            |
| `codex.token_file`                | No                          | `~/.codex/auth.json`              | Path to credential file written by `codex login`                                                                  |
| `codex.access_token`              | No                          | —                                 | Paste directly instead of using token_file                                                                        |
| `codex.refresh_token`             | No                          | —                                 | Enables automatic token renewal without re-running `codex login`                                                  |
| `budget.monthly_limit_usd`        | No                          | `0`                               | Monthly spend cap in USD. When exceeded, DevBot switches to the local model. `0` = unlimited (still tracks spend) |
| `database.path`                   | No                          | `./devbot.db`                     | SQLite file path; ignored when `DATABASE_URL` env var is set                                                      |
| `schedule.enabled`                | No                          | `false`                           | Set to `true` to enable the auto-scheduler                                                                        |
| `schedule.timezone`               | No                          | `UTC`                             | IANA timezone name (e.g. `Asia/Bangkok`, `America/New_York`)                                                      |
| `schedule.work_start`             | No                          | `09:00`                           | Local time to start processing tasks (Mon-Fri only, 24h format)                                                   |
| `schedule.work_end`               | No                          | `17:00`                           | Local time to stop starting new tasks                                                                             |
| `schedule.check_interval_minutes` | No                          | `10`                              | How often (minutes) to poll for TODO tasks                                                                        |
| `schedule.enable_weekend`         | No                          | `false`                           | Set to `true` to process tasks on Saturday and Sunday as well                                                     |

**Single-repo example (Telegram):**

```yaml
bot:
  platform: "telegram" # default; can be omitted

telegram:
  token: "7123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"
  allowed_user_ids:
    - 123456789

# Set to your GitHub-verified email so commits show as Verified
git:
  name: "Alice"
  email: "alice@example.com"

github:
  token: "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  owner: "alice"
  repo: "my-project"
  base_branch: "main"

ai:
  provider: "claude"

claude:
  api_key: "sk-ant-..."
```

**Single-repo example (Discord):**

```yaml
bot:
  platform: "discord"

discord:
  token: "YOUR_DISCORD_BOT_TOKEN"
  allowed_user_ids:
    - "123456789012345678" # your Discord user ID (quoted string)
  command_prefix: "!" # use !task, !pr, !status, etc.

git:
  name: "Alice"
  email: "alice@example.com"

github:
  token: "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  owner: "alice"
  repo: "my-project"
  base_branch: "main"

ai:
  provider: "claude"

claude:
  api_key: "sk-ant-..."
```

**Multi-repo example:**

```yaml
bot:
  platform: "telegram"

telegram:
  token: "7123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"
  allowed_user_ids:
    - 123456789

git:
  name: "Alice"
  email: "alice@example.com"

github:
  token: "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" # shared default token
  repos:
    - owner: "alice"
      repo: "backend"
      name: "backend" # alias shown in /task create repo selection
      base_branch: "main" # required per repo in multi-repo mode
    - owner: "alice"
      repo: "frontend"
      name: "frontend"
      base_branch: "develop"
    - owner: "alice"
      repo: "infra"
      name: "infra"
      base_branch: "main"
      token: "ghp_separate_infra_token" # optional per-repo token override

ai:
  provider: "claude"

claude:
  api_key: "sk-ant-..."
```

---

## Command Reference

### Task Management

| Command                      | What it does                                                      | Example                                |
| ---------------------------- | ----------------------------------------------------------------- | -------------------------------------- |
| `/task create`               | Start the guided wizard (title → description → repo → tech stack) | `/task create`                         |
| `/task list`                 | Show all tasks and their current status                           | `/task list`                           |
| `/task do <id>`              | Trigger the agent to start work on a task                         | `/task do 14`                          |
| `/task done <id>`            | Mark a task complete after merging the PR                         | `/task done 14`                        |
| `/task block <id> <reason>`  | Block a task with a reason                                        | `/task block 7 "Waiting for API spec"` |
| `/task show <id>`            | Show full details for a single task                               | `/task show 14`                        |
| `/task status <id> <status>` | Manually set a task to any status                                 | `/task status 14 todo`                 |

### PR & Review

| Command            | What it does                                       | Example          |
| ------------------ | -------------------------------------------------- | ---------------- |
| `/pr <id>`         | Show the PR link and status for a task             | `/pr 14`         |
| `/pr diff <id>`    | Show an abbreviated diff in chat                   | `/pr diff 14`    |
| `/pr explain <id>` | Ask the AI to explain the changes in plain English | `/pr explain 14` |
| `/pr tests <id>`   | Ask the AI to list tests added or modified         | `/pr tests 14`   |
| `/pr retry <id>`   | Discard the current branch and start again         | `/pr retry 14`   |

### Budget

| Command          | What it does                                    |
| ---------------- | ----------------------------------------------- |
| `/budget`        | Show monthly spend, limit, and active provider  |
| `/budget pause`  | Override limit — always use commercial provider |
| `/budget resume` | Re-enable automatic fallback to local model     |

### System

| Command   | What it does                                       |
| --------- | -------------------------------------------------- |
| `/status` | Show agent health, task counts, and budget summary |
| `/help`   | List all commands with short descriptions          |

### Auto-Scheduler

| Command           | What it does                                                             |
| ----------------- | ------------------------------------------------------------------------ |
| `/schedule`       | Show scheduler status (enabled, paused, work window, queue size)         |
| `/schedule on`    | Resume auto-processing                                                   |
| `/schedule off`   | Pause auto-processing                                                    |
| `/schedule next`  | Show the next TODO task that will be picked up                           |
| `/schedule setup` | Interactive wizard to reconfigure timezone, work hours, and weekend mode |

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

| Status        | Meaning                                                                                 |
| ------------- | --------------------------------------------------------------------------------------- |
| `TODO`        | Ready to work on; accepts `/task do`                                                    |
| `IN_PROGRESS` | Agent is running — cloning, generating, pushing                                         |
| `IN_REVIEW`   | PR is open on GitHub; awaiting your review                                              |
| `DONE`        | PR merged and task manually marked complete                                             |
| `BLOCKED`     | Waiting on something external; reason stored in task                                    |
| `FAILED`      | Agent encountered an error; inspect with `/task show <id>`, retry with `/pr retry <id>` |

Use `/task status <id> <status>` to move a task to any state manually. Valid values: `todo`, `in_progress`, `in_review`, `done`, `blocked`, `failed`.

---

## Multi-Repository

DevBot can manage tasks across multiple GitHub repositories from the same bot instance. Each task is permanently linked to the repo it was created for — the agent clones, branches, and opens PRs in the correct repo automatically.

### Configuration

Replace the single `github.owner` / `github.repo` fields with a `github.repos` list. Each entry **must** declare its own `base_branch` (the global fallback is intentionally disabled in multi-repo mode to prevent branches from accidentally targeting the wrong base):

```yaml
github:
  token: "ghp_shared_token"
  repos:
    - owner: "my-org"
      repo: "backend"
      name: "backend" # short alias used in commands
      base_branch: "main"
    - owner: "my-org"
      repo: "frontend"
      name: "frontend"
      base_branch: "develop"
```

### Adding tasks to a specific repo

Use `/task create` and the wizard will list all configured repos and ask you to choose:

```
/task create
→ What's the title for your task?
→ Add a description (or /skip):
→ Which repository?
   • backend  (alice/backend)
   • frontend (alice/frontend)
   → Reply with the alias (e.g. backend) or owner/repo.
→ Any tech stack or constraints? (or /skip):
```

If you skip the repo step, the task goes to the first repo in the list (the default).

### Task list with multiple repos

`/task list` shows the repo label for every task when multiple repos are configured:

```
Tasks:

[1] [my-org/backend] Add rate limiting — IN_REVIEW
    PR: https://github.com/my-org/backend/pull/42

[2] [my-org/frontend] Update login page — TODO

[3] [my-org/backend] Fix auth token expiry — DONE
```

The label `[owner/repo]` is shown only in multi-repo mode and disappears in single-repo setups to keep the output uncluttered.

### Verified commits

Set `git.name` and `git.email` to your GitHub-verified email so all agent commits show the green **Verified** badge:

```yaml
git:
  name: "Your Name"
  email: "you@example.com" # must be verified in GitHub → Settings → Emails
```

---

## Typical Workflow

```
1. /task create
   → What's the title for your task?
   Add pagination to the /users endpoint
   → Add a description (or /skip): /skip
   → Any tech stack or constraints? (or /skip): Go, cursor-based
   → Task 1 created!

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

## Auto-Scheduler Workflow

The auto-scheduler lets you batch up tasks on the weekend and have DevBot work through them automatically during weekday business hours — no `/task do` needed.

### 1. Enable the scheduler in `config.yaml`

```yaml
schedule:
  enabled: true
  timezone: "America/New_York" # your local IANA timezone
  work_start: "09:00" # start picking up tasks (24h)
  work_end: "17:00" # stop picking up new tasks
  check_interval_minutes: 10 # poll interval
  enable_weekend: false # set true to also run Sat/Sun
```

Restart DevBot after editing the config.

### 2. Add your tasks over the weekend

Use `/task create` to add tasks one by one (guided wizard):

```
/task create   → follow the prompts for each task
```

After adding a few tasks, `/task list` shows the queue:

```
Tasks:

[1] Refactor authentication middleware to use JWT — TODO
[2] Add pagination to the /products endpoint — TODO
[3] Write unit tests for the order service — TODO
```

### 3. Let DevBot work through them on weekdays

Every `check_interval_minutes`, DevBot checks whether it's within the work window (`work_start`–`work_end` in your timezone, Mon–Fri by default or Mon–Sun when `enable_weekend: true`). If a TODO task is queued and the agent is idle, it picks up the next task automatically.

At the start of each work day you'll receive a morning briefing:

```
Good morning! Work day started. 3 task(s) in the queue.
```

As each task completes you receive the usual PR notification:

```
PR opened: https://github.com/alice/my-project/pull/12
Added JWT-based authentication middleware replacing the session-based approach…
```

### 4. Review PRs the following weekend

All PRs land in GitHub for your review. Nothing merges automatically. Use the PR review commands to inspect the work:

```
/pr explain 1      → plain-English summary of what changed
/pr diff 1         → abbreviated diff in chat
/pr tests 1        → list of tests added or modified
/pr retry 1        → discard and regenerate if the output isn't right
```

### Scheduler commands

```
/schedule          → status: work window, timezone, weekend mode, queue size, whether agent is running
/schedule off      → pause (tasks accumulate but nothing auto-starts)
/schedule on       → resume
/schedule next     → peek at the next task that will be picked up
/schedule setup    → reconfigure timezone, work hours, and weekend mode (changes are live immediately)
```

---

## Using Codex (ChatGPT Subscription — No API Key)

If you have a ChatGPT Plus, Pro, or Team subscription you can use OpenAI models without paying per-token API fees — the same way the official [OpenAI Codex CLI](https://github.com/openai/codex) works.

### 1. Log in with the Codex CLI

```bash
npm install -g @openai/codex   # install the official CLI once
codex login                    # opens browser, saves tokens to ~/.codex/auth.json
```

DevBot reads `~/.codex/auth.json` automatically. No further configuration is needed for the tokens.

### 2. Set the provider in `config.yaml`

```yaml
ai:
  provider: "codex"

codex:
  model: "codex-mini-latest" # or o4-mini, gpt-4o, etc.
```

That's it. DevBot will use your subscription credentials and automatically refresh the access token when it expires.

---

## Budget & Cost Control

DevBot tracks token usage for every AI API call and can automatically switch to a local model when your monthly spending limit is reached — so you never get a surprise bill.

### 1. Configure your limit and local fallback

```yaml
ai:
  provider: "openai" # your primary commercial provider

openai:
  api_key: "sk-..."
  model: "gpt-4o"

# Local model acts as the free fallback when budget is exceeded
local:
  base_url: "http://localhost:11434" # Ollama (default if omitted)
  model: "llama3.2"

budget:
  monthly_limit_usd: 100 # switch to local when $100 is reached for the month
```

### 2. How the switching works

| Situation             | Provider used                            |
| --------------------- | ---------------------------------------- |
| Monthly spend < limit | Commercial (OpenAI / Claude / Gemini)    |
| Monthly spend ≥ limit | Local model — automatically and silently |
| New calendar month    | Resets to commercial                     |
| `/budget pause`       | Always commercial, ignores limit         |
| `/budget resume`      | Back to automatic switching              |

When the threshold is first crossed, DevBot broadcasts a Telegram message:

```
Budget limit of $100.00 reached (spent $101.23 this month).
Switching to Local (llama3.2) for the rest of the month.
Use /budget pause to override, or /budget resume to re-enable automatic switching.
```

### 3. Check your spend

```
/budget
```

Example output:

```
Budget — 2026-04

Spent:     $42.1800 / $100.00 (42.2%)
Remaining: $57.8200
Enforcement: active — using OpenAI

Breakdown:
  OpenAI         $41.3500  (12k in / 95k out)
  Claude         $0.8300   (1k in / 3k out)
```

### 4. Override for a single urgent task

If you need the commercial model even after the budget is exceeded:

```
/budget pause       → use commercial provider regardless of spend
/task do 5          → runs on commercial
/budget resume      → automatic switching back on
```

### Notes

- **`budget.monthly_limit_usd: 0`** disables the limit but still records usage — useful for monitoring without enforcement.
- **No local model configured?** DevBot continues using the commercial provider with a warning when the limit is exceeded.
- Token prices are approximate. Use `/budget` to track actual spend and adjust the limit as needed.
- The budget counter resets at midnight UTC on the 1st of each month.

---

## Security Design

| Attack surface           | DevBot                      | Why                                                                                                        |
| ------------------------ | --------------------------- | ---------------------------------------------------------------------------------------------------------- |
| Web UI / browser surface | None                        | No HTTP server at any port                                                                                 |
| Open inbound ports       | None                        | Outbound polling to Telegram API only                                                                      |
| Authentication           | Telegram user ID allowlist  | Commands from unknown IDs are silently dropped                                                             |
| Credentials stored       | `config.yaml` only          | Two tokens total; `chmod 600` on the file                                                                  |
| GitHub token scope       | `repo` only                 | Cannot delete repos, manage org, etc.                                                                      |
| Auto-merge               | Not possible                | Token lacks merge permissions by design                                                                    |
| Plugin system            | None                        | No dynamic code loading, no community registry                                                             |
| Filesystem access        | Per-task temp dir           | Cloned to `os.MkdirTemp`, deleted with `defer os.RemoveAll`                                                |
| Prompt injection         | JSON schema + system prompt | AI output must match a strict schema; rogue instructions in repo content are rejected by the output parser |

---
