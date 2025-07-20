package config

import (
	"errors"
	"os"
)

// Config holds runtime configuration loaded from the environment.
type Config struct {
	TelegramToken string
	OpenAIToken   string
	SettingsPath  string
}

// FromEnv loads configuration from environment variables. TELEGRAM_TOKEN is required.
// OPENAI_TOKEN is optional but should be set if OpenAI integration is needed.
// SETTINGS_FILE specifies the path to the user settings JSON file and defaults
// to "settings.json" if empty.
func FromEnv() (*Config, error) {
	c := &Config{
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		OpenAIToken:   os.Getenv("OPENAI_TOKEN"),
		SettingsPath:  os.Getenv("SETTINGS_FILE"),
	}
	if c.TelegramToken == "" {
		return nil, errors.New("TELEGRAM_TOKEN is not set")
	}
	if c.SettingsPath == "" {
		c.SettingsPath = "settings.json"
	}
	return c, nil
}
