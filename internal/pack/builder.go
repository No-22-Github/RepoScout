// Package pack provides ContextPack building functionality.
// It assembles the final ContextPack from ranked file candidates.
package pack

import (
	"strings"
	"time"

	"github.com/no22/repo-scout/internal/ranking"
	"github.com/no22/repo-scout/internal/schema"
)

// BuilderConfig holds configuration for the ContextPack builder.
type BuilderConfig struct {
	// MainChainThreshold is the minimum score for a file to be considered main chain.
	// Default: 0.5
	MainChainThreshold float64

	// CompanionThreshold is the minimum score for a file to be a companion.
	// Files below this but above UncertainThreshold go to uncertain.
	// Default: 0.3
	CompanionThreshold float64

	// UncertainThreshold is the minimum score to include at all.
	// Files below this are filtered out entirely.
	// Default: 0.1
	UncertainThreshold float64

	// MaxMainChain limits the number of files in main chain.
	// 0 means no limit. Default: 10
	MaxMainChain int

	// MaxCompanion limits the number of companion files.
	// 0 means no limit. Default: 20
	MaxCompanion int

	// MaxUncertain limits the number of uncertain files.
	// 0 means no limit. Default: 10
	MaxUncertain int

	// MaxTotalFiles limits the total number of emitted files across all buckets.
	// 0 means no limit.
	MaxTotalFiles int
}

// DefaultBuilderConfig returns the default configuration.
func DefaultBuilderConfig() *BuilderConfig {
	return &BuilderConfig{
		MainChainThreshold: 0.5,
		CompanionThreshold: 0.3,
		UncertainThreshold: 0.1,
		MaxMainChain:       10,
		MaxCompanion:       20,
		MaxUncertain:       10,
	}
}

// Builder assembles ContextPack from ranked file candidates.
type Builder struct {
	config *BuilderConfig
}

// NewBuilder creates a new Builder with the given configuration.
func NewBuilder(config *BuilderConfig) *Builder {
	if config == nil {
		config = DefaultBuilderConfig()
	}
	return &Builder{config: config}
}

// BuildInput contains the input for building a ContextPack.
type BuildInput struct {
	// Task is the original task description.
	Task string

	// RepoFamily identifies the technology stack.
	// If empty, will be inferred from file languages.
	RepoFamily string

	// RankResult is the output from the ranker.
	RankResult *ranking.RankResult

	// Request is the original ReconRequest (optional, for additional context).
	Request *schema.ReconRequest

	// ModelEnhanced indicates whether LLM-based reranking was applied.
	ModelEnhanced bool
}

// Build creates a ContextPack from ranked file candidates.
func (b *Builder) Build(input *BuildInput) *schema.ContextPack {
	startTime := time.Now()

	pack := schema.NewContextPack(input.Task)
	pack.RepoFamily = input.RepoFamily

	if input.RankResult == nil || len(input.RankResult.Cards) == 0 {
		pack.UpdateStats()
		return pack
	}

	// Classify files into main_chain, companion_files, uncertain_nodes.
	b.classifyFiles(pack, input.RankResult.Cards)

	// Apply total output limit before generating derived fields.
	b.trimToTotalLimit(pack)

	// Generate reading order
	b.generateReadingOrder(pack, input.Request)

	// Generate risk hints based on file analysis
	b.generateRiskHints(pack, input.RankResult.Cards)

	// Generate a Markdown summary embedded in the JSON payload.
	pack.SummaryMarkdown = b.generateSummaryMarkdown(pack)

	// Update stats
	pack.Stats.AnalysisTimeMs = time.Since(startTime).Milliseconds()
	pack.Stats.ModelEnhanced = input.ModelEnhanced
	pack.UpdateStats()

	return pack
}

