// Package schema defines core data structures for RepoScout.
package schema

import (
	"encoding/json"
)

// ContextPack is the final output of RepoScout reconnaissance.
// It contains the curated context to be passed to an upstream coding Agent.
type ContextPack struct {
	// Task echoes the original task description from the request.
	Task string `json:"task"`

	// RepoFamily identifies the high-level technology stack or framework.
	// Examples: "go-web", "react-ts", "python-ml", "cpp-embedded"
	RepoFamily string `json:"repo_family,omitempty"`

	// MainChain contains the primary files that form the main execution path
	// relevant to the task. These should be read first and in order.
	MainChain []string `json:"main_chain"`

	// CompanionFiles are supporting files that provide context but are not
	// part of the main execution path. Examples: config, types, helpers.
	CompanionFiles []string `json:"companion_files"`

	// UncertainNodes are files that might be relevant but the system
	// couldn't determine with high confidence. May need human review.
	UncertainNodes []string `json:"uncertain_nodes,omitempty"`

	// ReadingOrder provides a recommended sequence for reading the files.
	// This is a flat list combining main_chain and companion_files in
	// an optimal reading sequence.
	ReadingOrder []string `json:"reading_order"`

	// RiskHints contains warnings and risk indicators discovered during
	// analysis. These help the Agent understand potential pitfalls.
	RiskHints []*RiskHint `json:"risk_hints,omitempty"`

	// SummaryMarkdown is a human-readable summary of the ContextPack
	// in Markdown format. Can be used directly by Agents or for debugging.
	SummaryMarkdown string `json:"summary_markdown,omitempty"`

	// Stats contains statistics about the reconnaissance process.
	Stats *PackStats `json:"stats,omitempty"`
}

// RiskHint represents a potential risk or warning discovered during analysis.
type RiskHint struct {
	// Level indicates the severity: "info", "warning", "error".
	Level string `json:"level"`

	// Category classifies the risk type.
	// Examples: "test-coverage", "complexity", "dependencies", "security"
	Category string `json:"category"`

	// Message is a human-readable description of the risk.
	Message string `json:"message"`

	// AffectedFiles lists files related to this risk.
	AffectedFiles []string `json:"affected_files,omitempty"`
}

// PackStats contains statistics about the ContextPack generation.
type PackStats struct {
	// TotalFiles is the total number of files in the candidate set.
	TotalFiles int `json:"total_files,omitempty"`

	// MainChainCount is the number of files in the main chain.
	MainChainCount int `json:"main_chain_count,omitempty"`

	// CompanionCount is the number of companion files.
	CompanionCount int `json:"companion_count,omitempty"`

	// UncertainCount is the number of uncertain nodes.
	UncertainCount int `json:"uncertain_count,omitempty"`

	// AnalysisTimeMs is the total analysis time in milliseconds.
	AnalysisTimeMs int64 `json:"analysis_time_ms,omitempty"`

	// ModelEnhanced indicates if LLM was used for enhancement.
	ModelEnhanced bool `json:"model_enhanced,omitempty"`
}

// NewContextPack creates a new ContextPack with initialized slices.
func NewContextPack(task string) *ContextPack {
	return &ContextPack{
		Task:           task,
		MainChain:      []string{},
		CompanionFiles: []string{},
		UncertainNodes: []string{},
		ReadingOrder:   []string{},
		RiskHints:      []*RiskHint{},
		Stats:          &PackStats{},
	}
}

// AddMainChain adds files to the main chain.
func (cp *ContextPack) AddMainChain(paths ...string) {
	for _, p := range paths {
		for _, existing := range cp.MainChain {
			if existing == p {
				return
			}
		}
		cp.MainChain = append(cp.MainChain, p)
	}
}

// AddCompanion adds files to the companion list.
func (cp *ContextPack) AddCompanion(paths ...string) {
	for _, p := range paths {
		for _, existing := range cp.CompanionFiles {
			if existing == p {
				return
			}
		}
		cp.CompanionFiles = append(cp.CompanionFiles, p)
	}
}

// AddUncertain adds files to the uncertain list.
func (cp *ContextPack) AddUncertain(paths ...string) {
	for _, p := range paths {
		for _, existing := range cp.UncertainNodes {
			if existing == p {
				return
			}
		}
		cp.UncertainNodes = append(cp.UncertainNodes, p)
	}
}

// AddRiskHint adds a risk hint to the ContextPack.
func (cp *ContextPack) AddRiskHint(level, category, message string, affectedFiles ...string) {
	hint := &RiskHint{
		Level:         level,
		Category:      category,
		Message:       message,
		AffectedFiles: affectedFiles,
	}
	cp.RiskHints = append(cp.RiskHints, hint)
}

// SetReadingOrder sets the recommended reading order.
// It validates that all files in the order exist in main_chain or companion_files.
func (cp *ContextPack) SetReadingOrder(order []string) {
	cp.ReadingOrder = order
}

// UpdateStats updates the statistics based on current content.
func (cp *ContextPack) UpdateStats() {
	cp.Stats.MainChainCount = len(cp.MainChain)
	cp.Stats.CompanionCount = len(cp.CompanionFiles)
	cp.Stats.UncertainCount = len(cp.UncertainNodes)
	cp.Stats.TotalFiles = cp.Stats.MainChainCount + cp.Stats.CompanionCount + cp.Stats.UncertainCount
}

// ToJSON returns the ContextPack as formatted JSON.
func (cp *ContextPack) ToJSON() ([]byte, error) {
	return json.MarshalIndent(cp, "", "  ")
}

// AllFiles returns all files across main_chain, companion_files, and uncertain_nodes.
func (cp *ContextPack) AllFiles() []string {
	total := len(cp.MainChain) + len(cp.CompanionFiles) + len(cp.UncertainNodes)
	files := make([]string, 0, total)
	files = append(files, cp.MainChain...)
	files = append(files, cp.CompanionFiles...)
	files = append(files, cp.UncertainNodes...)
	return files
}
