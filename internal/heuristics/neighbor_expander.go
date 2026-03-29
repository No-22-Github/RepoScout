// Package heuristics provides heuristic rules for file analysis.
package heuristics

import (
	"path/filepath"
	"sort"
	"strings"
)

// ExpandConfig holds configuration for neighbor expansion.
type ExpandConfig struct {
	// ModuleConfig is the configuration for module detection.
	// If nil, default configuration is used.
	ModuleConfig *ModuleConfig

	// IncludeSameDir includes files in the same directory as seed files.
	// Default is true.
	IncludeSameDir bool

	// IncludeSameModule includes files in the same module as seed files.
	// Default is true.
	IncludeSameModule bool

	// IncludePrefixMatch includes files with matching name prefixes.
	// Default is true.
	IncludePrefixMatch bool

	// IncludeSiblingMatch includes likely companion files such as implementation/test
	// pairs and source/header pairs discovered by normalized basename matching.
	// Default is true.
	IncludeSiblingMatch bool

	// PrefixMinLength is the minimum length for prefix matching.
	// Files with names shorter than this are not used for prefix matching.
	// Default is 3.
	PrefixMinLength int
}

// DefaultExpandConfig returns the default configuration for neighbor expansion.
func DefaultExpandConfig() *ExpandConfig {
	return &ExpandConfig{
		ModuleConfig:        nil,
		IncludeSameDir:      true,
		IncludeSameModule:   true,
		IncludePrefixMatch:  true,
		IncludeSiblingMatch: true,
		PrefixMinLength:     3,
	}
}

// NeighborExpander expands a set of seed files to a candidate set.
type NeighborExpander struct {
	config          *ExpandConfig
	moduleDetector  *ModuleDetector
	moduleToFileMap map[string][]string // cached mapping from module to files
}

// NewNeighborExpander creates a new NeighborExpander with the given configuration.
func NewNeighborExpander(config *ExpandConfig) *NeighborExpander {
	if config == nil {
		config = DefaultExpandConfig()
	}
	return &NeighborExpander{
		config:         config,
		moduleDetector: NewModuleDetector(config.ModuleConfig),
	}
}

// Expand builds the initial candidate set from seed files.
// It performs three types of expansion:
//  1. Same directory: files in the same directory as any seed file
//  2. Same module: files in the same module as any seed file
//  3. Prefix match: files with similar name prefixes to seed files
//
// All seed files are always included in the result.
func (ne *NeighborExpander) Expand(seedFiles, allFiles []string) []string {
	if len(seedFiles) == 0 {
		return []string{}
	}

	// Build module to files mapping for efficient lookup
	ne.buildModuleToFileMap(allFiles)

	candidates := make(map[string]bool)

	// Always include all seed files
	for _, seed := range seedFiles {
		candidates[seed] = true
	}

	// Build a quick lookup for seed files
	seedSet := make(map[string]bool)
	for _, seed := range seedFiles {
		seedSet[seed] = true
	}

	// Expand for each seed file
	for _, seed := range seedFiles {
		// 1. Same directory expansion
		if ne.config.IncludeSameDir {
			ne.expandSameDir(seed, allFiles, candidates)
		}

		// 2. Same module expansion
		if ne.config.IncludeSameModule {
			ne.expandSameModule(seed, candidates)
		}

		// 3. Prefix match expansion
		if ne.config.IncludePrefixMatch {
			ne.expandPrefixMatch(seed, allFiles, candidates)
		}

		// 4. Companion sibling expansion
		if ne.config.IncludeSiblingMatch {
			ne.expandSiblingMatch(seed, allFiles, candidates)
		}
	}

	// Convert to sorted slice for stable output
	result := make([]string, 0, len(candidates))
	for path := range candidates {
		result = append(result, path)
	}
	sort.Strings(result)

	return result
}

// expandSameDir adds files in the same directory as the seed file.
func (ne *NeighborExpander) expandSameDir(seed string, allFiles []string, candidates map[string]bool) {
	seedDir := filepath.Dir(seed)
	if seedDir == "." {
		seedDir = ""
	}

	for _, file := range allFiles {
		fileDir := filepath.Dir(file)
		if fileDir == "." {
			fileDir = ""
		}
		if fileDir == seedDir {
			candidates[file] = true
		}
	}
}

// expandSameModule adds files in the same module as the seed file.
func (ne *NeighborExpander) expandSameModule(seed string, candidates map[string]bool) {
	seedModule := ne.moduleDetector.Detect(seed)

	// Get all files in the same module
	if files, ok := ne.moduleToFileMap[seedModule]; ok {
		for _, file := range files {
			candidates[file] = true
		}
	}
}

