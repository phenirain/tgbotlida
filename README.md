# Telegram Email Collection Bot

A high-performance Telegram bot written in Go that collects user emails with policy acceptance. Features async database operations and PostgreSQL storage. Each user can submit only one email.

## Features

- Written in Go for high performance and concurrency
- Async database operations using worker pattern
- PostgreSQL database for reliable storage
- Welcome message on bot start
- Policy acceptance requirement
- Email validation
- One email per user restriction
- Dockerized deployment with PostgreSQL
- Health checks and graceful shutdown

## Setup

### 1. Get Bot Token

1. Open Telegram and find [@BotFather](https://t.me/botfather)
2. Send `/newbot` command
3. Follow the instructions to create your bot
4. Copy the bot token

### 2. Configure Environment

```bash
cp .env.example .env
```

Edit `.env` and add your bot token, database URL, and customize messages:

```env
BOT_TOKEN=your_bot_token_here
DATABASE_URL=postgres://bot_user:bot_password@postgres:5432/bot_db?sslmode=disable
WELCOME_MESSAGE=Your welcome message
POLICY_TEXT=Your policy text
FINAL_MESSAGE=Your thank you message
```

Note: The DATABASE_URL in the example uses `postgres` as the hostname, which is the PostgreSQL service name in docker-compose. If running locally without Docker, use `localhost` instead.

### 3. Run with Docker (Recommended)

The bot and PostgreSQL will run in Docker containers:

```bash
# Start both bot and PostgreSQL
make up
# or
docker-compose up -d --build
```

To view logs:

```bash
make logs
# or
docker-compose logs -f bot
```

To stop:

```bash
make down
# or
docker-compose down
```

To restart:

```bash
make restart
```

### 4. Run without Docker

First, ensure you have PostgreSQL installed and running locally:

```bash
# Install PostgreSQL (macOS)
brew install postgresql
brew services start postgresql

# Create database and user
createdb bot_db
psql -d bot_db -c "CREATE USER bot_user WITH PASSWORD 'bot_password';"
psql -d bot_db -c "GRANT ALL PRIVILEGES ON DATABASE bot_db TO bot_user;"
```

Then run the bot:

```bash
# Install Go dependencies
go mod download

# Run the bot
go run .

# Or build and run
go build -o bot .
./bot
```

## Bot Flow

1. User sends `/start` command
2. Bot sends welcome message
3. Bot shows policy text with "Accept" button
4. User clicks "Accept"
5. Bot asks for email
6. User provides email
7. Bot validates and saves email to database
8. Bot sends final message

## Database

The bot uses PostgreSQL for reliable and scalable storage. Database operations are handled asynchronously using a worker pattern with Go channels for optimal performance.

### Schema

```sql
CREATE TABLE users (
    user_id BIGINT PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
)
```

### Async Operations

All database operations (checking user existence, saving emails) are performed asynchronously:

1. Main handlers send requests to a buffered channel
2. A dedicated worker goroutine processes requests sequentially
3. Responses are sent back through response channels
4. This ensures thread-safe database access and prevents blocking the main bot loop

## Commands

- `/start` - Start the bot and begin email collection process
- `/cancel` - Cancel current operation

## Project Structure

```
.
├── main.go              # Main bot logic and handlers
├── config/
│   └── config.go        # Configuration management
├── database/
│   └── db.go            # Async database operations
├── docker-compose.yml   # Docker services configuration
├── Dockerfile           # Multi-stage Docker build
├── Makefile            # Build and deployment commands
├── go.mod              # Go dependencies
└── .env                # Environment variables (not in repo)
```

## Technologies

- **Language**: Go 1.21
- **Database**: PostgreSQL 16
- **Bot API**: github.com/go-telegram-bot-api/telegram-bot-api/v5
- **Database Driver**: github.com/jackc/pgx/v5
- **Containerization**: Docker & Docker Compose

## Architecture Highlights

- **Async Database Layer**: Uses Go channels and worker goroutines for non-blocking database operations
- **Connection Pooling**: Configurable PostgreSQL connection pool (max 25 open, 5 idle)
- **State Management**: Thread-safe conversation state tracking using sync.RWMutex
- **Multi-stage Build**: Optimized Docker image using multi-stage builds
- **Health Checks**: PostgreSQL health checks ensure database is ready before bot starts
