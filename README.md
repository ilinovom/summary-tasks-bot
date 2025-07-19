# Summary Tasks Bot

This example project demonstrates a simple Telegram bot with pluggable storage.
It is intentionally minimal and uses only the standard library.

## Usage

The bot understands the following commands:

* `/start` – start receiving periodic updates about default topics.
* `/update_topics <topic1> <topic2>` – change the topics you are interested in.
* `/stop` – stop receiving updates.

User settings are stored in a JSON file specified when creating the repository.

To run the bot you would initialize the `app.App` with your Telegram token and a
`FileUserSettingsRepository`:

```go
repo, _ := repository.NewFileUserSettingsRepository("settings.json")
app := app.New("<telegram-token>", repo)
app.Run(context.Background())
```

The bot periodically sends messages based on stored user preferences.
