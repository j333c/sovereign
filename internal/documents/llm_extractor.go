package documents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// LLMExtractor is an interface for Tier 2 LLM-based obligation extraction.
type LLMExtractor interface {
	ExtractObligations(ctx context.Context, text string, fallbackTitle string) ([]ExtractedObligation, error)
}

// GatewayLLMExtractor communicates with the internal AI Gateway's /v1/chat/completions endpoint.
type GatewayLLMExtractor struct {
	baseURL    string
	client     *http.Client
	logger     *slog.Logger
	timeout    time.Duration
}

// NewGatewayLLMExtractor creates an LLM extractor targeting the internal AI Gateway.
func NewGatewayLLMExtractor(baseURL string, logger *slog.Logger) *GatewayLLMExtractor {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &GatewayLLMExtractor{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
		logger:  logger,
		timeout: 90 * time.Second,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Messages    []chatMessage `json:"messages"`
	Preset      string        `json:"preset,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatCompletionChoice struct {
	Message chatMessage `json:"message"`
}

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
}

type llmObligationItem struct {
	DueDate  string   `json:"due_date"`
	Amount   *float64 `json:"amount,omitempty"`
	Currency string   `json:"currency,omitempty"`
	Title    string   `json:"title,omitempty"`
	Snippet  string   `json:"snippet,omitempty"`
}

type llmExtractionResult struct {
	Obligations []llmObligationItem `json:"obligations"`
	// Backwards compatibility for single item response
	DueDate  string   `json:"due_date,omitempty"`
	Amount   *float64 `json:"amount,omitempty"`
	Currency string   `json:"currency,omitempty"`
	Title    string   `json:"title,omitempty"`
	Snippet  string   `json:"snippet,omitempty"`
}

var jsonCodeBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

// ExtractObligations queries the internal AI Gateway with background lane headers and Precise & Factual preset.
func (e *GatewayLLMExtractor) ExtractObligations(ctx context.Context, text string, fallbackTitle string) ([]ExtractedObligation, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	systemPrompt := `You are a precise document obligation and deadline extraction assistant.
Analyze the provided document text and extract any impending payment due dates, contract expiration dates, or policy renewal deadlines.
Output strictly valid JSON with the format:
{
  "obligations": [
    {
      "due_date": "YYYY-MM-DD",
      "amount": 150.00,
      "currency": "USD",
      "title": "Invoice #482",
      "snippet": "..."
    }
  ]
}
If no deadlines or obligations exist, return: {"obligations": []}.`

	reqBody := chatCompletionRequest{
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Extract obligations from this document text:\n\n%s", text)},
		},
		Preset:      "precise",
		Temperature: 0.1,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal LLM request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/v1/chat/completions", e.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Mandatory background priority lane and intent headers per spec
	req.Header.Set("X-Inference-Priority", "background")
	req.Header.Set("X-Generation-Intent", "precise")
	req.Header.Set("X-Priority", "background")
	req.Header.Set("X-Lane", "batch")

	resp, err := e.client.Do(req)
	if err != nil {
		e.logger.Debug("LLM gateway unavailable, falling back gracefully to Tier 1 baseline", "error", err)
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respSnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		e.logger.Debug("LLM gateway returned non-200 status, falling back", "status", resp.StatusCode, "body", string(respSnippet))
		return nil, fmt.Errorf("LLM gateway returned HTTP %d: %s", resp.StatusCode, string(respSnippet))
	}

	var chatResp chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode LLM response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, errors.New("empty choices from LLM gateway")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	if content == "" {
		return nil, nil
	}

	// Strip markdown ```json code blocks if present
	if match := jsonCodeBlockRegex.FindStringSubmatch(content); len(match) > 1 {
		content = match[1]
	}

	var parsed llmExtractionResult
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		e.logger.Debug("malformed JSON from LLM gateway, falling back", "content", content, "error", err)
		return nil, fmt.Errorf("malformed JSON from LLM: %w", err)
	}

	var items []llmObligationItem
	if len(parsed.Obligations) > 0 {
		items = parsed.Obligations
	} else if parsed.DueDate != "" {
		items = append(items, llmObligationItem{
			DueDate:  parsed.DueDate,
			Amount:   parsed.Amount,
			Currency: parsed.Currency,
			Title:    parsed.Title,
			Snippet:  parsed.Snippet,
		})
	}

	var results []ExtractedObligation
	for _, it := range items {
		cleanDate := strings.TrimSpace(it.DueDate)
		if cleanDate == "" {
			continue
		}

		// Validate that the date is a valid calendar date
		t, err := time.Parse("2006-01-02", cleanDate)
		if err != nil {
			// Try European format if returned
			if tEuro, errEuro := time.Parse("02.01.2006", cleanDate); errEuro == nil {
				cleanDate = tEuro.Format("2006-01-02")
			} else {
				continue
			}
		} else {
			cleanDate = t.Format("2006-01-02")
		}

		title := strings.TrimSpace(it.Title)
		if title == "" {
			title = fallbackTitle
		}
		if title == "" {
			title = "Contractual Obligation"
		}

		results = append(results, ExtractedObligation{
			DueDate:    cleanDate,
			Amount:     it.Amount,
			Currency:   normalizeCurrency(it.Currency),
			Title:      title,
			Snippet:    strings.TrimSpace(it.Snippet),
			Confidence: 0.98,
		})
	}

	return results, nil
}