// expandPrefixMatch adds files with similar name prefixes.
// For example, "settings_page.cc" would match "settings_handler.cc".
func (ne *NeighborExpander) expandPrefixMatch(seed string, allFiles []string, candidates map[string]bool) {
	seedName := filepath.Base(seed)
	seedExt := filepath.Ext(seedName)
	seedBase := strings.TrimSuffix(seedName, seedExt)

	minLen := ne.config.PrefixMinLength
	if minLen <= 0 {
		minLen = 3
	}

	// Skip short names
	if len(seedBase) < minLen {
		return
	}

	// Use at most half the name length as prefix for matching
	prefixLen := len(seedBase) / 2
	if prefixLen < minLen {
		prefixLen = minLen
	}
	if prefixLen > len(seedBase) {
		prefixLen = len(seedBase)
	}

	prefix := strings.ToLower(seedBase[:prefixLen])

	for _, file := range allFiles {
		fileName := filepath.Base(file)
		fileExt := filepath.Ext(fileName)
		fileBase := strings.TrimSuffix(fileName, fileExt)

		// Skip short names
		if len(fileBase) < minLen {
			continue
		}

		// Check if prefixes match
		if len(fileBase) >= prefixLen {
			filePrefix := strings.ToLower(fileBase[:prefixLen])
			if filePrefix == prefix {
				candidates[file] = true
			}
		}
	}
}

func (ne *NeighborExpander) expandSiblingMatch(seed string, allFiles []string, candidates map[string]bool) {
	for _, file := range allFiles {
		if ne.isSiblingMatch(seed, file) {
			candidates[file] = true
		}
	}
}

// buildModuleToFileMap creates a mapping from module names to file paths.
func (ne *NeighborExpander) buildModuleToFileMap(allFiles []string) {
	ne.moduleToFileMap = make(map[string][]string)
	for _, file := range allFiles {
		module := ne.moduleDetector.Detect(file)
		ne.moduleToFileMap[module] = append(ne.moduleToFileMap[module], file)
	}
}

// ExpandNeighbors is a convenience function that expands seed files using default configuration.
func ExpandNeighbors(seedFiles, allFiles []string) []string {
	return NewNeighborExpander(nil).Expand(seedFiles, allFiles)
}

// ExpandNeighborsWithConfig expands seed files using custom configuration.
func ExpandNeighborsWithConfig(seedFiles, allFiles []string, config *ExpandConfig) []string {
	return NewNeighborExpander(config).Expand(seedFiles, allFiles)
}

// ExpansionSource describes how a file was added to the candidate set.
type ExpansionSource string

const (
	SourceSeed         ExpansionSource = "seed"
	SourceSameDir      ExpansionSource = "same_dir"
	SourceSameModule   ExpansionSource = "same_module"
	SourcePrefixMatch  ExpansionSource = "prefix_match"
	SourceSiblingMatch ExpansionSource = "sibling_match"
)

// ExpansionResult contains detailed information about how each file was discovered.
type ExpansionResult struct {
	// Candidates is the final candidate file set.
	Candidates []string

	// Sources maps each candidate file to its discovery sources.
	Sources map[string][]ExpansionSource
}

