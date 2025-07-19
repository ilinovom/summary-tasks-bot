package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token:      token,
		baseURL:    "https://api.openai.com/v1",
		httpClient: http.DefaultClient,
	}
}

func (c *Client) do(ctx context.Context, endpoint string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("openai: unexpected status " + resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ChatCompletion sends a minimal chat completion request using the gpt-3.5-turbo model.
func (c *Client) ChatCompletion(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]any{
		"model":    "gpt-3.5-turbo",
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	}
	var respBody struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := c.do(ctx, "/chat/completions", reqBody, &respBody); err != nil {
		return "", err
	}
	if len(respBody.Choices) == 0 {
		return "", errors.New("openai: empty response")
	}
	return respBody.Choices[0].Message.Content, nil
}
