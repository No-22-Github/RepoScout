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

	// IncludeImport includes files connected via import/include relationships.
	// Default is true.
	IncludeImport bool

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
		IncludeImport:       true,
		PrefixMinLength:     3,
	}
}

// NeighborExpander expands a set of seed files to a candidate set.
type NeighborExpander struct {
	config          *ExpandConfig
	moduleDetector  *ModuleDetector
	moduleToFileMap map[string][]string // cached mapping from module to files
	importGraph     *ImportGraph        // optional import graph for import-based expansion
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

// WithImportGraph sets the import graph for import-based expansion.
func (ne *NeighborExpander) WithImportGraph(g *ImportGraph) *NeighborExpander {
	ne.importGraph = g
	return ne
}

// Expand builds the initial candidate set from seed files.
// It performs three types of expansion:
//  1. Same directory: files in the same directory as any seed file
//  2. Same module: files in the same module as any seed file
//  3. Prefix match: files with similar name prefixes to seed files
//
// All seed files are always included in the result.
func (ne *NeighborExpander) Expand(seedFiles, allFiles []string) []string {
	return ne.expandCore(seedFiles, allFiles, false).Candidates
}

// expandSameDir adds files in the same directory as the seed file.
func (ne *NeighborExpander) expandSameDir(seed string, allFiles []string, result *expansionAccumulator) {
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
			result.add(file, SourceSameDir)
		}
	}
}

// expandSameModule adds files in the same module as the seed file.
func (ne *NeighborExpander) expandSameModule(seed string, result *expansionAccumulator) {
	seedModule := ne.moduleDetector.Detect(seed)

	// Get all files in the same module
	if files, ok := ne.moduleToFileMap[seedModule]; ok {
		for _, file := range files {
			result.add(file, SourceSameModule)
		}
	}
}

// expandPrefixMatch adds files with similar name prefixes.
// For example, "settings_page.cc" would match "settings_handler.cc".
func (ne *NeighborExpander) expandPrefixMatch(seed string, allFiles []string, result *expansionAccumulator) {
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
				result.add(file, SourcePrefixMatch)
			}
		}
	}
}

func (ne *NeighborExpander) expandSiblingMatch(seed string, allFiles []string, result *expansionAccumulator) {
	for _, file := range allFiles {
		if ne.isSiblingMatch(seed, file) {
			result.add(file, SourceSiblingMatch)
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
	SourceImport       ExpansionSource = "import"
)

// ExpansionResult contains detailed information about how each file was discovered.
type ExpansionResult struct {
	// Candidates is the final candidate file set.
	Candidates []string

	// Sources maps each candidate file to its discovery sources.
	Sources map[string][]ExpansionSource
}

type expansionAccumulator struct {
	candidates   map[string]bool
	sources      map[string][]ExpansionSource
	trackSources bool
}

func (ne *NeighborExpander) newExpansionAccumulator(trackSources bool) *expansionAccumulator {
	sources := make(map[string][]ExpansionSource)
	if !trackSources {
		sources = nil
	}
	return &expansionAccumulator{
		candidates:   make(map[string]bool),
		sources:      sources,
		trackSources: trackSources,
	}
}

func (e *expansionAccumulator) addSeed(path string) {
	if path == "" {
		return
	}
	e.candidates[path] = true
	if e.trackSources {
		e.sources[path] = append(e.sources[path], SourceSeed)
	}
}

func (e *expansionAccumulator) add(path string, source ExpansionSource) {
	if path == "" || e.candidates[path] {
		return
	}
	e.candidates[path] = true
	if e.trackSources {
		e.sources[path] = append(e.sources[path], source)
	}
}

func (e *expansionAccumulator) finalize() *ExpansionResult {
	result := make([]string, 0, len(e.candidates))
	for path := range e.candidates {
		result = append(result, path)
	}
	sort.Strings(result)

	sources := e.sources
	if !e.trackSources {
		sources = make(map[string][]ExpansionSource)
	}

	return &ExpansionResult{
		Candidates: result,
		Sources:    sources,
	}
}

func (ne *NeighborExpander) expandCore(seedFiles, allFiles []string, trackSources bool) *ExpansionResult {
	if len(seedFiles) == 0 {
		return ne.newExpansionAccumulator(trackSources).finalize()
	}

	ne.buildModuleToFileMap(allFiles)
	acc := ne.newExpansionAccumulator(trackSources)

	for _, seed := range seedFiles {
		acc.addSeed(seed)
	}

	for _, seed := range seedFiles {
		if ne.config.IncludeSameDir {
			ne.expandSameDir(seed, allFiles, acc)
		}
		if ne.config.IncludeSameModule {
			ne.expandSameModule(seed, acc)
		}
		if ne.config.IncludePrefixMatch {
			ne.expandPrefixMatch(seed, allFiles, acc)
		}
		if ne.config.IncludeSiblingMatch {
			ne.expandSiblingMatch(seed, allFiles, acc)
		}
		if ne.config.IncludeImport && ne.importGraph != nil {
			for _, file := range ne.importGraph.Neighbors(seed) {
				acc.add(file, SourceImport)
			}
		}
	}

	return acc.finalize()
}

// ExpandWithSources expands seed files and tracks how each file was discovered.
func (ne *NeighborExpander) ExpandWithSources(seedFiles, allFiles []string) *ExpansionResult {
	return ne.expandCore(seedFiles, allFiles, true)
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