// ExpandWithSources expands seed files and tracks how each file was discovered.
func (ne *NeighborExpander) ExpandWithSources(seedFiles, allFiles []string) *ExpansionResult {
	if len(seedFiles) == 0 {
		return &ExpansionResult{
			Candidates: []string{},
			Sources:    make(map[string][]ExpansionSource),
		}
	}

	// Build module to files mapping
	ne.buildModuleToFileMap(allFiles)

	sources := make(map[string][]ExpansionSource)
	candidates := make(map[string]bool)

	// Mark all seed files
	for _, seed := range seedFiles {
		candidates[seed] = true
		sources[seed] = append(sources[seed], SourceSeed)
	}

	// Expand for each seed file
	for _, seed := range seedFiles {
		// 1. Same directory expansion
		if ne.config.IncludeSameDir {
			seedDir := filepath.Dir(seed)
			if seedDir == "." {
				seedDir = ""
			}
			for _, file := range allFiles {
				fileDir := filepath.Dir(file)
				if fileDir == "." {
					fileDir = ""
				}
				if fileDir == seedDir && !candidates[file] {
					candidates[file] = true
					sources[file] = append(sources[file], SourceSameDir)
				}
			}
		}

		// 2. Same module expansion
		if ne.config.IncludeSameModule {
			seedModule := ne.moduleDetector.Detect(seed)
			if files, ok := ne.moduleToFileMap[seedModule]; ok {
				for _, file := range files {
					if !candidates[file] {
						candidates[file] = true
						sources[file] = append(sources[file], SourceSameModule)
					}
				}
			}
		}

		// 3. Prefix match expansion
		if ne.config.IncludePrefixMatch {
			seedName := filepath.Base(seed)
			seedExt := filepath.Ext(seedName)
			seedBase := strings.TrimSuffix(seedName, seedExt)

			minLen := ne.config.PrefixMinLength
			if minLen <= 0 {
				minLen = 3
			}

			if len(seedBase) >= minLen {
				prefixLen := len(seedBase) / 2
				if prefixLen < minLen {
					prefixLen = minLen
				}
				if prefixLen > len(seedBase) {
					prefixLen = len(seedBase)
				}

				prefix := strings.ToLower(seedBase[:prefixLen])

				for _, file := range allFiles {
					fileName := filepath.Base(file)
					fileExt := filepath.Ext(fileName)
					fileBase := strings.TrimSuffix(fileName, fileExt)

					if len(fileBase) >= prefixLen {
						filePrefix := strings.ToLower(fileBase[:prefixLen])
						if filePrefix == prefix && !candidates[file] {
							candidates[file] = true
							sources[file] = append(sources[file], SourcePrefixMatch)
						}
					}
				}
			}
		}

		// 4. Companion sibling expansion
		if ne.config.IncludeSiblingMatch {
			for _, file := range allFiles {
				if ne.isSiblingMatch(seed, file) && !candidates[file] {
					candidates[file] = true
					sources[file] = append(sources[file], SourceSiblingMatch)
				}
			}
		}
	}

	// Convert to sorted slice
	result := make([]string, 0, len(candidates))
	for path := range candidates {
		result = append(result, path)
	}
	sort.Strings(result)

	return &ExpansionResult{
		Candidates: result,
		Sources:    sources,
	}
}

func (ne *NeighborExpander) isSiblingMatch(seed, file string) bool {
	if seed == "" || file == "" || seed == file {
		return false
	}

	seedBase := strings.TrimSuffix(filepath.Base(seed), filepath.Ext(seed))
	fileBase := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	if seedBase == "" || fileBase == "" {
		return false
	}

	seedStem := normalizedCompanionStem(seedBase)
	fileStem := normalizedCompanionStem(fileBase)
	if seedStem == "" || seedStem != fileStem {
		return false
	}

	seedDir := filepath.Dir(seed)
	fileDir := filepath.Dir(file)
	sameDir := seedDir == fileDir
	sameModule := ne.moduleDetector.Detect(seed) == ne.moduleDetector.Detect(file)

	seedExt := strings.ToLower(filepath.Ext(seed))
	fileExt := strings.ToLower(filepath.Ext(file))

	if isTestCompanionPair(seedBase, fileBase) {
		return sameDir || sameModule
	}

	if isSourceHeaderPair(seedExt, fileExt) {
		return sameDir || sameModule
	}

	return false
}

func normalizedCompanionStem(base string) string {
	lower := strings.ToLower(base)
	suffixes := []string{
		"_test", "_tests", ".test", ".tests",
		"_spec", "_specs", ".spec", ".specs",
		"_mock", ".mock", "_fixture", ".fixture",
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSuffix(lower, suffix)
		}
	}
	return lower
}

func isTestCompanionPair(a, b string) bool {
	aNorm := strings.ToLower(a)
	bNorm := strings.ToLower(b)
	return aNorm != bNorm && normalizedCompanionStem(aNorm) == normalizedCompanionStem(bNorm)
}

func isSourceHeaderPair(aExt, bExt string) bool {
	pairs := map[string]map[string]bool{
		".c":   {".h": true},
		".cc":  {".h": true, ".hh": true, ".hpp": true},
		".cpp": {".h": true, ".hh": true, ".hpp": true},
		".h":   {".c": true, ".cc": true, ".cpp": true},
		".hh":  {".cc": true, ".cpp": true},
		".hpp": {".cc": true, ".cpp": true},
	}
	if targets, ok := pairs[aExt]; ok && targets[bExt] {
		return true
	}
	if targets, ok := pairs[bExt]; ok && targets[aExt] {
		return true
	}
	return false
}
