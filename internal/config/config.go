// Package config provides unified runtime configuration for RepoScout CLI and MCP.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProviderConfig holds LLM provider configuration.
type ProviderConfig struct {
	// BaseURL is the API endpoint URL (e.g., "https://api.openai.com/v1").
	BaseURL string `json:"base_url"`
	// APIKey is the authentication key for the provider.
	APIKey string `json:"api_key"`
	// Model is the model identifier to use.
	Model string `json:"model"`
	// APIStyle indicates the API format (e.g., "openai", "anthropic").
	APIStyle string `json:"api_style"`
}

// RuntimeConfig holds runtime behavior configuration.
type RuntimeConfig struct {
	// MaxConcurrency limits concurrent operations.
	MaxConcurrency int `json:"max_concurrency"`
	// RequestTimeoutSec is the timeout for API requests in seconds.
	RequestTimeoutSec int `json:"request_timeout_sec"`
	// MaxInputTokens is the prompt budget used when assembling LLM input.
	// RepoScout preserves the metadata prompt and uses the remaining budget
	// to attach code snippets and other context.
	MaxInputTokens int `json:"max_input_tokens"`
	// MaxCandidates limits the number of candidate files.
	MaxCandidates int `json:"max_candidates"`
	// MaxOutputFiles limits the number of files in output.
	MaxOutputFiles int `json:"max_output_files"`
	// EnableModelRerank enables model-based reranking on the static candidate set.
	EnableModelRerank bool `json:"enable_model_rerank"`
}

// Config is the unified configuration structure for RepoScout.
type Config struct {
	Provider ProviderConfig `json:"provider"`
	Runtime  RuntimeConfig  `json:"runtime"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Provider: ProviderConfig{
			BaseURL:  "https://api.openai.com/v1",
			APIKey:   "",
			Model:    "gpt-4",
			APIStyle: "openai",
		},
		Runtime: RuntimeConfig{
			MaxConcurrency:    4,
			RequestTimeoutSec: 30,
			MaxInputTokens:    4096,
			MaxCandidates:     100,
			MaxOutputFiles:    50,
			EnableModelRerank: false,
		},
	}
}

// Load reads configuration from a JSON file, falling back to defaults.
// Environment variables can override file values.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Try to load from file if path is provided
	if path != "" {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve config path: %w", err)
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				// File doesn't exist, use defaults
				return cfg, nil
			}
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Override with environment variables
	if v := os.Getenv("REPOSCOUT_PROVIDER_BASE_URL"); v != "" {
		cfg.Provider.BaseURL = v
	}
	if v := os.Getenv("REPOSCOUT_PROVIDER_API_KEY"); v != "" {
		cfg.Provider.APIKey = v
	}
	if v := os.Getenv("REPOSCOUT_PROVIDER_MODEL"); v != "" {
		cfg.Provider.Model = v
	}
	if v := os.Getenv("REPOSCOUT_PROVIDER_API_STYLE"); v != "" {
		cfg.Provider.APIStyle = v
	}
	if v := os.Getenv("REPOSCOUT_RUNTIME_MAX_CONCURRENCY"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.Runtime.MaxConcurrency = n
		}
	}
	if v := os.Getenv("REPOSCOUT_RUNTIME_REQUEST_TIMEOUT_SEC"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.Runtime.RequestTimeoutSec = n
		}
	}
	if v := os.Getenv("REPOSCOUT_RUNTIME_MAX_INPUT_TOKENS"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.Runtime.MaxInputTokens = n
		}
	}
	if v := os.Getenv("REPOSCOUT_RUNTIME_MAX_CANDIDATES"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.Runtime.MaxCandidates = n
		}
	}
	if v := os.Getenv("REPOSCOUT_RUNTIME_MAX_OUTPUT_FILES"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.Runtime.MaxOutputFiles = n
		}
	}
	if v := os.Getenv("REPOSCOUT_RUNTIME_ENABLE_MODEL_RERANK"); v != "" {
		cfg.Runtime.EnableModelRerank = v == "true"
	}

	return cfg, nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Runtime.MaxConcurrency <= 0 {
		return fmt.Errorf("runtime.max_concurrency must be positive")
	}
	if c.Runtime.RequestTimeoutSec <= 0 {
		return fmt.Errorf("runtime.request_timeout_sec must be positive")
	}
	if c.Runtime.MaxInputTokens <= 0 {
		return fmt.Errorf("runtime.max_input_tokens must be positive")
	}
	if c.Runtime.MaxCandidates <= 0 {
		return fmt.Errorf("runtime.max_candidates must be positive")
	}
	if c.Runtime.MaxOutputFiles <= 0 {
		return fmt.Errorf("runtime.max_output_files must be positive")
	}
	return nil
}

// ToJSON returns the configuration as formatted JSON.
func (c *Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}
