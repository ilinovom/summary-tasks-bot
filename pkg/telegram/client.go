package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// Update represents a Telegram update. Only fields we need.
type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type Chat struct {
	ID int64 `json:"id"`
}

// Client is a minimal Telegram Bot API client.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// BotCommand describes a bot command for the Telegram menu.
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

func NewClient(token string) *Client {
	return &Client{
		token:      token,
		baseURL:    "https://api.telegram.org",
		httpClient: http.DefaultClient,
	}
}

func (c *Client) url(method string) string {
	return c.baseURL + "/bot" + c.token + "/" + method
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, keyboard [][]string) error {
	body := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	if keyboard != nil {
		body["reply_markup"] = map[string]any{
			"keyboard":          keyboard,
			"one_time_keyboard": true,
			"resize_keyboard":   true,
		}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("sendMessage"), bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("telegram: unexpected status " + resp.Status)
	}
	return nil
}

func (c *Client) GetUpdates(ctx context.Context, offset int) ([]Update, error) {
	q := url.Values{}
	if offset != 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url("getUpdates"), nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = q.Encode()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("telegram: unexpected status " + resp.Status)
	}
	var wrapper struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	if !wrapper.OK {
		return nil, errors.New("telegram: api responded with not ok")
	}
	return wrapper.Result, nil
}

// SetCommands registers the bot commands shown in the Telegram UI.
func (c *Client) SetCommands(ctx context.Context, commands []BotCommand) error {
	body := map[string]any{"commands": commands}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("setMyCommands"), bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("telegram: unexpected status " + resp.Status)
	}
	return nil
}
