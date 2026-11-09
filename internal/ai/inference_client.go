package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// inMemoryResponseWriter captures HTTP response headers, status code, and body in-memory.
type inMemoryResponseWriter struct {
	header http.Header
	buf    bytes.Buffer
	code   int
}

func newInMemoryResponseWriter() *inMemoryResponseWriter {
	return &inMemoryResponseWriter{
		header: make(http.Header),
		code:   http.StatusOK,
	}
}

func (w *inMemoryResponseWriter) Header() http.Header {
	return w.header
}

func (w *inMemoryResponseWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.buf.Write(b)
}

func (w *inMemoryResponseWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

// ChatCompletionMessage represents an OpenAI-compatible message.
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionChoice represents a single completion choice.
type ChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason,omitempty"`
}

// ChatCompletionRequest represents an OpenAI-compatible chat completion request.
type ChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	MaxTokens   int                     `json:"max_tokens,omitempty"`
	Temperature float64                 `json:"temperature,omitempty"`
	TopP        float64                 `json:"top_p,omitempty"`
	Stream      bool                    `json:"stream,omitempty"`
}

// ChatCompletionResponse represents an OpenAI-compatible chat completion response.
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Choices []ChatCompletionChoice `json:"choices"`
}

// InferenceClient executes an inference request.
type InferenceClient interface {
	Complete(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error)
}

// GatewayInferenceClient implements InferenceClient by routing completion requests
// through the supervisor AI Gateway with PriorityInteractive (Lane 1).
type GatewayInferenceClient struct {
	gateway Gateway
}

// NewGatewayInferenceClient creates a new GatewayInferenceClient.
func NewGatewayInferenceClient(gw Gateway) *GatewayInferenceClient {
	return &GatewayInferenceClient{gateway: gw}
}

// Complete dispatches the completion request to the AI Gateway via an internal HTTP request.
func (c *GatewayInferenceClient) Complete(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if c.gateway == nil {
		return nil, errors.New("ai gateway is not configured")
	}

	avail, err := c.gateway.CheckModelAvailability(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check model availability: %w", err)
	}
	if !avail {
		return nil, ErrSafeStub
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal completion request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Inference-Priority", "interactive")
	httpReq.Header.Set("X-Lane", "1")
	httpReq.Header.Set("X-Generation-Intent", "precise")

	rec := newInMemoryResponseWriter()
	c.gateway.ServeHTTP(rec, httpReq)

	if rec.code == http.StatusServiceUnavailable {
		return nil, ErrSafeStub
	}
	if rec.code < 200 || rec.code >= 300 {
		return nil, fmt.Errorf("gateway inference error (status %d): %s", rec.code, rec.buf.String())
	}

	var resp ChatCompletionResponse
	if err := json.Unmarshal(rec.buf.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal completion response: %w", err)
	}
	return &resp, nil
}

// InferenceClient returns the active inference client.
func (e *RAGEngine) InferenceClient() InferenceClient {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.inferenceClient
}
