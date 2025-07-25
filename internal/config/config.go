package config

import (
	"encoding/json"
	"errors"
	"os"
)

// Config holds runtime configuration loaded from the environment.
type Options struct {
	InfoOptions     []string `json:"info_options"`
	CategoryOptions []string `json:"category_options"`
}

type Tariff struct {
	FrequencyScheduledMsgSendInMinutes int    `json:"frequency_scheduled_msg_send_in_minutes"`
	TimeRangeScheduledMsgSendPerDay    string `json:"time_range_scheduled_msg_send_per_day"`
	NumberGetNewsNowMessagesPerDay     int    `json:"number_get_news_now_messages_per_day"`
	Prompt                             string `json:"prompt"`
	Style                              string `json:"style"`
	Volume                             string `json:"volume"`
	GptModelVersion                    string `json:"gpt_model_version"`
	CategoryNumLimit                   int    `json:"category_num_limit"`
	InfoTypeNumLimit                   int    `json:"info_type_num_limit"`
	AllowCustomCategory                bool   `json:"allow_custom_category"`
}

type Config struct {
	TelegramToken string
	OpenAIToken   string
	OpenAIBaseURL string
	OpenAIModel   string
	DBConnString  string
	SettingsPath  string
	OptionsFile   string
	PromptFile    string
	TariffFile    string

	Options Options
	Tariffs map[string]Tariff
}

// FromEnv loads configuration from environment variables. TELEGRAM_TOKEN is required.
// OPENAI_TOKEN is optional but should be set if OpenAI integration is needed.
// DATABASE_URL specifies the Postgres connection string. SETTINGS_FILE is ignored
// but defaults to "settings.json" if empty for backward compatibility.
func FromEnv() (*Config, error) {
	c := &Config{
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		OpenAIToken:   os.Getenv("OPENAI_TOKEN"),
		OpenAIBaseURL: os.Getenv("OPENAI_BASE_URL"),
		OpenAIModel:   os.Getenv("OPENAI_MODEL"),
		DBConnString:  os.Getenv("DATABASE_URL"),
		SettingsPath:  os.Getenv("SETTINGS_FILE"),
		OptionsFile:   os.Getenv("OPTIONS_FILE"),
		TariffFile:    os.Getenv("TARIFF_FILE"),
	}
	if c.TelegramToken == "" {
		return nil, errors.New("TELEGRAM_TOKEN is not set")
	}
	if c.DBConnString == "" {
		c.DBConnString = "postgres://user:pass@localhost:5432/postgres?sslmode=disable"
	}
	if c.SettingsPath == "" {
		c.SettingsPath = "settings.json"
	}
	if c.OpenAIBaseURL == "" {
		c.OpenAIBaseURL = "https://api.openai.com/v1"
	}
	if c.OptionsFile == "" {
		c.OptionsFile = "options.json"
	}
	if c.TariffFile == "" {
		c.TariffFile = "tariff.json"
	}
	if err := c.loadOptions(); err != nil {
		return nil, err
	}
	if err := c.loadTariffs(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) loadOptions() error {
	file, err := os.Open(c.OptionsFile)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&c.Options)
}

func (c *Config) loadTariffs() error {
	file, err := os.Open(c.TariffFile)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&c.Tariffs)
}
