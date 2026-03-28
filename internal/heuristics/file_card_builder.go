// Package heuristics provides heuristic rules for file analysis.
package heuristics

import (
	"path/filepath"
	"strings"

	"github.com/no22/repo-scout/internal/scanner"
	"github.com/no22/repo-scout/internal/schema"
)

// FileCardBuilderConfig holds configuration for the FileCard builder.
type FileCardBuilderConfig struct {
	// ModuleConfig is the configuration for module detection.
	ModuleConfig *ModuleConfig

	// BasicRulesConfig is the configuration for basic heuristic rules.
	BasicRulesConfig *BasicRulesConfig

	// BrowserSettingsConfig is the configuration for browser settings profile rules.
	BrowserSettingsConfig *BrowserSettingsProfileConfig

	// SymbolExtractorConfig configures the symbol extractor.
	// If nil, a default SymbolExtractor is used.
	MaxFileSize int64
	MaxSymbols  int
}

// DefaultFileCardBuilderConfig returns the default configuration.
func DefaultFileCardBuilderConfig() *FileCardBuilderConfig {
	return &FileCardBuilderConfig{
		ModuleConfig:          nil,
		BasicRulesConfig:      nil,
		BrowserSettingsConfig: nil,
		MaxFileSize:           500 * 1024, // 500KB
		MaxSymbols:            100,
	}
}

// FileCardBuilder builds FileCard instances from candidate files.
type FileCardBuilder struct {
	config            *FileCardBuilderConfig
	moduleDetector    *ModuleDetector
	basicRuleEngine   *BasicRuleEngine
	profileRuleEngine *BrowserSettingsProfileRuleEngine
	symbolExtractor   *scanner.SymbolExtractor
}

// NewFileCardBuilder creates a new FileCardBuilder with the given configuration.
func NewFileCardBuilder(config *FileCardBuilderConfig) *FileCardBuilder {
	if config == nil {
		config = DefaultFileCardBuilderConfig()
	}

	extractor := scanner.NewSymbolExtractor()
	if config.MaxFileSize > 0 {
		extractor.WithMaxFileSize(config.MaxFileSize)
	}
	if config.MaxSymbols > 0 {
		extractor.WithMaxSymbols(config.MaxSymbols)
	}

	return &FileCardBuilder{
		config:            config,
		moduleDetector:    NewModuleDetector(config.ModuleConfig),
		basicRuleEngine:   NewBasicRuleEngine(config.BasicRulesConfig),
		profileRuleEngine: NewBrowserSettingsProfileRuleEngine(config.BrowserSettingsConfig),
		symbolExtractor:   extractor,
	}
}

// BuildOptions contains options for building FileCards.
type BuildOptions struct {
	// RepoRoot is the root directory of the repository.
	RepoRoot string

	// Profile is the profile to use for profile-specific rules.
	// If empty, only basic rules are applied.
	Profile string

	// FocusChecks are the focus checks to apply for basic rules.
	// If empty, all basic rules are applied.
	FocusChecks []string

	// SeedFiles are the seed files that should be marked as seeds.
	SeedFiles []string

	// DiscoverySources maps file paths to their discovery sources.
	// This is populated by neighbor expansion.
	DiscoverySources map[string][]ExpansionSource
}

// Build creates a FileCard for a single file.
func (b *FileCardBuilder) Build(filePath string, opts *BuildOptions) *schema.FileCard {
	card := schema.NewFileCard(filePath)

	// 1. Set language
	card.Lang = LangDetect(filePath)

	// 2. Set module
	card.Module = b.moduleDetector.Detect(filePath)

	// 3. Set discovery sources
	b.setDiscoverySources(card, filePath, opts)

	// 4. Apply basic heuristic rules
	basicResult := b.basicRuleEngine.ApplyRules(filePath, opts.FocusChecks)
	b.applyRuleResult(card, basicResult)

	// 5. Apply profile-specific rules if applicable
	if opts.Profile != "" && MatchesProfile(opts.Profile) {
		profileResult := b.profileRuleEngine.ApplyRules(filePath)
		b.applyRuleResult(card, profileResult)
		// Store profile score separately
		card.Scores.ProfileScore = profileResult.Score
	}

	// 6. Extract symbols if it's a source file
	if IsSourceFile(filePath) && opts.RepoRoot != "" {
		absPath := filepath.Join(opts.RepoRoot, filePath)
		symbols := b.symbolExtractor.ExtractFromFile(absPath, card.Lang)
		for _, s := range symbols {
			card.AddSymbol(s.Name)
		}
	}

	// 7. Calculate initial heuristic score
	card.Scores.HeuristicScore = basicResult.Score
	if card.Scores.ProfileScore > 0 {
		// Combine basic and profile scores
		combinedScore := card.Scores.HeuristicScore + card.Scores.ProfileScore*0.5
		if combinedScore > 1.0 {
			combinedScore = 1.0
		}
		card.Scores.HeuristicScore = combinedScore
	}

	// 8. Set seed weight for seed files
	if b.isSeedFile(filePath, opts) {
		card.Scores.SeedWeight = 1.0
	}

	return card
}

