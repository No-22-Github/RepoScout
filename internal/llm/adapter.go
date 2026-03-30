// Package llm provides LLM integration for RepoScout.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/no22/repo-scout/internal/config"
)

// ProviderAdapter defines the interface for LLM provider backends.
// This abstraction allows RepoScout to work with different LLM providers
// without coupling to any specific implementation.
type ProviderAdapter interface {
	// Execute sends a TaskCard to the LLM and returns a TaskResult.
	// The context can be used for cancellation and timeout.
	Execute(ctx context.Context, card *TaskCard) (*TaskResult, error)

	// ExecuteBatch sends multiple TaskCards and returns their results.
	// Implementations may optimize batch processing.
	ExecuteBatch(ctx context.Context, cards []*TaskCard) ([]*TaskResult, error)

	// IsAvailable returns true if the provider is ready to accept requests.
	IsAvailable() bool

	// Close releases any resources held by the adapter.
	Close() error
}

// AdapterConfig holds configuration for creating a provider adapter.
type AdapterConfig struct {
	// BaseURL is the API endpoint URL.
	BaseURL string

	// APIKey is the authentication key.
	APIKey string

	// Model is the model identifier to use.
	Model string

	// Timeout is the request timeout.
	Timeout time.Duration

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// HTTPClient is an optional custom HTTP client.
	HTTPClient *http.Client

	// SystemPromptPath is an optional path to a text file containing the system prompt.
	// If empty or unreadable, the built-in default prompt is used.
	SystemPromptPath string
}

// AdapterConfigFromConfig creates an AdapterConfig from a Config struct.
func AdapterConfigFromConfig(cfg *config.Config) *AdapterConfig {
	return &AdapterConfig{
		BaseURL:          cfg.Provider.BaseURL,
		APIKey:           cfg.Provider.APIKey,
		Model:            cfg.Provider.Model,
		Timeout:          time.Duration(cfg.Runtime.RequestTimeoutSec) * time.Second,
		MaxRetries:       3,
		SystemPromptPath: cfg.Provider.SystemPromptPath,
	}
}

// defaultSystemPrompt is the built-in fallback system prompt.
const defaultSystemPrompt = "You are a code relevance classifier. Given a file and a task description, classify the file's role in relation to the task.\nRespond only with valid JSON matching the requested format. Do not add any explanation outside the JSON object."

// OpenAICompatibleAdapter implements ProviderAdapter for OpenAI-compatible APIs.
// This works with any backend that follows the OpenAI chat completions API format.
type OpenAICompatibleAdapter struct {
	config       *AdapterConfig
	httpClient   *http.Client
	systemPrompt string
}

// NewOpenAICompatibleAdapter creates a new OpenAI-compatible adapter.
func NewOpenAICompatibleAdapter(cfg *AdapterConfig) *OpenAICompatibleAdapter {
	if cfg == nil {
		cfg = &AdapterConfig{
			Timeout:    30 * time.Second,
			MaxRetries: 3,
		}
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: cfg.Timeout,
		}
	}

	systemPrompt := loadSystemPrompt(cfg.SystemPromptPath)

	return &OpenAICompatibleAdapter{
		config:       cfg,
		httpClient:   httpClient,
		systemPrompt: systemPrompt,
	}
}

// loadSystemPrompt reads the system prompt from path, falling back to the default.
func loadSystemPrompt(path string) string {
	if path != "" {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return strings.TrimSpace(string(data))
		}
	}
	return defaultSystemPrompt
}

// chatCompletionRequest represents an OpenAI chat completion request.
type chatCompletionRequest struct {
	Model    string          `json:"model"`
	Messages []chatMessage   `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *requestOptions `json:"options,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type requestOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// chatCompletionResponse represents an OpenAI chat completion response.
type chatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Execute sends a TaskCard to the LLM and returns a TaskResult.
func (a *OpenAICompatibleAdapter) Execute(ctx context.Context, card *TaskCard) (*TaskResult, error) {
	if card == nil {
		return nil, fmt.Errorf("task card is nil")
	}

	// Build the chat completion request
	req := &chatCompletionRequest{
		Model: a.config.Model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: a.systemPrompt,
			},
			{
				Role:    "user",
				Content: card.ToPrompt(),
			},
		},
		Stream: false,
		Options: &requestOptions{
			Temperature: 0.3, // Lower temperature for more deterministic results
		},
	}

	// Make the HTTP request
	respBody, err := a.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Parse the response
	var resp chatCompletionResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API errors
	if resp.Error != nil {
		return nil, fmt.Errorf("API error: %s (type: %s, code: %s)",
			resp.Error.Message, resp.Error.Type, resp.Error.Code)
	}

	// Extract the content
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := resp.Choices[0].Message.Content

	// Parse the content into a TaskResult
	result, err := ParseTaskResult(card.Type, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse task result: %w", err)
	}

	return result, nil
}

