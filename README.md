# Telegram Bot with GPT

This project is a Go-based Telegram bot that can send invites, ideas, news, and more. The bot is designed with a simple layered architecture and can run in Docker.

## Project Structure

```
cmd/telegrambot       Entry point
internal/app          Application coordination
internal/service      Business logic services
internal/repository   Data access layer
internal/model        Shared data types
pkg/                  Reusable packages
config/               Configuration files
```

## Quick Start

```bash
go build ./cmd/telegrambot
./telegrambot
```

Or run with Docker:

```bash
docker build -t telegrambot .
docker run telegrambot
```

## Testing

Run all unit tests:

```bash
go test ./...
```

The project is built and tested automatically using GitHub Actions.

