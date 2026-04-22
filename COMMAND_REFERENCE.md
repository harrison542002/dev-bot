# Command Reference

DevBot supports the same core commands across Telegram and Discord.

- Telegram examples use `/...`
- Discord uses your configured prefix instead, such as `!task create`

## Task Management

| Command                      | What it does                                                      | Example                                |
| ---------------------------- | ----------------------------------------------------------------- | -------------------------------------- |
| `/task create`               | Start the guided wizard (title -> description -> repo -> tech stack) | `/task create`                         |
| `/task list`                 | Show all tasks and their current status                           | `/task list`                           |
| `/task do <id>`              | Trigger the agent to start work on a task                         | `/task do 14`                          |
| `/task done <id>`            | Mark a task complete after merging the PR                         | `/task done 14`                        |
| `/task block <id> <reason>`  | Block a task with a reason                                        | `/task block 7 "Waiting for API spec"` |
| `/task show <id>`            | Show full details for a single task                               | `/task show 14`                        |
| `/task status <id> <status>` | Manually set a task to any status                                 | `/task status 14 todo`                 |

## PR & Review

| Command            | What it does                                       | Example          |
| ------------------ | -------------------------------------------------- | ---------------- |
| `/pr <id>`         | Show the PR link and status for a task             | `/pr 14`         |
| `/pr diff <id>`    | Show an abbreviated diff in chat                   | `/pr diff 14`    |
| `/pr explain <id>` | Ask the AI to explain the changes in plain English | `/pr explain 14` |
| `/pr tests <id>`   | Ask the AI to list tests added or modified         | `/pr tests 14`   |
| `/pr retry <id>`   | Discard the current branch and start again         | `/pr retry 14`   |

## Budget

| Command          | What it does                                    |
| ---------------- | ----------------------------------------------- |
| `/budget`        | Show monthly spend, limit, and active provider  |
| `/budget pause`  | Override limit -> always use commercial provider |
| `/budget resume` | Re-enable automatic fallback to local model     |

## System

| Command   | What it does                                       |
| --------- | -------------------------------------------------- |
| `/status` | Show agent health, task counts, and budget summary |
| `/help`   | List all commands with short descriptions          |

## Auto-Scheduler

| Command           | What it does                                                             |
| ----------------- | ------------------------------------------------------------------------ |
| `/schedule`       | Show scheduler status (enabled, paused, work window, queue size)         |
| `/schedule on`    | Resume auto-processing                                                   |
| `/schedule off`   | Pause auto-processing                                                    |
| `/schedule next`  | Show the next TODO task that will be picked up                           |
| `/schedule setup` | Interactive wizard to reconfigure timezone, work hours, and weekend mode |

## Task Lifecycle

```text
                 /task do <id>
  +--------+  --------------------->  +-------------+
  |  TODO  |                          | IN_PROGRESS |
  +--------+  <---------------------  +-------------+
      ^         error / /pr retry          |
      |                                    | agent opens PR
      |                                    v
      |                           +--------------+
      |     /task done <id>       |  IN_REVIEW   |
      |  <------------------------+--------------+
      |                                    |
      |                                    v
      |                            +--------------+
      |                            |     DONE     |
      |                            +--------------+
      |
      |  /task block <id> <reason>
      +---------------------------> BLOCKED (any state)
```

## States

| Status        | Meaning                                                                                 |
| ------------- | --------------------------------------------------------------------------------------- |
| `TODO`        | Ready to work on; accepts `/task do`                                                    |
| `IN_PROGRESS` | Agent is running -> cloning, generating, pushing                                        |
| `IN_REVIEW`   | PR is open on GitHub; awaiting your review                                              |
| `DONE`        | PR merged and task manually marked complete                                             |
| `BLOCKED`     | Waiting on something external; reason stored in task                                    |
| `FAILED`      | Agent encountered an error; inspect with `/task show <id>`, retry with `/pr retry <id>` |

Use `/task status <id> <status>` to move a task to any state manually. Valid values: `todo`, `in_progress`, `in_review`, `done`, `blocked`, `failed`.
