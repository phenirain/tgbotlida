# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run locally (requires .env file)
go run main.go

# Build binary
go build -o bot .

# Docker (primary deployment method)
make up        # build and start containers
make down      # stop and remove containers
make restart   # restart containers
make logs      # follow bot logs
```

Copy `.env.example` to `.env` and fill in `BOT_TOKEN` and `DATABASE_URL` before running.

## Architecture

Single-package Go bot with three packages:

- **`main.go`** — `Bot` struct owns all Telegram logic. Conversation state is tracked in `map[int64]ConversationState` guarded by `sync.RWMutex`. State machine has four states: `StateNone → StateAwaitingListenConfirm → StateAwaitingPolicyAccept → StateAwaitingEmail`.
- **`config/config.go`** — loads all config from env vars. All message texts (welcome, policy, final) come from env, not code. `\n` escape sequences in env values are processed into real newlines.
- **`database/db.go`** — wraps PostgreSQL (`jackc/pgx/v5` via `database/sql` stdlib). Direct concurrent calls to `sql.DB` — it is goroutine-safe and manages the connection pool (25 open / 5 idle) internally. No extra synchronization layer needed.

## Database

PostgreSQL only. Schema is created inline at startup (`initDB`) — no migration system. One table:

```sql
CREATE TABLE IF NOT EXISTS users (
    user_id   BIGINT PRIMARY KEY,
    username  VARCHAR(255),
    email     VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
)
```

Uniqueness is enforced at both the DB level (PRIMARY KEY + UNIQUE) and in code (existence checks before insert).

## Configuration

All texts and links are environment-driven — changing bot messages requires only updating `.env`, not code. `PHOTO_URL` accepts a URL, local file path, or Telegram `file_id`; leave it empty to skip the photo.
