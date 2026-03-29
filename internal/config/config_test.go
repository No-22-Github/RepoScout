package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Provider.BaseURL == "" {
		t.Error("default provider.base_url should not be empty")
	}
	if cfg.Provider.APIStyle == "" {
		t.Error("default provider.api_style should not be empty")
	}
	if cfg.Runtime.MaxConcurrency <= 0 {
		t.Error("default runtime.max_concurrency should be positive")
	}
	if cfg.Runtime.RequestTimeoutSec <= 0 {
		t.Error("default runtime.request_timeout_sec should be positive")
	}
	if cfg.Runtime.MaxInputTokens <= 0 {
		t.Error("default runtime.max_input_tokens should be positive")
	}
	if cfg.Runtime.MaxCandidates <= 0 {
		t.Error("default runtime.max_candidates should be positive")
	}
	if cfg.Runtime.MaxOutputFiles <= 0 {
		t.Error("default runtime.max_output_files should be positive")
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	cfg, err := Load("/non/existent/path/config.json")
	if err != nil {
		t.Errorf("loading non-existent file should return defaults, got error: %v", err)
	}
	if cfg.Provider.BaseURL == "" {
		t.Error("default config should have non-empty base_url")
	}
}

func TestLoadValidFile(t *testing.T) {
	content := `{
		"provider": {
			"base_url": "https://custom.api/v1",
			"api_key": "test-key",
			"model": "custom-model",
			"api_style": "custom"
		},
		"runtime": {
			"max_concurrency": 8,
			"request_timeout_sec": 60,
			"max_input_tokens": 8192,
			"max_candidates": 200,
			"max_output_files": 100,
			"enable_model_rerank": false
		}
	}`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Provider.BaseURL != "https://custom.api/v1" {
		t.Errorf("expected custom base_url, got %s", cfg.Provider.BaseURL)
	}
	if cfg.Provider.APIKey != "test-key" {
		t.Errorf("expected test-key, got %s", cfg.Provider.APIKey)
	}
	if cfg.Provider.Model != "custom-model" {
		t.Errorf("expected custom-model, got %s", cfg.Provider.Model)
	}
	if cfg.Provider.APIStyle != "custom" {
		t.Errorf("expected custom api_style, got %s", cfg.Provider.APIStyle)
	}
	if cfg.Runtime.MaxConcurrency != 8 {
		t.Errorf("expected max_concurrency 8, got %d", cfg.Runtime.MaxConcurrency)
	}
	if cfg.Runtime.RequestTimeoutSec != 60 {
		t.Errorf("expected request_timeout_sec 60, got %d", cfg.Runtime.RequestTimeoutSec)
	}
	if cfg.Runtime.MaxInputTokens != 8192 {
		t.Errorf("expected max_input_tokens 8192, got %d", cfg.Runtime.MaxInputTokens)
	}
	if cfg.Runtime.MaxCandidates != 200 {
		t.Errorf("expected max_candidates 200, got %d", cfg.Runtime.MaxCandidates)
	}
	if cfg.Runtime.MaxOutputFiles != 100 {
		t.Errorf("expected max_output_files 100, got %d", cfg.Runtime.MaxOutputFiles)
	}
	if cfg.Runtime.EnableModelRerank {
		t.Error("expected enable_model_rerank to be false")
	}
}

func TestLoadWithEnvOverride(t *testing.T) {
	os.Setenv("REPOSCOUT_PROVIDER_BASE_URL", "https://env.override/v1")
	os.Setenv("REPOSCOUT_PROVIDER_API_KEY", "env-key")
	os.Setenv("REPOSCOUT_RUNTIME_MAX_CONCURRENCY", "16")
	os.Setenv("REPOSCOUT_RUNTIME_MAX_INPUT_TOKENS", "2048")
	defer func() {
		os.Unsetenv("REPOSCOUT_PROVIDER_BASE_URL")
		os.Unsetenv("REPOSCOUT_PROVIDER_API_KEY")
		os.Unsetenv("REPOSCOUT_RUNTIME_MAX_CONCURRENCY")
		os.Unsetenv("REPOSCOUT_RUNTIME_MAX_INPUT_TOKENS")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Provider.BaseURL != "https://env.override/v1" {
		t.Errorf("expected env override base_url, got %s", cfg.Provider.BaseURL)
	}
	if cfg.Provider.APIKey != "env-key" {
		t.Errorf("expected env-key, got %s", cfg.Provider.APIKey)
	}
	if cfg.Runtime.MaxConcurrency != 16 {
		t.Errorf("expected max_concurrency 16, got %d", cfg.Runtime.MaxConcurrency)
	}
	if cfg.Runtime.MaxInputTokens != 2048 {
		t.Errorf("expected max_input_tokens 2048, got %d", cfg.Runtime.MaxInputTokens)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "empty base_url is allowed in static mode",
			cfg: &Config{
				Provider: ProviderConfig{BaseURL: ""},
				Runtime:  RuntimeConfig{MaxConcurrency: 4, RequestTimeoutSec: 30, MaxInputTokens: 4096, MaxCandidates: 100, MaxOutputFiles: 50},
			},
			wantErr: false,
		},
		{
			name: "zero max_concurrency",
			cfg: &Config{
				Provider: ProviderConfig{BaseURL: "https://api.example.com/v1"},
				Runtime:  RuntimeConfig{MaxConcurrency: 0, RequestTimeoutSec: 30, MaxInputTokens: 4096, MaxCandidates: 100, MaxOutputFiles: 50},
			},
			wantErr: true,
		},
		{
			name: "negative max_candidates",
			cfg: &Config{
				Provider: ProviderConfig{BaseURL: "https://api.example.com/v1"},
				Runtime:  RuntimeConfig{MaxConcurrency: 4, RequestTimeoutSec: 30, MaxInputTokens: 4096, MaxCandidates: -1, MaxOutputFiles: 50},
			},
			wantErr: true,
		},
		{
			name: "zero max_input_tokens",
			cfg: &Config{
				Provider: ProviderConfig{BaseURL: "https://api.example.com/v1"},
				Runtime:  RuntimeConfig{MaxConcurrency: 4, RequestTimeoutSec: 30, MaxInputTokens: 0, MaxCandidates: 100, MaxOutputFiles: 50},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToJSON(t *testing.T) {
	cfg := DefaultConfig()
	data, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("ToJSON should return non-empty data")
	}
}
