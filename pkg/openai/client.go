package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates an OpenAI API client. If baseURL is empty the official
// endpoint is used.
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

// do performs a POST request to the given endpoint and decodes the response.
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

// ChatResponses calls the experimental /responses endpoint to get news with web search results.
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

// markdownToTelegramHTML converts a subset of Markdown to HTML allowed by Telegram.
func markdownToTelegramHTML(input string) string {
	// Экранируем спецсимволы
	input = html.EscapeString(input)

	// Жирный (**…**)
	reBold := regexp.MustCompile(`\*\*(.*?)\*\*`)
	input = reBold.ReplaceAllString(input, "<b>$1</b>")

	// Курсив (*…*)
	reItalic := regexp.MustCompile(`\*(.*?)\*`)
	input = reItalic.ReplaceAllString(input, "<i>$1</i>")

	// Ссылки [текст](url)
	reLink := regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
	input = reLink.ReplaceAllString(input, `<a href="$2">$1</a>`)

	return removeUnclosedAnchor(input)
}

func removeUnclosedAnchor(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")

	if len(lines) == 0 {
		return text
	}

	lastLine := lines[len(lines)-1]

	// Проверяем наличие <a и отсутствие </a>
	if strings.Contains(lastLine, "<a") && !strings.Contains(lastLine, "</a>") {
		lines = lines[:len(lines)-1]
	}

	return strings.Join(lines, "\n")
}

//// Разрешённые теги для Telegram
//var allowedTags = map[string]bool{
//	"b":      true,
//	"strong": true,
//	"i":      true,
//	"em":     true,
//	"u":      true,
//	"s":      true,
//	"strike": true,
//	"del":    true,
//	"a":      true,
//	"code":   true,
//	"pre":    true,
//}
//
//// Конвертация Markdown в Telegram HTML
//func markdownToTelegramHTML(md string) (string, error) {
//	// Конвертируем Markdown в обычный HTML
//	var buf bytes.Buffer
//	if err := goldmark.Convert([]byte(md), &buf); err != nil {
//		return "", err
//	}
//	html := buf.String()
//
//	// Убираем запрещённые теги
//	re := regexp.MustCompile(`</?([a-zA-Z0-9]+)(\s[^>]*)?>`)
//	html = re.ReplaceAllStringFunc(html, func(tag string) string {
//		// Оставляем только разрешённые теги
//		tagName := strings.Trim(tag, "</> ")
//		tagName = strings.Split(tagName, " ")[0] // отрезаем атрибуты
//		if allowedTags[tagName] {
//			// Если тег <a>, оставляем href
//			if strings.HasPrefix(tag, "<a ") || strings.HasPrefix(tag, "</a") {
//				return tag
//			}
//			// Остальные возвращаем как есть
//			return tag
//		}
//		return "" // убираем запрещённые теги
//	})
//
//	// Убираем лишние <p> и заменяем их на переносы строк
//	html = strings.ReplaceAll(html, "<p>", "")
//	html = strings.ReplaceAll(html, "</p>", "\n")
//
//	// Убираем лишние пробелы
//	html = strings.TrimSpace(html)
//
//	return html, nil
//}
