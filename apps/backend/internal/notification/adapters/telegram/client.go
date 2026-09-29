package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Button = application.Button

type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

func NewClient(token string) *Client {
	return &Client{token: token, baseURL: "https://api.telegram.org", http: &http.Client{Timeout: 10 * time.Second}}
}

func NewClientWithBaseURL(token, baseURL string, client *http.Client) *Client {
	return &Client{token: token, baseURL: strings.TrimRight(baseURL, "/"), http: client}
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, buttons [][]Button) error {
	body := map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML", "disable_web_page_preview": true}
	if len(buttons) > 0 {
		body["reply_markup"] = map[string]any{"inline_keyboard": buttons}
	}
	return c.call(ctx, "sendMessage", body)
}

func (c *Client) AnswerCallback(ctx context.Context, callbackID, text string) error {
	return c.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": callbackID, "text": text})
}

func (c *Client) SetWebhook(ctx context.Context, webhookURL, secret string) error {
	return c.call(ctx, "setWebhook", map[string]any{
		"url":             webhookURL,
		"secret_token":    secret,
		"allowed_updates": []string{"message", "callback_query"},
	})
}

func (c *Client) call(ctx context.Context, method string, payload any) error {
	if c.token == "" || c.http == nil {
		return fmt.Errorf("Telegram bot client is not configured")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+c.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("call Telegram %s: network request failed", method)
	}
	defer response.Body.Close()
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return fmt.Errorf("decode Telegram %s response: %w", method, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !result.OK {
		retryAfter := result.Parameters.RetryAfter
		if retryAfter == 0 {
			retryAfter, _ = strconv.Atoi(response.Header.Get("Retry-After"))
		}
		description := strings.ToLower(result.Description)
		return &application.TelegramAPIError{StatusCode: response.StatusCode, RetryAfter: time.Duration(retryAfter) * time.Second, Blocked: response.StatusCode == http.StatusForbidden || strings.Contains(description, "bot was blocked") || strings.Contains(description, "chat not found")}
	}
	return nil
}
