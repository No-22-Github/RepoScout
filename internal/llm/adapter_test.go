package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/no22/repo-scout/internal/config"
	"github.com/no22/repo-scout/internal/schema"
)

func TestAdapterConfigFromConfig(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderConfig{
			BaseURL:  "https://api.example.com/v1",
			APIKey:   "test-key",
			Model:    "test-model",
			APIStyle: "openai",
		},
		Runtime: config.RuntimeConfig{
			RequestTimeoutSec: 60,
		},
	}

	adapterCfg := AdapterConfigFromConfig(cfg)

	if adapterCfg.BaseURL != "https://api.example.com/v1" {
		t.Errorf("BaseURL = %q, want %q", adapterCfg.BaseURL, "https://api.example.com/v1")
	}
	if adapterCfg.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", adapterCfg.APIKey, "test-key")
	}
	if adapterCfg.Model != "test-model" {
		t.Errorf("Model = %q, want %q", adapterCfg.Model, "test-model")
	}
	if adapterCfg.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v", adapterCfg.Timeout, 60*time.Second)
	}
}

func TestNewOpenAICompatibleAdapter(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		cfg := &AdapterConfig{
			BaseURL:    "https://api.example.com/v1",
			APIKey:     "test-key",
			Model:      "test-model",
			Timeout:    30 * time.Second,
			MaxRetries: 3,
		}

		adapter := NewOpenAICompatibleAdapter(cfg)

		if adapter.config.BaseURL != "https://api.example.com/v1" {
			t.Errorf("BaseURL = %q, want %q", adapter.config.BaseURL, "https://api.example.com/v1")
		}
		if adapter.httpClient == nil {
			t.Error("httpClient should not be nil")
		}
	})

	t.Run("with nil config", func(t *testing.T) {
		adapter := NewOpenAICompatibleAdapter(nil)

		if adapter.config == nil {
			t.Error("config should not be nil after initialization")
		}
		if adapter.config.Timeout != 30*time.Second {
			t.Errorf("default Timeout = %v, want %v", adapter.config.Timeout, 30*time.Second)
		}
	})

	t.Run("with custom http client", func(t *testing.T) {
		customClient := &http.Client{Timeout: 5 * time.Second}
		cfg := &AdapterConfig{
			HTTPClient: customClient,
		}

		adapter := NewOpenAICompatibleAdapter(cfg)

		if adapter.httpClient != customClient {
			t.Error("should use custom HTTP client")
		}
	})
}

func TestOpenAICompatibleAdapterIsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		config   *AdapterConfig
		expected bool
	}{
		{
			name: "available with valid config",
			config: &AdapterConfig{
				BaseURL: "https://api.example.com/v1",
			},
			expected: true,
		},
		{
			name:     "not available with nil config",
			config:   nil,
			expected: false,
		},
		{
			name: "not available with empty base URL",
			config: &AdapterConfig{
				BaseURL: "",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewOpenAICompatibleAdapter(tt.config)
			if got := adapter.IsAvailable(); got != tt.expected {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOpenAICompatibleAdapterExecute(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Path = %s, want /chat/completions", r.URL.Path)
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-api-key")
		}

		// Parse request body
		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		// Send response
		resp := chatCompletionResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "test-model",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: `{"classification": "main_chain", "confidence": 0.9, "reason": "Test response"}`,
					},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create adapter
	cfg := &AdapterConfig{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
		Model:   "test-model",
		Timeout: 10 * time.Second,
	}
	adapter := NewOpenAICompatibleAdapter(cfg)

	// Create task card
	fc := schema.NewFileCard("internal/handler.go")
	card := NewTaskCard(TaskClassifyFileRole, "Test task", fc)

	// Execute
	ctx := context.Background()
	result, err := adapter.Execute(ctx, card)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify result
	if result.Type != TaskClassifyFileRole {
		t.Errorf("Type = %q, want %q", result.Type, TaskClassifyFileRole)
	}
	if result.Classification != "main_chain" {
		t.Errorf("Classification = %q, want %q", result.Classification, "main_chain")
	}
	if result.Confidence != 0.9 {
		t.Errorf("Confidence = %f, want %f", result.Confidence, 0.9)
	}
}

func TestOpenAICompatibleAdapterExecuteBatch(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		var resp chatCompletionResponse
		switch callCount {
		case 1:
			resp = chatCompletionResponse{
				Choices: []struct {
					Index   int `json:"index"`
					Message struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				}{
					{
						Message: struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						}{
							Content: `{"classification": "main_chain", "confidence": 0.8, "reason": "First"}`,
						},
					},
				},
			}
		case 2:
			resp = chatCompletionResponse{
				Choices: []struct {
					Index   int `json:"index"`
					Message struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				}{
					{
						Message: struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						}{
							Content: `{"classification": "companion", "confidence": 0.7, "reason": "Second"}`,
						},
					},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &AdapterConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 10 * time.Second,
	}
	adapter := NewOpenAICompatibleAdapter(cfg)

	cards := []*TaskCard{
		NewTaskCard(TaskClassifyFileRole, "Task 1", schema.NewFileCard("file1.go")),
		NewTaskCard(TaskClassifyFileRole, "Task 2", schema.NewFileCard("file2.go")),
	}

	ctx := context.Background()
	results, err := adapter.ExecuteBatch(ctx, cards)
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Classification != "main_chain" {
		t.Errorf("results[0].Classification = %q, want %q", results[0].Classification, "main_chain")
	}
	if results[1].Classification != "companion" {
		t.Errorf("results[1].Classification = %q, want %q", results[1].Classification, "companion")
	}
}

func TestOpenAICompatibleAdapterExecuteError(t *testing.T) {
	t.Run("nil card", func(t *testing.T) {
		adapter := NewOpenAICompatibleAdapter(&AdapterConfig{BaseURL: "http://example.com"})
		_, err := adapter.Execute(context.Background(), nil)
		if err == nil {
			t.Error("Execute with nil card should return error")
		}
	})

	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := chatCompletionResponse{
				Error: &struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Code    string `json:"code"`
				}{
					Message: "Invalid API key",
					Type:    "invalid_request_error",
					Code:    "invalid_api_key",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		adapter := NewOpenAICompatibleAdapter(&AdapterConfig{
			BaseURL: server.URL,
			Model:   "test-model",
		})

		card := NewTaskCard(TaskClassifyFileRole, "test", schema.NewFileCard("test.go"))
		_, err := adapter.Execute(context.Background(), card)
		if err == nil {
			t.Error("Execute should return error for API error response")
		}
		if !strings.Contains(err.Error(), "Invalid API key") {
			t.Errorf("Error should contain 'Invalid API key', got: %v", err)
		}
	})

	t.Run("HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
		}))
		defer server.Close()

		adapter := NewOpenAICompatibleAdapter(&AdapterConfig{
			BaseURL: server.URL,
			Model:   "test-model",
		})

		card := NewTaskCard(TaskClassifyFileRole, "test", schema.NewFileCard("test.go"))
		_, err := adapter.Execute(context.Background(), card)
		if err == nil {
			t.Error("Execute should return error for HTTP 500")
		}
	})

	t.Run("empty choices", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := chatCompletionResponse{
				Choices: []struct {
					Index   int `json:"index"`
					Message struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				}{},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		adapter := NewOpenAICompatibleAdapter(&AdapterConfig{
			BaseURL: server.URL,
			Model:   "test-model",
		})

		card := NewTaskCard(TaskClassifyFileRole, "test", schema.NewFileCard("test.go"))
		_, err := adapter.Execute(context.Background(), card)
		if err == nil {
			t.Error("Execute should return error for empty choices")
		}
	})
}