// ExecuteBatch sends multiple TaskCards and returns their results.
func (a *OpenAICompatibleAdapter) ExecuteBatch(ctx context.Context, cards []*TaskCard) ([]*TaskResult, error) {
	if len(cards) == 0 {
		return []*TaskResult{}, nil
	}

	results := make([]*TaskResult, len(cards))
	errors := make([]error, len(cards))

	// Execute each card individually
	// Future optimization: use goroutines with concurrency control
	for i, card := range cards {
		results[i], errors[i] = a.Execute(ctx, card)
	}

	// Return the first error encountered, but include partial results
	for _, err := range errors {
		if err != nil {
			return results, fmt.Errorf("batch execution had errors: %w", err)
		}
	}

	return results, nil
}

// IsAvailable returns true if the adapter can accept requests.
func (a *OpenAICompatibleAdapter) IsAvailable() bool {
	return a.config != nil && a.config.BaseURL != ""
}

// Close releases resources.
func (a *OpenAICompatibleAdapter) Close() error {
	// Nothing to close for HTTP client
	return nil
}

// doRequest makes an HTTP request to the OpenAI-compatible API.
func (a *OpenAICompatibleAdapter) doRequest(ctx context.Context, req *chatCompletionRequest) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := a.config.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if a.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.config.APIKey)
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// MockAdapter is a mock implementation of ProviderAdapter for testing.
// It returns predefined responses without making actual API calls.
type MockAdapter struct {
	// Responses is a map from task type to predefined response.
	Responses map[TaskType]*TaskResult

	// ExecuteFunc is an optional custom execute function.
	ExecuteFunc func(ctx context.Context, card *TaskCard) (*TaskResult, error)

	// CallCount tracks how many times Execute was called.
	CallCount int

	// LastCard tracks the last TaskCard passed to Execute.
	LastCard *TaskCard

	// available controls the return value of IsAvailable.
	available bool
}

// NewMockAdapter creates a new mock adapter with default responses.
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		Responses: map[TaskType]*TaskResult{
			TaskClassifyFileRole: {
				Type:           TaskClassifyFileRole,
				Classification: "main_chain",
				Confidence:     0.85,
				Reason:         "Mock response: file appears to be core to the task",
			},
			TaskJudgeRelevance: {
				Type:       TaskJudgeRelevance,
				Relevance:  "relevant",
				Confidence: 0.9,
				Reason:     "Mock response: file is directly related to the task",
			},
			TaskShouldExpand: {
				Type:       TaskShouldExpand,
				Decision:   "expand",
				Confidence: 0.7,
				Reason:     "Mock response: file has relevant neighbors",
			},
			TaskIsImplicitDependency: {
				Type:       TaskIsImplicitDependency,
				IsImplicit: "yes",
				Confidence: 0.6,
				Reason:     "Mock response: file is an implicit dependency",
			},
		},
		available: true,
	}
}

// Execute returns a predefined or custom response.
func (m *MockAdapter) Execute(ctx context.Context, card *TaskCard) (*TaskResult, error) {
	m.CallCount++
	m.LastCard = card

	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, card)
	}

	if result, ok := m.Responses[card.Type]; ok {
		return result, nil
	}

	return &TaskResult{
		Type:       card.Type,
		Confidence: 0.5,
		Reason:     "Mock response: unknown task type",
	}, nil
}

// ExecuteBatch executes multiple cards.
func (m *MockAdapter) ExecuteBatch(ctx context.Context, cards []*TaskCard) ([]*TaskResult, error) {
	results := make([]*TaskResult, len(cards))
	for i, card := range cards {
		result, err := m.Execute(ctx, card)
		if err != nil {
			return results, err
		}
		results[i] = result
	}
	return results, nil
}

// IsAvailable returns whether the mock adapter is available.
func (m *MockAdapter) IsAvailable() bool {
	return m.available
}

// SetAvailable sets the available status.
func (m *MockAdapter) SetAvailable(available bool) {
	m.available = available
}

// Close is a no-op for the mock adapter.
func (m *MockAdapter) Close() error {
	return nil
}

// SetResponse sets a predefined response for a task type.
func (m *MockAdapter) SetResponse(taskType TaskType, result *TaskResult) {
	m.Responses[taskType] = result
}

// Assert interface compliance at compile time.
var (
	_ ProviderAdapter = (*OpenAICompatibleAdapter)(nil)
	_ ProviderAdapter = (*MockAdapter)(nil)
)