// classifyFiles separates files into main_chain, companion_files, and uncertain_nodes.
func (b *Builder) classifyFiles(pack *schema.ContextPack, cards []*schema.FileCard) {
	var mainChain, companion, uncertain []*schema.FileCard

	for _, card := range cards {
		if card.Scores == nil {
			continue
		}

		score := card.Scores.FinalScore

		// Skip files below uncertain threshold
		if score < b.config.UncertainThreshold {
			continue
		}

		// Classify based on score thresholds
		if score >= b.config.MainChainThreshold {
			mainChain = append(mainChain, card)
		} else if score >= b.config.CompanionThreshold {
			companion = append(companion, card)
		} else {
			uncertain = append(uncertain, card)
		}
	}

	// Apply limits
	mainChain = b.limitCards(mainChain, b.config.MaxMainChain)
	companion = b.limitCards(companion, b.config.MaxCompanion)
	uncertain = b.limitCards(uncertain, b.config.MaxUncertain)

	// Add to pack (already sorted by score)
	for _, card := range mainChain {
		pack.AddMainChain(card.Path)
	}
	for _, card := range companion {
		pack.AddCompanion(card.Path)
	}
	for _, card := range uncertain {
		pack.AddUncertain(card.Path)
	}
}

// limitCards limits the number of cards to max.
func (b *Builder) limitCards(cards []*schema.FileCard, max int) []*schema.FileCard {
	if max <= 0 || len(cards) <= max {
		return cards
	}
	return cards[:max]
}

// generateReadingOrder creates a recommended reading sequence.
// The order prioritizes:
// 1. Seed files first (already known to be relevant)
// 2. Main chain files by score
// 3. Companion files by score
func (b *Builder) generateReadingOrder(pack *schema.ContextPack, req *schema.ReconRequest) {
	var order []string
	seen := make(map[string]bool)

	// Helper to add if not seen
	addToOrder := func(path string) {
		if !seen[path] {
			order = append(order, path)
			seen[path] = true
		}
	}

	isSeed := make(map[string]bool)
	if req != nil {
		for _, path := range req.SeedFiles {
			isSeed[path] = true
		}
	}

	addSeedFirst := func(paths []string) {
		for _, path := range paths {
			if isSeed[path] {
				addToOrder(path)
			}
		}
		for _, path := range paths {
			if !isSeed[path] {
				addToOrder(path)
			}
		}
	}

	// First: seed files from the main chain, then the rest of the main chain.
	addSeedFirst(pack.MainChain)

	// Then: seed files from companions, then the rest.
	addSeedFirst(pack.CompanionFiles)

	// Finally: uncertain nodes.
	addSeedFirst(pack.UncertainNodes)

	pack.SetReadingOrder(order)
}

// trimToTotalLimit applies the total output limit while preserving priority
// order: main_chain > companion_files > uncertain_nodes.
func (b *Builder) trimToTotalLimit(pack *schema.ContextPack) {
	maxTotal := b.config.MaxTotalFiles
	if maxTotal <= 0 {
		return
	}

	remaining := maxTotal
	trim := func(paths []string) []string {
		if remaining <= 0 {
			return []string{}
		}
		if len(paths) <= remaining {
			remaining -= len(paths)
			return paths
		}
		result := paths[:remaining]
		remaining = 0
		return result
	}

	pack.MainChain = trim(pack.MainChain)
	pack.CompanionFiles = trim(pack.CompanionFiles)
	pack.UncertainNodes = trim(pack.UncertainNodes)
}

