// Package schema defines core data structures for RepoScout.
package schema

import (
	"encoding/json"
	"fmt"
)

// ReconRequest represents a repository reconnaissance request.
// It defines what the user wants to understand about a codebase.
type ReconRequest struct {
	// Task describes what the user wants to accomplish (required).
	// Example: "Add a new REST API endpoint for user authentication"
	Task string `json:"task"`

	// RepoRoot is the root directory of the repository to analyze (required).
	RepoRoot string `json:"repo_root"`

	// Profile hints at the type of analysis to perform (optional).
	// Examples: "feature-add", "bug-fix", "refactor", "understand"
	Profile string `json:"profile,omitempty"`

	// SeedFiles are initial files the user already knows are relevant (optional).
	// These serve as starting points for candidate expansion.
	SeedFiles []string `json:"seed_files,omitempty"`

	// FocusSymbols are specific symbols (functions, types, etc.) to focus on (optional).
	FocusSymbols []string `json:"focus_symbols,omitempty"`

	// FocusChecks are specific checks or aspects to pay attention to (optional).
	// Examples: "security", "performance", "error-handling", "thread-safety"
	FocusChecks []string `json:"focus_checks,omitempty"`

	// Budget constrains the analysis scope (optional).
	Budget *Budget `json:"budget,omitempty"`
}

// Budget constrains the scope of reconnaissance.
type Budget struct {
	// MaxFiles limits the number of files to analyze.
	MaxFiles int `json:"max_files,omitempty"`
	// MaxTokens limits the total token count for model-based analysis.
	MaxTokens int `json:"max_tokens,omitempty"`
	// MaxTimeSec limits the total analysis time in seconds.
	MaxTimeSec int `json:"max_time_sec,omitempty"`
}

// ParseReconRequest parses a JSON byte slice into a ReconRequest.
func ParseReconRequest(data []byte) (*ReconRequest, error) {
	var req ReconRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to parse recon request: %w", err)
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return &req, nil
}

// Validate checks if the ReconRequest has all required fields.
func (r *ReconRequest) Validate() error {
	if r.Task == "" {
		return fmt.Errorf("recon request: task is required")
	}
	if r.RepoRoot == "" {
		return fmt.Errorf("recon request: repo_root is required")
	}
	if r.Budget != nil {
		if r.Budget.MaxFiles < 0 {
			return fmt.Errorf("recon request: budget.max_files cannot be negative")
		}
		if r.Budget.MaxTokens < 0 {
			return fmt.Errorf("recon request: budget.max_tokens cannot be negative")
		}
		if r.Budget.MaxTimeSec < 0 {
			return fmt.Errorf("recon request: budget.max_time_sec cannot be negative")
		}
	}
	return nil
}

// ToJSON returns the ReconRequest as formatted JSON.
func (r *ReconRequest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
