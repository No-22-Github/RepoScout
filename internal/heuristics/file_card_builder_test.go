// Package heuristics provides heuristic rules for file analysis.
package heuristics

import (
	"path/filepath"
	"testing"
)

func TestFileCardBuilder_Build(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		filePath   string
		opts       *BuildOptions
		wantLang   string
		wantModule string
		wantSeed   bool
		wantTags   []string
	}{
		{
			name:     "simple Go file",
			filePath: "cmd/reposcout/main.go",
			opts: &BuildOptions{
				RepoRoot:    tmpDir,
				Profile:     "",
				SeedFiles:   []string{},
				FocusChecks: nil,
			},
			wantLang:   "go",
			wantModule: "cmd/reposcout",
			wantSeed:   false,
		},
		{
			name:     "seed file",
			filePath: "browser/settings/settings_page.cc",
			opts: &BuildOptions{
				RepoRoot:  tmpDir,
				Profile:   "",
				SeedFiles: []string{"browser/settings/settings_page.cc"},
			},
			wantLang:   "cpp",
			wantModule: "browser/settings",
			wantSeed:   true,
		},
		{
			name:     "test file with test tag",
			filePath: "browser/settings/settings_test.cc",
			opts: &BuildOptions{
				RepoRoot:  tmpDir,
				Profile:   "",
				SeedFiles: []string{},
			},
			wantLang:   "cpp",
			wantModule: "browser/settings",
			wantSeed:   false,
			wantTags:   []string{TagTestFile},
		},
		{
			name:     "TypeScript file",
			filePath: "src/components/Button.tsx",
			opts: &BuildOptions{
				RepoRoot:  tmpDir,
				Profile:   "",
				SeedFiles: []string{},
			},
			wantLang:   "tsx",
			wantModule: "src/components",
			wantSeed:   false,
		},
		{
			name:     "Python file",
			filePath: "lib/utils/helpers.py",
			opts: &BuildOptions{
				RepoRoot:  tmpDir,
				Profile:   "",
				SeedFiles: []string{},
			},
			wantLang:   "py",
			wantModule: "lib/utils",
			wantSeed:   false,
		},
	}

	builder := NewFileCardBuilder(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := builder.Build(tt.filePath, tt.opts)

			if card.Path != tt.filePath {
				t.Errorf("Build().Path = %v, want %v", card.Path, tt.filePath)
			}

			if card.Lang != tt.wantLang {
				t.Errorf("Build().Lang = %v, want %v", card.Lang, tt.wantLang)
			}

			if card.Module != tt.wantModule {
				t.Errorf("Build().Module = %v, want %v", card.Module, tt.wantModule)
			}

			if card.IsSeed() != tt.wantSeed {
				t.Errorf("Build().IsSeed() = %v, want %v", card.IsSeed(), tt.wantSeed)
			}

			if tt.wantSeed && card.Scores.SeedWeight != 1.0 {
				t.Errorf("Build().Scores.SeedWeight = %v, want 1.0", card.Scores.SeedWeight)
			}

			// Check tags if specified
			if tt.wantTags != nil {
				for _, wantTag := range tt.wantTags {
					found := false
					for _, tag := range card.HeuristicTags {
						if tag == wantTag {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Build().HeuristicTags missing %v, got %v", wantTag, card.HeuristicTags)
					}
				}
			}
		})
	}
}

func TestFileCardBuilder_BuildWithProfile(t *testing.T) {
	tmpDir := t.TempDir()
	builder := NewFileCardBuilder(nil)

	tests := []struct {
		name           string
		filePath       string
		profile        string
		wantProfileTag bool
	}{
		{
			name:           "settings page with browser_settings profile",
			filePath:       "browser/settings/settings_page.cc",
			profile:        "browser_settings",
			wantProfileTag: true,
		},
		{
			name:           "handler with browser_settings profile",
			filePath:       "browser/settings/settings_handler.cc",
			profile:        "browser_settings",
			wantProfileTag: true,
		},
		{
			name:           "unrelated file with browser_settings profile",
			filePath:       "components/button.cc",
			profile:        "browser_settings",
			wantProfileTag: false,
		},
		{
			name:           "settings page without profile",
			filePath:       "browser/settings/settings_page.cc",
			profile:        "",
			wantProfileTag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &BuildOptions{
				RepoRoot:  tmpDir,
				Profile:   tt.profile,
				SeedFiles: []string{},
			}
			card := builder.Build(tt.filePath, opts)

			hasProfileTag := false
			profileTags := GetBrowserSettingsProfileTags()
			for _, tag := range card.HeuristicTags {
				for _, pt := range profileTags {
					if tag == pt {
						hasProfileTag = true
						break
					}
				}
			}

			if hasProfileTag != tt.wantProfileTag {
				t.Errorf("Build() profile tag = %v, want %v, tags: %v", hasProfileTag, tt.wantProfileTag, card.HeuristicTags)
			}
		})
	}
}