func (b *Builder) generateSummaryMarkdown(pack *schema.ContextPack) string {
	var sb strings.Builder

	sb.WriteString("# RepoScout Summary\n\n")
	sb.WriteString("## Task\n\n")
	sb.WriteString(pack.Task)
	sb.WriteString("\n\n")

	if len(pack.ReadingOrder) > 0 {
		sb.WriteString("## Recommended First Reads\n\n")
		limit := len(pack.ReadingOrder)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			sb.WriteString(strings.Join([]string{"- `", pack.ReadingOrder[i], "`\n"}, ""))
		}
		sb.WriteString("\n")
	}

	if len(pack.RiskHints) > 0 {
		sb.WriteString("## Risk Hints\n\n")
		for _, hint := range pack.RiskHints {
			sb.WriteString("- ")
			sb.WriteString(hint.Message)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// generateRiskHints creates risk hints based on file analysis.
func (b *Builder) generateRiskHints(pack *schema.ContextPack, cards []*schema.FileCard) {
	// Check for test coverage
	b.checkTestCoverage(pack, cards)

	// Check for complexity indicators
	b.checkComplexity(pack, cards)

	// Check for configuration concerns
	b.checkConfiguration(pack, cards)
}

// checkTestCoverage adds risk hints about test coverage.
func (b *Builder) checkTestCoverage(pack *schema.ContextPack, cards []*schema.FileCard) {
	var nonTestFiles []string
	hasTestFiles := false

	for _, card := range cards {
		if isTestFile(card) {
			hasTestFiles = true
		} else if card.Scores != nil && card.Scores.FinalScore >= b.config.CompanionThreshold {
			nonTestFiles = append(nonTestFiles, card.Path)
		}
	}

	// If we have significant main/companion files but no tests, warn
	if len(nonTestFiles) > 3 && !hasTestFiles {
		pack.AddRiskHint(
			"warning",
			"test-coverage",
			"No test files found in the candidate set. Consider verifying the changes have adequate test coverage.",
		)
	}
}

// checkComplexity adds risk hints about complexity.
func (b *Builder) checkComplexity(pack *schema.ContextPack, cards []*schema.FileCard) {
	for _, card := range cards {
		// Files with many symbols might be complex
		if len(card.Symbols) > 20 {
			pack.AddRiskHint(
				"info",
				"complexity",
				"File has many symbols, may require careful review.",
				card.Path,
			)
		}
	}
}

// checkConfiguration adds risk hints about configuration.
func (b *Builder) checkConfiguration(pack *schema.ContextPack, cards []*schema.FileCard) {
	configFiles := []string{}
	for _, card := range cards {
		if hasConfigTag(card) {
			configFiles = append(configFiles, card.Path)
		}
	}

	if len(configFiles) > 0 {
		pack.AddRiskHint(
			"info",
			"configuration",
			"Configuration files detected in candidate set. Verify changes don't break existing configurations.",
			configFiles...,
		)
	}
}

// isTestFile checks if a file is a test file.
func isTestFile(card *schema.FileCard) bool {
	// Check heuristic tags
	for _, tag := range card.HeuristicTags {
		if tag == "tests" || tag == "test" {
			return true
		}
	}

	// Check path patterns
	path := strings.ToLower(card.Path)
	if strings.HasSuffix(path, "_test.go") ||
		strings.HasSuffix(path, ".test.ts") ||
		strings.HasSuffix(path, ".test.tsx") ||
		strings.HasSuffix(path, ".spec.ts") ||
		strings.HasSuffix(path, ".spec.tsx") ||
		strings.HasSuffix(path, "_test.py") ||
		strings.Contains(path, "/test_") ||
		strings.Contains(path, "/test/") ||
		strings.Contains(path, "/tests/") ||
		strings.Contains(path, "/__tests__/") {
		return true
	}

	// Check for test_ prefix in filename (common Python pattern)
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		filename := path[idx+1:]
		if strings.HasPrefix(filename, "test_") && strings.HasSuffix(filename, ".py") {
			return true
		}
	} else if strings.HasPrefix(path, "test_") && strings.HasSuffix(path, ".py") {
		// File in root directory with test_ prefix
		return true
	}

	return false
}

// hasConfigTag checks if a file has configuration-related tags.
func hasConfigTag(card *schema.FileCard) bool {
	for _, tag := range card.HeuristicTags {
		if tag == "default_config" || tag == "config" || tag == "configuration" {
			return true
		}
	}
	return false
}

// BuildFromRankResult is a convenience function that builds a ContextPack
// from a rank result with default configuration.
func BuildFromRankResult(task string, result *ranking.RankResult) *schema.ContextPack {
	return NewBuilder(nil).Build(&BuildInput{
		Task:       task,
		RankResult: result,
	})
}

// BuildFromCards is a convenience function that builds a ContextPack
// from a slice of FileCards using default ranking and building.
func BuildFromCards(task string, cards []*schema.FileCard) *schema.ContextPack {
	// Rank the cards first
	result := ranking.RankCards(cards)

	// Then build the pack
	return BuildFromRankResult(task, result)
}
