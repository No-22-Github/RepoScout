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

// LoadResult describes the resolved config and which config files were applied.
type LoadResult struct {
	Config      *Config
	LoadedPaths []string
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

// configPaths returns the layered config file paths in order of increasing priority:
// user config (~/.config/reposcout.json), repo config (.reposcout.json in repoRoot),
// and the explicitly provided path (if any).
func configPaths(explicit, repoRoot string) []string {
	var paths []string

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "reposcout.json"))
	}

	if repoRoot != "" {
		paths = append(paths, filepath.Join(repoRoot, ".reposcout.json"))
	} else if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".reposcout.json"))
	}

	if explicit != "" {
		paths = append(paths, explicit)
	}

	return paths
}

// mergeFile reads a JSON config file and merges it into cfg.
// Missing files are silently skipped; parse errors are returned.
func mergeFile(cfg *Config, path string) (bool, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("failed to resolve config path %s: %w", path, err)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return false, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}
	return true, nil
}

// LoadForRepoWithMeta reads configuration using a layered strategy (lowest → highest priority):
//  1. Built-in defaults
//  2. User config: ~/.config/reposcout.json
//  3. Repo config: .reposcout.json in repoRoot (or the current working directory if repoRoot is empty)
//  4. Explicit path (the -c / --config flag), if provided
//  5. Environment variables (REPOSCOUT_PROVIDER_* / REPOSCOUT_RUNTIME_*)
func LoadForRepoWithMeta(path, repoRoot string) (*LoadResult, error) {
	cfg := DefaultConfig()
	loadedPaths := make([]string, 0, 3)

	for _, p := range configPaths(path, repoRoot) {
		loaded, err := mergeFile(cfg, p)
		if err != nil {
			return nil, err
		}
		if loaded {
			absPath, err := filepath.Abs(p)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve config path %s: %w", p, err)
			}
			loadedPaths = append(loadedPaths, absPath)
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

	return &LoadResult{
		Config:      cfg,
		LoadedPaths: loadedPaths,
	}, nil
}

// LoadForRepo reads configuration using the target repository root for repo-level config lookup.
func LoadForRepo(path, repoRoot string) (*Config, error) {
	result, err := LoadForRepoWithMeta(path, repoRoot)
	if err != nil {
		return nil, err
	}
	return result.Config, nil
}

// Load reads configuration using the current working directory as the repo config location.
// Call LoadForRepo when the target repository root differs from the process working directory.
func Load(path string) (*Config, error) {
	return LoadForRepo(path, "")
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
