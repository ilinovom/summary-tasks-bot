# Summary Tasks Bot

This example project demonstrates a simple Telegram bot with pluggable storage.
It is intentionally minimal and uses only the standard library.

## Usage

The bot understands the following commands:

* `/start` – start receiving periodic updates about default topics.
* `/update_topics <topic1> <topic2>` – change the topics you are interested in.
* `/get_news_now` – request an immediate news summary based on your preferences.
* `/stop` – stop receiving updates.

User settings are stored in a JSON file specified via the `SETTINGS_FILE` environment variable.

### Running

Set the following environment variables before running the bot:

* `TELEGRAM_TOKEN` – your Telegram bot token (required)
* `OPENAI_TOKEN` – OpenAI API token (optional, enables news generation using OpenAI)
* `SETTINGS_FILE` – path to the JSON file for storing user settings (defaults to `settings.json`)

Then start the bot with:

```bash
go run ./cmd/bot
```

The bot periodically sends messages based on stored user preferences.
