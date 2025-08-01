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

type Schedule struct {
	FrequencyMinutes int    `json:"frequency_minutes"`
	TimeRange        string `json:"time_range"`
}

type Limits struct {
	GetNewsNowPerDay    int `json:"get_news_now_per_day"`
	GetLast24hNewPerDay int `json:"get_last_24h_new_per_day"`
	CategoryLimit       int `json:"category_limit"`
	InfoTypeLimit       int `json:"info_type_limit"`
}

type GPTConfig struct {
	Model         string `json:"model"`
	PromptMain    string `json:"prompt_main"`
	PromptLast24h string `json:"prompt_last_24h"`
	MaxTokens     int    `json:"max_tokens"`
	Style         string `json:"style"`
	Volume        string `json:"volume"`
}

type Tariff struct {
	Schedule            Schedule  `json:"schedule"`
	Limits              Limits    `json:"limits"`
	GPT                 GPTConfig `json:"gpt"`
	AllowCustomCategory bool      `json:"allow_custom_category"`
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
	MessagesFile  string

	Options  Options
	Tariffs  map[string]Tariff
	Messages map[string]string
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
		MessagesFile:  os.Getenv("MESSAGES_FILE"),
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
	if c.MessagesFile == "" {
		c.MessagesFile = "messages.json"
	}
	if err := c.loadOptions(); err != nil {
		return nil, err
	}
	if err := c.loadTariffs(); err != nil {
		return nil, err
	}
	if err := c.loadMessages(); err != nil {
		return nil, err
	}
	return c, nil
}

// loadOptions reads info and category options from disk.
func (c *Config) loadOptions() error {
	file, err := os.Open(c.OptionsFile)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&c.Options)
}

// loadTariffs loads tariff definitions from the configured file.
func (c *Config) loadTariffs() error {
	file, err := os.Open(c.TariffFile)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&c.Tariffs)
}

// loadMessages parses bot reply templates from disk.
func (c *Config) loadMessages() error {
	file, err := os.Open(c.MessagesFile)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&c.Messages)
}
