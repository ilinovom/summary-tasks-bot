package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func NewClient(token, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Client{
		token:      token,
		baseURL:    baseURL,
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
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openai: unexpected status %s: %s", resp.Status, string(data))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

// ChatCompletion sends a minimal chat completion request using the configured model.
func (c *Client) ChatCompletion(ctx context.Context, model, prompt string, maxTokens int) (string, error) {

	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	if maxTokens > 0 {
		reqBody["max_tokens"] = maxTokens
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

// ChatResponses
func (c *Client) ChatResponses(ctx context.Context, model, prompt string, maxTokens int) (string, error) {

	reqBody := map[string]any{
		"model": model,
		"input": []map[string]string{{"role": "user", "content": prompt}},
	}
	if maxTokens > 0 {
		reqBody["max_output_tokens"] = maxTokens
	}

	//// Пример добавления функции поиска
	//reqBody["tools"] = []map[string]any{
	//	{
	//		"type": "function",
	//		"function": map[string]any{
	//			"name":        "web_search",
	//			"description": "Search the web for information",
	//			"parameters": map[string]any{
	//				"type": "object",
	//				"properties": map[string]any{
	//					"query": map[string]string{
	//						"type":        "string",
	//						"description": "Search query",
	//					},
	//				},
	//				"required": []string{"query"},
	//			},
	//		},
	//	},
	//}

	reqBody["tools"] = []map[string]string{{"type": "web_search_preview"}}

	var respBody struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := c.do(ctx, "/responses", reqBody, &respBody); err != nil {
		return "", err
	}
	if len(respBody.Output) < 2 || len(respBody.Output[1].Content) == 0 {
		return "", errors.New("openai: empty response")
	}

	return markdownToTelegramHTML(respBody.Output[1].Content[0].Text), nil
}

func markdownToTelegramHTML(input string) string {
	// Жирный
	reBold := regexp.MustCompile(`\*\*(.*?)\*\*`)
	input = reBold.ReplaceAllString(input, "<b>$1</b>")

	// Ссылки [text](url)
	reLink := regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
	input = reLink.ReplaceAllString(input, `<a href="$2">$1</a>`)

	return input
}
