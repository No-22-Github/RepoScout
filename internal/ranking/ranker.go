// Package ranking provides ranking algorithms for file candidates.
package ranking

import (
	"sort"
	"strings"

	"github.com/no22/repo-scout/internal/schema"
)

// RankerConfig holds configuration for the ranking algorithm.
type RankerConfig struct {
	// DiscoveryWeight is the weight for how the file was discovered.
	// import=0.7, sibling=0.5, seed=1.0, same_dir/prefix=0.2, same_module=0.1
	// Range: 0.0 to 1.0. Default: 0.35
	DiscoveryWeight float64

	// SameModuleWeight is the weight bonus for files in the same module as seeds.
	// Range: 0.0 to 1.0. Default: 0.15
	SameModuleWeight float64

	// HeuristicWeight is the weight for heuristic score contribution.
	// Range: 0.0 to 1.0. Default: 0.20
	HeuristicWeight float64

	// ProfileWeight is the weight for profile score contribution.
	// Range: 0.0 to 1.0. Default: 0.10
	ProfileWeight float64

	// LLMWeight is the weight for LLM relevance when blending with structural score.
	// When LLM is available: final = structural*(1-LLMWeight) + llm*LLMWeight
	// Range: 0.0 to 1.0. Default: 0.65
	LLMWeight float64

	// MaxFinalScore is the maximum allowed final score.
	// Default: 1.0
	MaxFinalScore float64
}

// DefaultRankerConfig returns the default configuration for the ranker.
func DefaultRankerConfig() *RankerConfig {
	return &RankerConfig{
		DiscoveryWeight:  0.35,
		SameModuleWeight: 0.15,
		HeuristicWeight:  0.20,
		ProfileWeight:    0.10,
		LLMWeight:        0.65,
		MaxFinalScore:    1.0,
	}
}

// Ranker ranks file candidates based on multiple scoring factors.
type Ranker struct {
	config *RankerConfig
}

// NewRanker creates a new Ranker with the given configuration.
func NewRanker(config *RankerConfig) *Ranker {
	if config == nil {
		config = DefaultRankerConfig()
	}
	return &Ranker{config: config}
}

// RankInput contains the input for ranking.
type RankInput struct {
	// Cards is the list of FileCards to rank.
	Cards []*schema.FileCard

	// SeedModules is the set of modules that contain seed files.
	// Used to boost files in the same module as seeds.
	SeedModules map[string]bool
}

// RankResult contains the result of ranking.
type RankResult struct {
	// Cards is the sorted list of FileCards, highest score first.
	Cards []*schema.FileCard

	// TopFiles is a convenience slice of the top N files.
	TopFiles []string

	// ScoreBreakdown contains detailed score breakdown for each file.
	ScoreBreakdown map[string]*FileScoreBreakdown
}

// FileScoreBreakdown contains detailed score components for a file.
type FileScoreBreakdown struct {
	Path string `json:"path"`

	// Component scores (input)
	DiscoveryScore float64 `json:"discovery_score"`
	ModuleWeight   float64 `json:"module_weight"`
	HeuristicScore float64 `json:"heuristic_score"`
	ProfileScore   float64 `json:"profile_score"`
	LLMScore       float64 `json:"llm_score"`

	// Structural score (no LLM)
	StructuralScore float64 `json:"structural_score"`

	// Weighted contributions
	DiscoveryContribution float64 `json:"discovery_contribution"`
	ModuleContribution    float64 `json:"module_contribution"`
	HeuristicContribution float64 `json:"heuristic_contribution"`
	ProfileContribution   float64 `json:"profile_contribution"`
	LLMContribution       float64 `json:"llm_contribution"`

	// Final result
	FinalScore float64 `json:"final_score"`
	Rank       int     `json:"rank"`
}

