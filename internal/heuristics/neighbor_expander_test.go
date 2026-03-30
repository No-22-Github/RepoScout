package heuristics

import (
	"sort"
	"testing"
)

func TestNeighborExpander_Expand(t *testing.T) {
	// Create a sample file list representing a repository
	allFiles := []string{
		"browser/settings/settings_page.cc",
		"browser/settings/settings_handler.cc",
		"browser/settings/settings_utils.cc",
		"browser/ui/browser_window.cc",
		"browser/ui/browser_tab.cc",
		"chrome/browser/chrome_main.cc",
		"chrome/browser/chrome_launcher.cc",
		"content/renderer/render_view.cc",
		"content/renderer/render_frame.cc",
		"base/files/file_util.cc",
		"base/files/file_path.cc",
		"base/memory/ref_counted.cc",
		"third_party/blink/renderer/core/dom/node.cc",
		"third_party/blink/renderer/core/dom/element.cc",
		"main.cc",
	}

	tests := []struct {
		name            string
		seedFiles       []string
		config          *ExpandConfig
		wantContains    []string // files that must be in the result
		wantNotContains []string // files that must NOT be in the result
	}{
		{
			name:            "empty seeds return empty result",
			seedFiles:       []string{},
			config:          nil,
			wantContains:    []string{},
			wantNotContains: allFiles,
		},
		{
			name:      "single seed includes same directory",
			seedFiles: []string{"browser/settings/settings_page.cc"},
			config:    nil,
			wantContains: []string{
				"browser/settings/settings_page.cc",
				"browser/settings/settings_handler.cc",
				"browser/settings/settings_utils.cc",
			},
			wantNotContains: []string{
				"content/renderer/render_view.cc",
			},
		},
		{
			name:      "single seed includes same module",
			seedFiles: []string{"browser/ui/browser_window.cc"},
			config:    nil,
			wantContains: []string{
				"browser/ui/browser_window.cc",
				"browser/ui/browser_tab.cc",
			},
			wantNotContains: []string{
				"base/files/file_util.cc",
			},
		},
		{
			name:      "prefix match finds related files",
			seedFiles: []string{"chrome/browser/chrome_main.cc"},
			config:    nil,
			wantContains: []string{
				"chrome/browser/chrome_main.cc",
				"chrome/browser/chrome_launcher.cc",
			},
			wantNotContains: []string{
				"browser/settings/settings_page.cc",
			},
		},
		{
			name:      "disable same dir expansion",
			seedFiles: []string{"browser/settings/settings_page.cc"},
			config: &ExpandConfig{
				IncludeSameDir:      false,
				IncludeSameModule:   false,
				IncludePrefixMatch:  false,
				IncludeSiblingMatch: false,
			},
			wantContains: []string{
				"browser/settings/settings_page.cc",
			},
			wantNotContains: []string{
				"browser/settings/settings_handler.cc",
			},
		},
		{
			name:      "multiple seeds combine results",
			seedFiles: []string{"browser/settings/settings_page.cc", "base/files/file_util.cc"},
			config:    nil,
			wantContains: []string{
				"browser/settings/settings_page.cc",
				"browser/settings/settings_handler.cc",
				"base/files/file_util.cc",
				"base/files/file_path.cc",
			},
			wantNotContains: []string{
				"main.cc",
			},
		},
		{
			name:      "root level seed file",
			seedFiles: []string{"main.cc"},
			config:    nil,
			wantContains: []string{
				"main.cc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expander := NewNeighborExpander(tt.config)
			result := expander.Expand(tt.seedFiles, allFiles)

			// Check that result is sorted
			if !sort.StringsAreSorted(result) {
				t.Errorf("result is not sorted")
			}

			// Create a set for quick lookup
			resultSet := make(map[string]bool)
			for _, f := range result {
				resultSet[f] = true
			}

			// Check required files are present
			for _, f := range tt.wantContains {
				if !resultSet[f] {
					t.Errorf("missing expected file %q in result", f)
				}
			}

			// Check excluded files are not present
			for _, f := range tt.wantNotContains {
				if resultSet[f] {
					t.Errorf("unexpected file %q in result", f)
				}
			}

			// Verify all seed files are always included
			for _, seed := range tt.seedFiles {
				if !resultSet[seed] {
					t.Errorf("seed file %q not in result", seed)
				}
			}
		})
	}
}

func TestNeighborExpander_ExpandWithSources(t *testing.T) {
	allFiles := []string{
		"browser/settings/settings_page.cc",
		"browser/settings/settings_handler.cc",
		"browser/ui/browser_window.cc",
		"chrome/browser/chrome_main.cc",
		"chrome/browser/chrome_launcher.cc",
	}

	seedFiles := []string{"browser/settings/settings_page.cc"}

	expander := NewNeighborExpander(nil)
	result := expander.ExpandWithSources(seedFiles, allFiles)

	// Verify result structure
	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Candidates) == 0 {
		t.Error("no candidates found")
	}

	if result.Sources == nil {
		t.Error("sources map is nil")
	}

	// Verify seed file is marked with seed source
	sources := result.Sources["browser/settings/settings_page.cc"]
	foundSeedSource := false
	for _, s := range sources {
		if s == SourceSeed {
			foundSeedSource = true
			break
		}
	}
	if !foundSeedSource {
		t.Error("seed file not marked with SourceSeed")
	}

	// Verify same directory file is marked correctly
	handlerSources := result.Sources["browser/settings/settings_handler.cc"]
	if len(handlerSources) == 0 {
		t.Error("settings_handler.cc should have been discovered")
	}
}

