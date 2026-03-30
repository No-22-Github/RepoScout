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
	// Supported values: "tests", "default_config", "resources_or_strings",
	// "build_registration", "feature_flag"
	FocusChecks []string `json:"focus_checks,omitempty"`

	// Budget constrains the analysis scope (optional).
	Budget *Budget `json:"budget,omitempty"`
}

// Budget constrains the scope of reconnaissance.
type Budget struct {
	// MaxSeedNeighbors limits how many non-seed candidates can be collected
	// during static expansion.
	MaxSeedNeighbors int `json:"max_seed_neighbors,omitempty"`
	// ExpandDepth controls how many rounds of neighbor expansion to run.
	// Static MVP supports shallow expansion only, usually 1-2.
	ExpandDepth int `json:"expand_depth,omitempty"`
	// MaxOutputFiles limits the total number of files emitted in ContextPack.
	MaxOutputFiles int `json:"max_output_files,omitempty"`
	// MaxLLMJobs is reserved for future model-enhanced stages.
	MaxLLMJobs int `json:"max_llm_jobs,omitempty"`
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
		if r.Budget.MaxSeedNeighbors < 0 {
			return fmt.Errorf("recon request: budget.max_seed_neighbors cannot be negative")
		}
		if r.Budget.ExpandDepth < 0 {
			return fmt.Errorf("recon request: budget.expand_depth cannot be negative")
		}
		if r.Budget.MaxOutputFiles < 0 {
			return fmt.Errorf("recon request: budget.max_output_files cannot be negative")
		}
		if r.Budget.MaxLLMJobs < 0 {
			return fmt.Errorf("recon request: budget.max_llm_jobs cannot be negative")
		}
	}
	return nil
}

// EffectiveExpandDepth returns the requested expansion depth with a static-MVP default.
func (r *ReconRequest) EffectiveExpandDepth() int {
	if r == nil || r.Budget == nil || r.Budget.ExpandDepth <= 0 {
		return 1
	}
	return r.Budget.ExpandDepth
}

// EffectiveMaxSeedNeighbors returns the configured neighbor budget.
func (r *ReconRequest) EffectiveMaxSeedNeighbors() int {
	if r == nil || r.Budget == nil {
		return 0
	}
	return r.Budget.MaxSeedNeighbors
}

// EffectiveMaxOutputFiles returns the configured output budget.
func (r *ReconRequest) EffectiveMaxOutputFiles() int {
	if r == nil || r.Budget == nil {
		return 0
	}
	return r.Budget.MaxOutputFiles
}

// EffectiveMaxLLMJobs returns the configured cap for model inference jobs.
func (r *ReconRequest) EffectiveMaxLLMJobs() int {
	if r == nil || r.Budget == nil {
		return 0
	}
	if r.Budget.MaxLLMJobs > 0 {
		return r.Budget.MaxLLMJobs
	}
	return 0
}

// ToJSON returns the ReconRequest as formatted JSON.
func (r *ReconRequest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