// Rank sorts FileCards by computing and combining multiple score factors.
func (r *Ranker) Rank(input *RankInput) *RankResult {
	if input == nil || len(input.Cards) == 0 {
		return &RankResult{
			Cards:          []*schema.FileCard{},
			TopFiles:       []string{},
			ScoreBreakdown: make(map[string]*FileScoreBreakdown),
		}
	}

	// Ensure Scores struct exists for all cards
	for _, card := range input.Cards {
		if card.Scores == nil {
			card.Scores = &schema.FileScores{}
		}
	}

	// Phase 1: Compute module weights
	r.computeModuleWeights(input)

	// Phase 2: Compute final scores
	breakdown := r.computeFinalScores(input)

	// Phase 3: Sort by final score (descending)
	sortedCards := make([]*schema.FileCard, len(input.Cards))
	copy(sortedCards, input.Cards)

	sort.Slice(sortedCards, func(i, j int) bool {
		return sortedCards[i].Scores.FinalScore > sortedCards[j].Scores.FinalScore
	})

	// Assign ranks
	for i, card := range sortedCards {
		if bd, ok := breakdown[card.Path]; ok {
			bd.Rank = i + 1
		}
	}

	// Build top files list
	topFiles := make([]string, len(sortedCards))
	for i, card := range sortedCards {
		topFiles[i] = card.Path
	}

	return &RankResult{
		Cards:          sortedCards,
		TopFiles:       topFiles,
		ScoreBreakdown: breakdown,
	}
}

// computeModuleWeights calculates module weights based on proximity to seed modules.
func (r *Ranker) computeModuleWeights(input *RankInput) {
	seedModules := input.SeedModules
	if seedModules == nil {
		// Extract seed modules from cards
		seedModules = make(map[string]bool)
		for _, card := range input.Cards {
			if card.IsSeed() && card.Module != "" {
				seedModules[card.Module] = true
			}
		}
	}

	for _, card := range input.Cards {
		card.Scores.ModuleWeight = r.calculateModuleWeight(card.Module, seedModules)
	}
}

// calculateModuleWeight returns the module weight for a file based on its proximity to seed modules.
func (r *Ranker) calculateModuleWeight(module string, seedModules map[string]bool) float64 {
	if len(seedModules) == 0 {
		return 0.0
	}

	// Exact match with a seed module
	if seedModules[module] {
		return 1.0
	}

	// Check if module is a sub-module of any seed module
	for seedMod := range seedModules {
		if isSubModule(module, seedMod) {
			return 0.8 // Sub-module gets high weight
		}
		// Check if seed module is a sub-module of this module
		if isSubModule(seedMod, module) {
			return 0.6 // Parent module gets medium weight
		}
	}

	// Check for partial match (shared prefix)
	for seedMod := range seedModules {
		if sharedPrefixDepth(module, seedMod) > 0 {
			return 0.4 // Some relation
		}
	}

	return 0.0
}

// computeFinalScores calculates the final score for each file.
// When LLM data is available, it blends structural and LLM scores.
// Without LLM, it uses a weighted sum of structural signals.
func (r *Ranker) computeFinalScores(input *RankInput) map[string]*FileScoreBreakdown {
	breakdown := make(map[string]*FileScoreBreakdown)

	for _, card := range input.Cards {
		llmScore := calculateLLMScore(card)
		bd := &FileScoreBreakdown{
			Path:           card.Path,
			DiscoveryScore: card.Scores.DiscoveryScore,
			ModuleWeight:   card.Scores.ModuleWeight,
			HeuristicScore: card.Scores.HeuristicScore,
			ProfileScore:   card.Scores.ProfileScore,
			LLMScore:       llmScore,
		}

		// Structural score: weighted sum of non-LLM signals
		bd.DiscoveryContribution = bd.DiscoveryScore * r.config.DiscoveryWeight
		bd.ModuleContribution = bd.ModuleWeight * r.config.SameModuleWeight
		bd.HeuristicContribution = bd.HeuristicScore * r.config.HeuristicWeight
		bd.ProfileContribution = bd.ProfileScore * r.config.ProfileWeight
		bd.StructuralScore = StructuralScore(card.Scores, r.config)

		var finalScore float64
		if llmScore > 0 {
			// Two-phase blend: LLM dominates when available
			bd.LLMContribution = llmScore * r.config.LLMWeight
			finalScore = bd.StructuralScore*(1-r.config.LLMWeight) + bd.LLMContribution
		} else {
			finalScore = bd.StructuralScore
		}

		// Cap at max
		if finalScore > r.config.MaxFinalScore {
			finalScore = r.config.MaxFinalScore
		}

		card.Scores.FinalScore = finalScore
		bd.FinalScore = finalScore

		breakdown[card.Path] = bd
	}

	return breakdown
}