// BuildAll creates FileCards for all candidate files.
func (b *FileCardBuilder) BuildAll(files []string, opts *BuildOptions) []*schema.FileCard {
	cards := make([]*schema.FileCard, 0, len(files))
	for _, file := range files {
		card := b.Build(file, opts)
		cards = append(cards, card)
	}
	return cards
}

// setDiscoverySources sets the discovery sources for a file.
func (b *FileCardBuilder) setDiscoverySources(card *schema.FileCard, filePath string, opts *BuildOptions) {
	// First check if it's a seed file
	if b.isSeedFile(filePath, opts) {
		card.AddDiscoveredBy("seed")
	}

	// Add discovery sources from neighbor expansion
	if opts.DiscoverySources != nil {
		if sources, ok := opts.DiscoverySources[filePath]; ok {
			for _, source := range sources {
				card.AddDiscoveredBy(string(source))
			}
		}
	}
}

// isSeedFile checks if a file is in the seed files list.
func (b *FileCardBuilder) isSeedFile(filePath string, opts *BuildOptions) bool {
	if opts == nil || opts.SeedFiles == nil {
		return false
	}

	// Normalize path for comparison
	normalizedPath := filepath.ToSlash(filePath)
	for _, seed := range opts.SeedFiles {
		normalizedSeed := filepath.ToSlash(seed)
		if normalizedPath == normalizedSeed {
			return true
		}
	}
	return false
}

// applyRuleResult applies a rule result to a FileCard.
func (b *FileCardBuilder) applyRuleResult(card *schema.FileCard, result *RuleResult) {
	// Add tags
	for _, tag := range result.Tags {
		card.AddHeuristicTag(tag)
	}

	// Add discovery sources from rules
	for _, discoveredBy := range result.DiscoveredBy {
		card.AddDiscoveredBy(discoveredBy)
	}
}

// FileCardBuilderInput contains all the inputs needed to build FileCards.
type FileCardBuilderInput struct {
	// Candidates is the list of candidate file paths.
	Candidates []string

	// RepoRoot is the root directory of the repository.
	RepoRoot string

	// Profile is the profile to use for profile-specific rules.
	Profile string

	// FocusChecks are the focus checks to apply.
	FocusChecks []string

	// SeedFiles are the seed files.
	SeedFiles []string

	// DiscoverySources maps file paths to their discovery sources.
	DiscoverySources map[string][]ExpansionSource
}

// BuildFileCards is a convenience function that builds FileCards using default configuration.
func BuildFileCards(input *FileCardBuilderInput) []*schema.FileCard {
	return NewFileCardBuilder(nil).BuildAll(input.Candidates, &BuildOptions{
		RepoRoot:         input.RepoRoot,
		Profile:          input.Profile,
		FocusChecks:      input.FocusChecks,
		SeedFiles:        input.SeedFiles,
		DiscoverySources: input.DiscoverySources,
	})
}

// BuildFileCardsWithConfig builds FileCards using custom configuration.
func BuildFileCardsWithConfig(input *FileCardBuilderInput, config *FileCardBuilderConfig) []*schema.FileCard {
	return NewFileCardBuilder(config).BuildAll(input.Candidates, &BuildOptions{
		RepoRoot:         input.RepoRoot,
		Profile:          input.Profile,
		FocusChecks:      input.FocusChecks,
		SeedFiles:        input.SeedFiles,
		DiscoverySources: input.DiscoverySources,
	})
}

// NormalizePath normalizes a file path for consistent comparison.
func NormalizePath(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(path), "\\", "/")
}