func TestNeighborExpander_ExpandMatchesExpandWithSourcesCandidates(t *testing.T) {
	allFiles := []string{
		"internal/auth/handler.go",
		"internal/auth/handler_test.go",
		"internal/auth/middleware.go",
		"internal/config/config.go",
	}

	seedFiles := []string{"internal/auth/handler.go"}
	expander := NewNeighborExpander(nil)

	plain := expander.Expand(seedFiles, allFiles)
	withSources := expander.ExpandWithSources(seedFiles, allFiles)

	if len(plain) != len(withSources.Candidates) {
		t.Fatalf("candidate length mismatch: Expand=%v ExpandWithSources=%v", plain, withSources.Candidates)
	}
	for i := range plain {
		if plain[i] != withSources.Candidates[i] {
			t.Fatalf("candidate mismatch at %d: Expand=%v ExpandWithSources=%v", i, plain, withSources.Candidates)
		}
	}
}

func TestExpandNeighbors(t *testing.T) {
	allFiles := []string{
		"a/b/file1.go",
		"a/b/file2.go",
		"a/c/file3.go",
		"d/e/other.go", // use different name prefix to avoid prefix match
	}

	seedFiles := []string{"a/b/file1.go"}

	result := ExpandNeighbors(seedFiles, allFiles)

	// Should include same directory files
	resultSet := make(map[string]bool)
	for _, f := range result {
		resultSet[f] = true
	}

	if !resultSet["a/b/file1.go"] {
		t.Error("seed file missing")
	}
	if !resultSet["a/b/file2.go"] {
		t.Error("same directory file missing")
	}
	if resultSet["d/e/other.go"] {
		t.Error("unrelated file should not be included")
	}
}

func TestExpandNeighborsWithConfig(t *testing.T) {
	allFiles := []string{
		"module1/file_a.go",
		"module1/file_b.go",
		"module2/file_c.go",
	}

	seedFiles := []string{"module1/file_a.go"}

	// Config that only does same directory
	config := &ExpandConfig{
		IncludeSameDir:      true,
		IncludeSameModule:   false,
		IncludePrefixMatch:  false,
		IncludeSiblingMatch: false,
	}

	result := ExpandNeighborsWithConfig(seedFiles, allFiles, config)

	resultSet := make(map[string]bool)
	for _, f := range result {
		resultSet[f] = true
	}

	if !resultSet["module1/file_a.go"] {
		t.Error("seed file missing")
	}
	if !resultSet["module1/file_b.go"] {
		t.Error("same directory file missing")
	}
	if resultSet["module2/file_c.go"] {
		t.Error("unrelated file should not be included")
	}
}

func TestPrefixMatch(t *testing.T) {
	// Test that prefix matching works correctly
	allFiles := []string{
		"handler_user.go",
		"handler_post.go",
		"handler_comment.go",
		"service_auth.go",
		"service_user.go",
		"model.go",
	}

	seedFiles := []string{"handler_user.go"}

	// Use config that only does prefix match
	config := &ExpandConfig{
		IncludeSameDir:      false,
		IncludeSameModule:   false,
		IncludePrefixMatch:  true,
		IncludeSiblingMatch: false,
		PrefixMinLength:     3,
	}

	result := ExpandNeighborsWithConfig(seedFiles, allFiles, config)

	resultSet := make(map[string]bool)
	for _, f := range result {
		resultSet[f] = true
	}

	// Seed file should always be present
	if !resultSet["handler_user.go"] {
		t.Error("seed file missing")
	}

	// Other handler files should match by prefix
	if !resultSet["handler_post.go"] {
		t.Error("handler_post.go should match by prefix")
	}
	if !resultSet["handler_comment.go"] {
		t.Error("handler_comment.go should match by prefix")
	}

	// Service files should not match
	if resultSet["service_auth.go"] {
		t.Error("service_auth.go should not match")
	}
}

func TestPrefixMatchShortNames(t *testing.T) {
	// Test that short names are handled correctly
	allFiles := []string{
		"ab.go",
		"abc.go",
		"abcd.go",
		"abcde.go",
	}

	seedFiles := []string{"ab.go"}

	config := &ExpandConfig{
		IncludeSameDir:      false,
		IncludeSameModule:   false,
		IncludePrefixMatch:  true,
		IncludeSiblingMatch: false,
		PrefixMinLength:     3,
	}

	result := ExpandNeighborsWithConfig(seedFiles, allFiles, config)

	// Short name "ab" should not trigger prefix matching
	// Only the seed file should be present
	if len(result) != 1 || result[0] != "ab.go" {
		t.Errorf("expected only seed file, got %v", result)
	}
}

