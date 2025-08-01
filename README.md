# Summary Tasks Bot

This example project demonstrates a simple Telegram bot with pluggable storage.
It is intentionally minimal and uses only the standard library.

## Usage

The bot understands the following commands:

* `/start` – start receiving periodic updates about default categories.
* `/info` – show all available commands.
* `/tariffs` – see a description of the tariffs.
* `/topics` – manage your topics (/update_topics, /add_topic, /delete_topics, /my_topics).
* `/update_topics` – update the categories you are interested in.
* `/add_topic` – add more categories without resetting everything.
* `/delete_topics` – remove selected categories.
* `/get_news_now` – request an immediate news summary based on your preferences.
* `/get_last_24h_news` – get a recap of news from the last 24 hours for one of your categories (Plus and higher).
* `/my_topics` – show your selected info types and categories.
* `/stop` – stop receiving updates.

User settings are stored in a Postgres database specified via the `DATABASE_URL` environment variable.

### Running

Set the following environment variables before running the bot:

* `TELEGRAM_TOKEN` – your Telegram bot token (required)
* `OPENAI_TOKEN` – OpenAI API token (optional, enables news generation using OpenAI)
* `OPENAI_MODEL` – GPT model to use (defaults to `gpt-3.5-turbo`)
* `OPENAI_BASE_URL` – base URL for the OpenAI API (optional)
* `DATABASE_URL` – Postgres connection string (required)
* `OPTIONS_FILE` – path to JSON with option lists (defaults to `options.json`)
* `PROMPT_FILE` – path to the prompt configuration JSON (defaults to `prompt.json`)
* `TARIFF_FILE` – path to the tariffs configuration JSON (defaults to `tariff.json`)

Then start the bot with:

```bash
go run ./cmd/bot
```

The bot periodically sends messages based on stored user preferences.

### Docker

Build and run the bot in a container:

```bash
docker build -t summary-tasks-bot .
docker run --rm \
  -e TELEGRAM_TOKEN=<token> \
  -e DATABASE_URL=<connection> \
  summary-tasks-bot
```

This image includes the default `options.json` and `tariff.json` files.
