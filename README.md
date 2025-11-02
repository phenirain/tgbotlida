# Telegram Email Collection Bot

A Telegram bot that collects user emails with policy acceptance. Each user can submit only one email.

## Features

- Welcome message on bot start
- Policy acceptance requirement
- Email validation
- SQLite database for storage
- One email per user restriction
- Dockerized deployment

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

Edit `.env` and add your bot token and customize messages:

```env
BOT_TOKEN=your_bot_token_here
WELCOME_MESSAGE=Your welcome message
POLICY_TEXT=Your policy text
FINAL_MESSAGE=Your thank you message
```

### 3. Run with Docker

```bash
docker-compose up -d
```

To view logs:

```bash
docker-compose logs -f
```

To stop:

```bash
docker-compose down
```

### 4. Run without Docker

```bash
pip install -r requirements.txt
python bot.py
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

The bot uses SQLite database stored in `./data/users.db` with the following schema:

```sql
CREATE TABLE users (
    user_id INTEGER PRIMARY KEY,
    email TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
```

## Commands

- `/start` - Start the bot and begin email collection process
- `/cancel` - Cancel current operation