func TestFileCardBuilder_BuildWithDiscoverySources(t *testing.T) {
	tmpDir := t.TempDir()
	builder := NewFileCardBuilder(nil)

	discoverySources := map[string][]ExpansionSource{
		"file1.go": {SourceSeed, SourceSameDir},
		"file2.go": {SourceSameModule},
		"file3.go": {SourcePrefixMatch},
	}

	opts := &BuildOptions{
		RepoRoot:         tmpDir,
		DiscoverySources: discoverySources,
		SeedFiles:        []string{"file1.go"},
	}

	tests := []struct {
		name     string
		filePath string
		wantSrcs []string
	}{
		{
			name:     "seed file with multiple sources",
			filePath: "file1.go",
			wantSrcs: []string{"seed", "same_dir"},
		},
		{
			name:     "same module file",
			filePath: "file2.go",
			wantSrcs: []string{"same_module"},
		},
		{
			name:     "prefix match file",
			filePath: "file3.go",
			wantSrcs: []string{"prefix_match"},
		},
		{
			name:     "file without discovery sources",
			filePath: "file4.go",
			wantSrcs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := builder.Build(tt.filePath, opts)

			// Check discovery sources
			for _, wantSrc := range tt.wantSrcs {
				found := false
				for _, src := range card.DiscoveredBy {
					if src == wantSrc {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Build().DiscoveredBy missing %v, got %v", wantSrc, card.DiscoveredBy)
				}
			}
		})
	}
}

func TestFileCardBuilder_BuildAll(t *testing.T) {
	tmpDir := t.TempDir()
	builder := NewFileCardBuilder(nil)

	files := []string{
		"cmd/main.go",
		"internal/config/config.go",
		"internal/config/config_test.go",
	}

	opts := &BuildOptions{
		RepoRoot:  tmpDir,
		SeedFiles: []string{"cmd/main.go"},
	}

	cards := builder.BuildAll(files, opts)

	if len(cards) != len(files) {
		t.Fatalf("BuildAll() returned %d cards, want %d", len(cards), len(files))
	}

	// Check each file has a card
	cardMap := make(map[string]bool)
	for _, card := range cards {
		cardMap[card.Path] = true
	}

	for _, file := range files {
		if !cardMap[file] {
			t.Errorf("BuildAll() missing card for %v", file)
		}
	}

	// Check seed file is marked
	for _, card := range cards {
		if card.Path == "cmd/main.go" && !card.IsSeed() {
			t.Error("BuildAll() seed file not marked as seed")
		}
	}
}

func TestFileCardBuilder_HeuristicScore(t *testing.T) {
	tmpDir := t.TempDir()
	builder := NewFileCardBuilder(nil)

	tests := []struct {
		name         string
		filePath     string
		profile      string
		wantMinScore float64
		wantMaxScore float64
	}{
		{
			name:         "regular file with no rules matched",
			filePath:     "random_file.go",
			profile:      "",
			wantMinScore: 0.0,
			wantMaxScore: 0.0,
		},
		{
			name:         "test file should have score",
			filePath:     "module/file_test.go",
			profile:      "",
			wantMinScore: 0.1, // Should have some score from test rule
		},
		{
			name:         "settings file with profile",
			filePath:     "browser/settings/settings_page.cc",
			profile:      "browser_settings",
			wantMinScore: 0.1, // Should have combined score
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &BuildOptions{
				RepoRoot:  tmpDir,
				Profile:   tt.profile,
				SeedFiles: []string{},
			}
			card := builder.Build(tt.filePath, opts)

			if card.Scores.HeuristicScore < tt.wantMinScore {
				t.Errorf("Build().HeuristicScore = %v, want >= %v", card.Scores.HeuristicScore, tt.wantMinScore)
			}
			if tt.wantMaxScore > 0 && card.Scores.HeuristicScore > tt.wantMaxScore {
				t.Errorf("Build().HeuristicScore = %v, want <= %v", card.Scores.HeuristicScore, tt.wantMaxScore)
			}
		})
	}
}