func TestOpenAICompatibleAdapterClose(t *testing.T) {
	adapter := NewOpenAICompatibleAdapter(nil)
	if err := adapter.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// MockAdapter tests

func TestNewMockAdapter(t *testing.T) {
	adapter := NewMockAdapter()

	// Should have default responses for all task types
	requiredTypes := []TaskType{
		TaskClassifyFileRole,
		TaskJudgeRelevance,
		TaskShouldExpand,
		TaskIsImplicitDependency,
	}

	for _, tt := range requiredTypes {
		if _, ok := adapter.Responses[tt]; !ok {
			t.Errorf("MockAdapter missing default response for %s", tt)
		}
	}

	if !adapter.IsAvailable() {
		t.Error("New MockAdapter should be available")
	}
}

func TestMockAdapterExecute(t *testing.T) {
	t.Run("predefined response", func(t *testing.T) {
		adapter := NewMockAdapter()

		card := NewTaskCard(TaskClassifyFileRole, "test task", schema.NewFileCard("test.go"))
		result, err := adapter.Execute(context.Background(), card)

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if result.Classification != "main_chain" {
			t.Errorf("Classification = %q, want %q", result.Classification, "main_chain")
		}
	})

	t.Run("custom execute function", func(t *testing.T) {
		adapter := NewMockAdapter()
		adapter.ExecuteFunc = func(ctx context.Context, card *TaskCard) (*TaskResult, error) {
			return &TaskResult{
				Type:           card.Type,
				Classification: "custom_classification",
				Confidence:     0.99,
				Reason:         "Custom response",
			}, nil
		}

		card := NewTaskCard(TaskClassifyFileRole, "test", schema.NewFileCard("test.go"))
		result, err := adapter.Execute(context.Background(), card)

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if result.Classification != "custom_classification" {
			t.Errorf("Classification = %q, want %q", result.Classification, "custom_classification")
		}
	})

	t.Run("unknown task type", func(t *testing.T) {
		adapter := NewMockAdapter()
		delete(adapter.Responses, TaskClassifyFileRole)

		card := NewTaskCard(TaskClassifyFileRole, "test", schema.NewFileCard("test.go"))
		result, err := adapter.Execute(context.Background(), card)

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if result.Confidence != 0.5 {
			t.Errorf("Confidence = %f, want %f", result.Confidence, 0.5)
		}
	})

	t.Run("tracking calls", func(t *testing.T) {
		adapter := NewMockAdapter()

		card := NewTaskCard(TaskClassifyFileRole, "test task", schema.NewFileCard("test.go"))
		adapter.Execute(context.Background(), card)
		adapter.Execute(context.Background(), card)

		if adapter.CallCount != 2 {
			t.Errorf("CallCount = %d, want 2", adapter.CallCount)
		}
		if adapter.LastCard != card {
			t.Error("LastCard should be set")
		}
	})
}

func TestMockAdapterExecuteBatch(t *testing.T) {
	adapter := NewMockAdapter()

	cards := []*TaskCard{
		NewTaskCard(TaskClassifyFileRole, "task1", schema.NewFileCard("file1.go")),
		NewTaskCard(TaskJudgeRelevance, "task2", schema.NewFileCard("file2.go")),
	}

	results, err := adapter.ExecuteBatch(context.Background(), cards)
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if adapter.CallCount != 2 {
		t.Errorf("CallCount = %d, want 2", adapter.CallCount)
	}
}

func TestMockAdapterSetAvailable(t *testing.T) {
	adapter := NewMockAdapter()

	if !adapter.IsAvailable() {
		t.Error("New MockAdapter should be available")
	}

	adapter.SetAvailable(false)
	if adapter.IsAvailable() {
		t.Error("MockAdapter should not be available after SetAvailable(false)")
	}

	adapter.SetAvailable(true)
	if !adapter.IsAvailable() {
		t.Error("MockAdapter should be available after SetAvailable(true)")
	}
}

func TestMockAdapterSetResponse(t *testing.T) {
	adapter := NewMockAdapter()

	newResult := &TaskResult{
		Type:           TaskClassifyFileRole,
		Classification: "new_classification",
		Confidence:     0.95,
		Reason:         "Updated response",
	}

	adapter.SetResponse(TaskClassifyFileRole, newResult)

	card := NewTaskCard(TaskClassifyFileRole, "test", schema.NewFileCard("test.go"))
	result, err := adapter.Execute(context.Background(), card)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Classification != "new_classification" {
		t.Errorf("Classification = %q, want %q", result.Classification, "new_classification")
	}
	if result.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want %f", result.Confidence, 0.95)
	}
}

func TestMockAdapterClose(t *testing.T) {
	adapter := NewMockAdapter()
	if err := adapter.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestProviderAdapterInterface(t *testing.T) {
	// Verify both adapters implement the interface
	var _ ProviderAdapter = NewOpenAICompatibleAdapter(nil)
	var _ ProviderAdapter = NewMockAdapter()
}

func TestOpenAICompatibleAdapterContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	cfg := &AdapterConfig{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 10 * time.Second,
	}
	adapter := NewOpenAICompatibleAdapter(cfg)

	// Create context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	card := NewTaskCard(TaskClassifyFileRole, "test", schema.NewFileCard("test.go"))
	_, err := adapter.Execute(ctx, card)

	if err == nil {
		t.Error("Execute should return error when context is cancelled")
	}
}