func TestModuleExpansionWithCustomModuleConfig(t *testing.T) {
	allFiles := []string{
		"src/components/Button.tsx",
		"src/components/Input.tsx",
		"src/pages/Home.tsx",
		"lib/utils/helper.ts",
	}

	seedFiles := []string{"src/components/Button.tsx"}

	// Custom module config that strips "src/" prefix
	moduleConfig := &ModuleConfig{
		MaxDepth:       2,
		IgnorePrefixes: []string{"src/", "lib/"},
	}

	config := &ExpandConfig{
		ModuleConfig:        moduleConfig,
		IncludeSameDir:      false,
		IncludeSameModule:   true,
		IncludePrefixMatch:  false,
		IncludeSiblingMatch: false,
	}

	result := ExpandNeighborsWithConfig(seedFiles, allFiles, config)

	resultSet := make(map[string]bool)
	for _, f := range result {
		resultSet[f] = true
	}

	// Files in "components" module should be included
	if !resultSet["src/components/Button.tsx"] {
		t.Error("seed file missing")
	}
	if !resultSet["src/components/Input.tsx"] {
		t.Error("same module file missing")
	}

	// Files in "pages" module should not be included
	if resultSet["src/pages/Home.tsx"] {
		t.Error("different module file should not be included")
	}
}

func TestSiblingMatchExpansion(t *testing.T) {
	allFiles := []string{
		"internal/auth/handler.go",
		"internal/auth/handler_test.go",
		"internal/auth/handler_mock.go",
		"internal/auth/middleware.go",
		"internal/native/bridge.cc",
		"internal/native/bridge.h",
	}

	t.Run("implementation pulls in test companion", func(t *testing.T) {
		result := ExpandNeighbors([]string{"internal/auth/handler.go"}, allFiles)
		resultSet := make(map[string]bool)
		for _, f := range result {
			resultSet[f] = true
		}

		if !resultSet["internal/auth/handler_test.go"] {
			t.Fatalf("expected handler_test.go to be included, got %v", result)
		}
		if !resultSet["internal/auth/handler_mock.go"] {
			t.Fatalf("expected handler_mock.go to be included, got %v", result)
		}
	})

	t.Run("source pulls in header companion", func(t *testing.T) {
		config := &ExpandConfig{
			IncludeSameDir:      false,
			IncludeSameModule:   false,
			IncludePrefixMatch:  false,
			IncludeSiblingMatch: true,
		}
		result := ExpandNeighborsWithConfig([]string{"internal/native/bridge.cc"}, allFiles, config)
		resultSet := make(map[string]bool)
		for _, f := range result {
			resultSet[f] = true
		}

		if !resultSet["internal/native/bridge.h"] {
			t.Fatalf("expected bridge.h to be included, got %v", result)
		}
		if resultSet["internal/auth/middleware.go"] {
			t.Fatalf("unexpected unrelated file in result: %v", result)
		}
	})
}

func TestExpandWithSources_SiblingMatchSource(t *testing.T) {
	allFiles := []string{
		"pkg/task/task_context.go",
		"pkg/task/task_context_test.go",
	}

	expander := NewNeighborExpander(&ExpandConfig{
		IncludeSameDir:      false,
		IncludeSameModule:   false,
		IncludePrefixMatch:  false,
		IncludeSiblingMatch: true,
	})
	result := expander.ExpandWithSources([]string{"pkg/task/task_context.go"}, allFiles)

	sources := result.Sources["pkg/task/task_context_test.go"]
	found := false
	for _, source := range sources {
		if source == SourceSiblingMatch {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected sibling match source, got %v", sources)
	}
}

func TestExpansionResultSorted(t *testing.T) {
	allFiles := []string{
		"z/last.go",
		"a/first.go",
		"m/middle.go",
	}

	seedFiles := []string{"a/first.go", "z/last.go"}

	result := ExpandNeighbors(seedFiles, allFiles)

	// Verify result is sorted
	if !sort.StringsAreSorted(result) {
		t.Errorf("result is not sorted: %v", result)
	}

	// Verify seeds are included
	resultSet := make(map[string]bool)
	for _, f := range result {
		resultSet[f] = true
	}
	if !resultSet["a/first.go"] {
		t.Error("seed file 'a/first.go' missing")
	}
	if !resultSet["z/last.go"] {
		t.Error("seed file 'z/last.go' missing")
	}

	// Verify order (sorted alphabetically)
	if len(result) >= 1 && result[0] != "a/first.go" {
		t.Errorf("expected first file to be 'a/first.go', got %q", result[0])
	}
	if len(result) >= 2 && result[len(result)-1] != "z/last.go" {
		t.Errorf("expected last file to be 'z/last.go', got %q", result[len(result)-1])
	}
}
