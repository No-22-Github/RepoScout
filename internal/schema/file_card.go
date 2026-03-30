// Package schema defines core data structures for RepoScout.
package schema

import (
	"encoding/json"
)

// FileCard represents metadata and analysis results for a single file.
// It is the primary intermediate data structure during reconnaissance.
type FileCard struct {
	// Path is the relative path from repo_root to the file.
	Path string `json:"path"`

	// Lang identifies the programming language or file type.
	// Examples: "go", "ts", "tsx", "cpp", "json", "text"
	Lang string `json:"lang,omitempty"`

	// Module is the coarse-grained module grouping for the file.
	// Used for neighborhood analysis and ranking.
	// Example: "browser/settings"
	Module string `json:"module,omitempty"`

	// Symbols contains extracted symbol names from the file.
	// Light-weight extraction, not a full AST.
	Symbols []string `json:"symbols,omitempty"`

	// Neighbors lists related files discovered through static analysis.
	// These are files that import or are imported by this file.
	Neighbors []string `json:"neighbors,omitempty"`

	// DiscoveredBy records how this file was added to the candidate set.
	// Examples: "seed", "same-directory", "same-module", "name-prefix-match"
	DiscoveredBy []string `json:"discovered_by,omitempty"`

	// HeuristicTags are labels applied by heuristic rules.
	// Examples: "tests", "default_config", "resources", "build_registration"
	HeuristicTags []string `json:"heuristic_tags,omitempty"`

	// Scores contains various scoring metrics for ranking.
	Scores *FileScores `json:"scores,omitempty"`
}

// FileScores contains scoring metrics for a file.
type FileScores struct {
	// SeedWeight is applied to files explicitly provided as seeds.
	// Range: 0.0 to 1.0, higher means more important.
	SeedWeight float64 `json:"seed_weight,omitempty"`

	// ModuleWeight reflects proximity to seed files' modules.
	// Range: 0.0 to 1.0
	ModuleWeight float64 `json:"module_weight,omitempty"`

	// HeuristicScore is the aggregate score from heuristic rules.
	// Range: 0.0 to 1.0
	HeuristicScore float64 `json:"heuristic_score,omitempty"`

	// ProfileScore is the score from profile-specific rules.
	// Range: 0.0 to 1.0
	ProfileScore float64 `json:"profile_score,omitempty"`

	// LLMLabel is the classification result from LLM analysis.
	// Examples: "main_chain", "companion", "uncertain", "irrelevant"
	LLMLabel string `json:"llm_label,omitempty"`

	// LLMConfidence is the confidence score from LLM analysis.
	// Range: 0.0 to 1.0
	LLMConfidence float64 `json:"llm_confidence,omitempty"`

	// DiscoveryScore reflects how the file was found.
	// import=0.7, sibling=0.5, symbol_hit=0.3, same_dir/prefix=0.2, same_module=0.1, seed=1.0
	DiscoveryScore float64 `json:"discovery_score,omitempty"`

	// FinalScore is the combined final score used for ranking.
	// Computed from other scores based on configurable weights.
	FinalScore float64 `json:"final_score,omitempty"`
}

// NewFileCard creates a FileCard with the given path.
func NewFileCard(path string) *FileCard {
	return &FileCard{
		Path:          path,
		Symbols:       []string{},
		Neighbors:     []string{},
		DiscoveredBy:  []string{},
		HeuristicTags: []string{},
		Scores:        &FileScores{},
	}
}

// AddDiscoveredBy adds a discovery method if not already present.
func (fc *FileCard) AddDiscoveredBy(method string) {
	for _, m := range fc.DiscoveredBy {
		if m == method {
			return
		}
	}
	fc.DiscoveredBy = append(fc.DiscoveredBy, method)
}

// AddHeuristicTag adds a heuristic tag if not already present.
func (fc *FileCard) AddHeuristicTag(tag string) {
	for _, t := range fc.HeuristicTags {
		if t == tag {
			return
		}
	}
	fc.HeuristicTags = append(fc.HeuristicTags, tag)
}

// AddSymbol adds a symbol name if not already present.
func (fc *FileCard) AddSymbol(symbol string) {
	for _, s := range fc.Symbols {
		if s == symbol {
			return
		}
	}
	fc.Symbols = append(fc.Symbols, symbol)
}

// AddNeighbor adds a neighbor file path if not already present.
func (fc *FileCard) AddNeighbor(neighbor string) {
	for _, n := range fc.Neighbors {
		if n == neighbor {
			return
		}
	}
	fc.Neighbors = append(fc.Neighbors, neighbor)
}

// IsSeed returns true if the file was provided as a seed file.
func (fc *FileCard) IsSeed() bool {
	for _, m := range fc.DiscoveredBy {
		if m == "seed" {
			return true
		}
	}
	return false
}

// ToJSON returns the FileCard as formatted JSON.
func (fc *FileCard) ToJSON() ([]byte, error) {
	return json.MarshalIndent(fc, "", "  ")
}

// FileCardList is a sortable slice of FileCards.
type FileCardList []*FileCard

// Len implements sort.Interface.
func (l FileCardList) Len() int {
	return len(l)
}

// Less implements sort.Interface. Sorts by FinalScore descending.
func (l FileCardList) Less(i, j int) bool {
	return l[i].Scores.FinalScore > l[j].Scores.FinalScore
}

// Swap implements sort.Interface.
func (l FileCardList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// Paths returns a slice of all file paths in the list.
func (l FileCardList) Paths() []string {
	paths := make([]string, len(l))
	for i, fc := range l {
		paths[i] = fc.Path
	}
	return paths
}