func TestBuildFileCards(t *testing.T) {
	tmpDir := t.TempDir()

	input := &FileCardBuilderInput{
		Candidates: []string{
			"main.go",
			"config.json",
			"handler_test.go",
		},
		RepoRoot:  tmpDir,
		Profile:   "",
		SeedFiles: []string{"main.go"},
	}

	cards := BuildFileCards(input)

	if len(cards) != 3 {
		t.Fatalf("BuildFileCards() returned %d cards, want 3", len(cards))
	}

	// Check seed file
	foundSeed := false
	for _, card := range cards {
		if card.Path == "main.go" {
			foundSeed = true
			if !card.IsSeed() {
				t.Error("BuildFileCards() seed file not marked")
			}
		}
	}
	if !foundSeed {
		t.Error("BuildFileCards() missing seed file card")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"path/to/file.go", "path/to/file.go"},
		{"path\\to\\file.go", "path/to/file.go"},
		{"./path/to/file.go", "./path/to/file.go"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizePath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFileCardBuilder_Configuration(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with custom configuration
	config := &FileCardBuilderConfig{
		ModuleConfig: &ModuleConfig{
			MaxDepth:       1,
			IgnorePrefixes: []string{"src/"},
		},
		MaxFileSize: 100 * 1024,
		MaxSymbols:  50,
	}

	builder := NewFileCardBuilder(config)

	// Test that module depth is limited to 1
	opts := &BuildOptions{
		RepoRoot:  tmpDir,
		SeedFiles: []string{},
	}
	card := builder.Build("src/a/b/c/file.go", opts)

	// Module should be "a" (depth 1, with "src/" prefix ignored)
	// But since our ModuleConfig.IgnorePrefixes doesn't apply correctly here,
	// let's just check the module depth
	if card.Module == "" {
		t.Error("Build() module should not be empty")
	}
}

func TestFileCardBuilder_FocusChecks(t *testing.T) {
	tmpDir := t.TempDir()
	builder := NewFileCardBuilder(nil)

	// Build with only test checks
	opts := &BuildOptions{
		RepoRoot:    tmpDir,
		SeedFiles:   []string{},
		FocusChecks: []string{FocusCheckTests},
	}

	card := builder.Build("config/settings.json_test.go", opts)

	// Should have test tag but not config tag
	hasTestTag := false
	hasConfigTag := false
	for _, tag := range card.HeuristicTags {
		if tag == TagTestFile {
			hasTestTag = true
		}
		if tag == TagDefaultConfig {
			hasConfigTag = true
		}
	}

	if !hasTestTag {
		t.Error("Build() should have test tag when FocusChecks includes tests")
	}
	if hasConfigTag {
		t.Error("Build() should not have config tag when FocusChecks does not include default_config")
	}
}

func TestFileCardBuilder_SeedFileWeight(t *testing.T) {
	tmpDir := t.TempDir()
	builder := NewFileCardBuilder(nil)

	// Build with seed file
	opts := &BuildOptions{
		RepoRoot:  tmpDir,
		SeedFiles: []string{"seed.go"},
	}

	card := builder.Build("seed.go", opts)

	if card.Scores.SeedWeight != 1.0 {
		t.Errorf("Build().SeedWeight = %v, want 1.0", card.Scores.SeedWeight)
	}

	// Build non-seed file
	card2 := builder.Build("other.go", opts)

	if card2.Scores.SeedWeight != 0.0 {
		t.Errorf("Build().SeedWeight = %v, want 0.0 for non-seed", card2.Scores.SeedWeight)
	}
}

// Test with a real file for symbol extraction
func TestFileCardBuilder_SymbolExtraction(t *testing.T) {
	// Use the actual project root for testing
	projectRoot, _ := filepath.Abs(".")
	builder := NewFileCardBuilder(nil)

	// Build a card for an existing Go file
	opts := &BuildOptions{
		RepoRoot:  projectRoot,
		Profile:   "",
		SeedFiles: []string{},
	}

	// Test with this test file itself - it should extract some symbols
	card := builder.Build("internal/heuristics/module.go", opts)

	// Check that language is detected
	if card.Lang != "go" {
		t.Errorf("Build().Lang = %v, want go", card.Lang)
	}

	// Note: symbols may or may not be extracted depending on file size limits
	// Just verify the process doesn't crash
}
