package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	EventDocumentDeadline = "document_deadline"
	DefaultHTTPTimeout    = 5 * time.Second
)

// WebhookDeadlinePayload represents the JSON payload dispatched to Home Assistant.
type WebhookDeadlinePayload struct {
	Event         string   `json:"event"`
	ID            string   `json:"id"`
	DocumentID    string   `json:"document_id"`
	Title         string   `json:"title"`
	DueDate       string   `json:"due_date"`
	DaysRemaining int      `json:"days_remaining"`
	Amount        *float64 `json:"amount,omitempty"`
	Currency      string   `json:"currency,omitempty"`
	URL           string   `json:"url"`
	Snippet       string   `json:"snippet,omitempty"`
}

// WebhookClient delivers notifications to Home Assistant automation endpoints.
type WebhookClient struct {
	client *http.Client
}

// NewWebhookClient initializes a WebhookClient with an enforced 5s timeout.
func NewWebhookClient() *WebhookClient {
	return &WebhookClient{
		client: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
	}
}

// SendWebhook dispatches a webhook payload to the specified URL with a 5-second timeout.
func (c *WebhookClient) SendWebhook(ctx context.Context, webhookURL string, payload *WebhookDeadlinePayload) error {
	trimmedURL := strings.TrimSpace(webhookURL)
	if trimmedURL == "" {
		return errors.New("webhook URL cannot be empty")
	}

	parsedURL, err := url.ParseRequestURI(trimmedURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return fmt.Errorf("invalid webhook URL format: %q", webhookURL)
	}

	if payload == nil {
		return errors.New("webhook payload cannot be nil")
	}
	if payload.Event == "" {
		payload.Event = EventDocumentDeadline
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, trimmedURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Sovereign-Alert-Dispatcher/0.1")

	resp, err := c.client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout") {
			return errors.New("webhook request timed out after 5 seconds")
		}
		return fmt.Errorf("webhook endpoint unreachable: %s", sanitizeNetworkError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := strings.TrimSpace(string(respBody))
		if msg != "" {
			return fmt.Errorf("webhook server responded with HTTP %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("webhook server responded with HTTP %d", resp.StatusCode)
	}

	return nil
}

// sanitizeNetworkError formats technical socket errors into clear, consumer-friendly messages.
func sanitizeNetworkError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "connection refused") {
		return "connection refused by target host"
	}
	if strings.Contains(msg, "no such host") {
		return "target host could not be resolved"
	}
	if strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "context deadline exceeded") {
		return "connection timed out"
	}
	if strings.Contains(msg, "certificate") {
		return "TLS certificate validation failed"
	}
	return "network request failed"
}