// StructuralScore returns the weighted non-LLM score for a file.
func StructuralScore(scores *schema.FileScores, config *RankerConfig) float64 {
	if scores == nil {
		return 0.0
	}
	if config == nil {
		config = DefaultRankerConfig()
	}
	return scores.DiscoveryScore*config.DiscoveryWeight +
		scores.ModuleWeight*config.SameModuleWeight +
		scores.HeuristicScore*config.HeuristicWeight +
		scores.ProfileScore*config.ProfileWeight
}

func calculateLLMScore(card *schema.FileCard) float64 {
	if card == nil || card.Scores == nil || card.Scores.LLMConfidence <= 0 {
		return 0.0
	}

	var labelScore float64
	switch card.Scores.LLMLabel {
	case "main_chain":
		labelScore = 1.0
	case "companion":
		labelScore = 0.7
	case "uncertain":
		labelScore = 0.4
	default:
		labelScore = 0.0
	}

	return labelScore * card.Scores.LLMConfidence
}

// isSubModule returns true if child is a sub-module of parent.
func isSubModule(child, parent string) bool {
	if parent == "" {
		return true
	}
	if child == parent {
		return true
	}
	if child == "" {
		return false
	}
	// Check if child starts with "parent/"
	if len(child) > len(parent) && child[:len(parent)+1] == parent+"/" {
		return true
	}
	return false
}

// sharedPrefixDepth returns the number of common path components at the start.
func sharedPrefixDepth(mod1, mod2 string) int {
	if mod1 == "" || mod2 == "" {
		return 0
	}

	// Split into parts
	parts1 := strings.Split(mod1, "/")
	parts2 := strings.Split(mod2, "/")

	// Find common prefix length
	minLen := len(parts1)
	if len(parts2) < minLen {
		minLen = len(parts2)
	}

	depth := 0
	for i := 0; i < minLen; i++ {
		if parts1[i] == parts2[i] {
			depth++
		} else {
			break
		}
	}

	return depth
}

// RankCards is a convenience function that ranks cards using default configuration.
func RankCards(cards []*schema.FileCard) *RankResult {
	return NewRanker(nil).Rank(&RankInput{Cards: cards})
}

// RankCardsWithSeedModules ranks cards with explicit seed modules.
func RankCardsWithSeedModules(cards []*schema.FileCard, seedModules map[string]bool) *RankResult {
	return NewRanker(nil).Rank(&RankInput{
		Cards:       cards,
		SeedModules: seedModules,
	})
}

// RankCardsWithConfig ranks cards using custom configuration.
func RankCardsWithConfig(cards []*schema.FileCard, config *RankerConfig) *RankResult {
	return NewRanker(config).Rank(&RankInput{Cards: cards})
}

// GetTopN returns the top N files from a rank result.
func (r *RankResult) GetTopN(n int) []string {
	if n <= 0 || len(r.TopFiles) == 0 {
		return []string{}
	}
	if n >= len(r.TopFiles) {
		return r.TopFiles
	}
	return r.TopFiles[:n]
}

// GetFilesAboveThreshold returns files with final score above the threshold.
func (r *RankResult) GetFilesAboveThreshold(threshold float64) []*schema.FileCard {
	result := make([]*schema.FileCard, 0)
	for _, card := range r.Cards {
		if card.Scores.FinalScore >= threshold {
			result = append(result, card)
		}
	}
	return result
}

// GetSeedModules extracts unique modules from seed files.
func GetSeedModules(cards []*schema.FileCard) map[string]bool {
	modules := make(map[string]bool)
	for _, card := range cards {
		if card.IsSeed() && card.Module != "" {
			modules[card.Module] = true
		}
	}
	return modules
}
