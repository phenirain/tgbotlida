.PHONY: build up down restart logs stop ps clean help

# Default target
.DEFAULT_GOAL := help

# Start containers in detached mode
up:
	docker compose up -d --build

# Stop and remove containers
down:
	docker compose down

# Restart containers
restart:
	docker compose restart

# Show logs (follow mode)
logs:
	docker compose logs -f bot
