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

// Config holds runtime configuration loaded from the environment.
type PromptConfig struct {
	Prompt string `json:"prompt"`
	Style  string `json:"style"`
	Volume string `json:"volume"`
}

type Config struct {
	TelegramToken string
	OpenAIToken   string
	OpenAIBaseURL string
	OpenAIModel   string
	SettingsPath  string
	OptionsFile   string
	PromptFile    string

	Options Options
	Prompt  PromptConfig
}

// FromEnv loads configuration from environment variables. TELEGRAM_TOKEN is required.
// OPENAI_TOKEN is optional but should be set if OpenAI integration is needed.
// SETTINGS_FILE specifies the path to the user settings JSON file and defaults
// to "settings.json" if empty.
func FromEnv() (*Config, error) {
	c := &Config{
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		OpenAIToken:   os.Getenv("OPENAI_TOKEN"),
		OpenAIBaseURL: os.Getenv("OPENAI_BASE_URL"),
		OpenAIModel:   os.Getenv("OPENAI_MODEL"),
		SettingsPath:  os.Getenv("SETTINGS_FILE"),
		OptionsFile:   os.Getenv("OPTIONS_FILE"),
		PromptFile:    os.Getenv("PROMPT_FILE"),
	}
	if c.TelegramToken == "" {
		return nil, errors.New("TELEGRAM_TOKEN is not set")
	}
	if c.SettingsPath == "" {
		c.SettingsPath = "settings.json"
	}
	if c.OpenAIBaseURL == "" {
		c.OpenAIBaseURL = "https://api.openai.com/v1"
	}
	if c.OpenAIModel == "" {
		c.OpenAIModel = "gpt-3.5-turbo"
	}
	if c.OptionsFile == "" {
		c.OptionsFile = "options.json"
	}
	if c.PromptFile == "" {
		c.PromptFile = "prompt.json"
	}
	if err := c.loadOptions(); err != nil {
		return nil, err
	}
	if err := c.loadPrompt(); err != nil {
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

func (c *Config) loadPrompt() error {
	file, err := os.Open(c.PromptFile)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&c.Prompt)
}
