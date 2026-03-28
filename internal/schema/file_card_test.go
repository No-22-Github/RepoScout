package schema

import (
	"encoding/json"
	"sort"
	"testing"
)

func TestNewFileCard(t *testing.T) {
	fc := NewFileCard("internal/auth/handler.go")

	if fc.Path != "internal/auth/handler.go" {
		t.Errorf("Path = %v, want internal/auth/handler.go", fc.Path)
	}
	if fc.Symbols == nil {
		t.Error("Symbols should not be nil")
	}
	if fc.Neighbors == nil {
		t.Error("Neighbors should not be nil")
	}
	if fc.DiscoveredBy == nil {
		t.Error("DiscoveredBy should not be nil")
	}
	if fc.HeuristicTags == nil {
		t.Error("HeuristicTags should not be nil")
	}
	if fc.Scores == nil {
		t.Error("Scores should not be nil")
	}
}

func TestFileCard_AddDiscoveredBy(t *testing.T) {
	fc := NewFileCard("test.go")

	fc.AddDiscoveredBy("seed")
	if len(fc.DiscoveredBy) != 1 || fc.DiscoveredBy[0] != "seed" {
		t.Errorf("DiscoveredBy = %v, want [seed]", fc.DiscoveredBy)
	}

	// Adding duplicate should not increase length
	fc.AddDiscoveredBy("seed")
	if len(fc.DiscoveredBy) != 1 {
		t.Errorf("DiscoveredBy = %v, should still have 1 element", fc.DiscoveredBy)
	}

	// Adding different method
	fc.AddDiscoveredBy("same-module")
	if len(fc.DiscoveredBy) != 2 {
		t.Errorf("DiscoveredBy = %v, should have 2 elements", fc.DiscoveredBy)
	}
}

func TestFileCard_AddHeuristicTag(t *testing.T) {
	fc := NewFileCard("test.go")

	fc.AddHeuristicTag("tests")
	if len(fc.HeuristicTags) != 1 || fc.HeuristicTags[0] != "tests" {
		t.Errorf("HeuristicTags = %v, want [tests]", fc.HeuristicTags)
	}

	// Adding duplicate should not increase length
	fc.AddHeuristicTag("tests")
	if len(fc.HeuristicTags) != 1 {
		t.Errorf("HeuristicTags = %v, should still have 1 element", fc.HeuristicTags)
	}

	// Adding different tag
	fc.AddHeuristicTag("default_config")
	if len(fc.HeuristicTags) != 2 {
		t.Errorf("HeuristicTags = %v, should have 2 elements", fc.HeuristicTags)
	}
}

func TestFileCard_AddSymbol(t *testing.T) {
	fc := NewFileCard("test.go")

	fc.AddSymbol("LoginHandler")
	if len(fc.Symbols) != 1 || fc.Symbols[0] != "LoginHandler" {
		t.Errorf("Symbols = %v, want [LoginHandler]", fc.Symbols)
	}

	// Adding duplicate should not increase length
	fc.AddSymbol("LoginHandler")
	if len(fc.Symbols) != 1 {
		t.Errorf("Symbols = %v, should still have 1 element", fc.Symbols)
	}

	// Adding different symbol
	fc.AddSymbol("AuthMiddleware")
	if len(fc.Symbols) != 2 {
		t.Errorf("Symbols = %v, should have 2 elements", fc.Symbols)
	}
}

func TestFileCard_AddNeighbor(t *testing.T) {
	fc := NewFileCard("test.go")

	fc.AddNeighbor("internal/auth/types.go")
	if len(fc.Neighbors) != 1 || fc.Neighbors[0] != "internal/auth/types.go" {
		t.Errorf("Neighbors = %v, want [internal/auth/types.go]", fc.Neighbors)
	}

	// Adding duplicate should not increase length
	fc.AddNeighbor("internal/auth/types.go")
	if len(fc.Neighbors) != 1 {
		t.Errorf("Neighbors = %v, should still have 1 element", fc.Neighbors)
	}
}

func TestFileCard_IsSeed(t *testing.T) {
	tests := []struct {
		name     string
		fc       *FileCard
		expected bool
	}{
		{
			name:     "is seed",
			fc:       &FileCard{Path: "main.go", DiscoveredBy: []string{"seed", "same-module"}},
			expected: true,
		},
		{
			name:     "not seed",
			fc:       &FileCard{Path: "handler.go", DiscoveredBy: []string{"same-module"}},
			expected: false,
		},
		{
			name:     "empty discovered_by",
			fc:       &FileCard{Path: "other.go", DiscoveredBy: []string{}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fc.IsSeed(); got != tt.expected {
				t.Errorf("IsSeed() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFileCard_ToJSON(t *testing.T) {
	fc := &FileCard{
		Path:          "internal/auth/handler.go",
		Lang:          "go",
		Module:        "auth",
		Symbols:       []string{"LoginHandler", "AuthMiddleware"},
		Neighbors:     []string{"internal/auth/types.go"},
		DiscoveredBy:  []string{"seed"},
		HeuristicTags: []string{"handler"},
		Scores: &FileScores{
			SeedWeight:     1.0,
			ModuleWeight:   0.8,
			HeuristicScore: 0.5,
			FinalScore:     0.85,
		},
	}

	data, err := fc.ToJSON()
	if err != nil {
		t.Errorf("ToJSON() unexpected error: %v", err)
		return
	}

	// Verify we can parse it back
	var parsed FileCard
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Failed to parse ToJSON output: %v", err)
		return
	}

	if parsed.Path != fc.Path {
		t.Errorf("Path mismatch: got %v, want %v", parsed.Path, fc.Path)
	}
	if parsed.Lang != fc.Lang {
		t.Errorf("Lang mismatch: got %v, want %v", parsed.Lang, fc.Lang)
	}
	if parsed.Module != fc.Module {
		t.Errorf("Module mismatch: got %v, want %v", parsed.Module, fc.Module)
	}
	if len(parsed.Symbols) != len(fc.Symbols) {
		t.Errorf("Symbols length mismatch: got %v, want %v", len(parsed.Symbols), len(fc.Symbols))
	}
}

func TestFileCard_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		fc   *FileCard
	}{
		{
			name: "minimal file card",
			fc:   NewFileCard("main.go"),
		},
		{
			name: "full file card",
			fc: &FileCard{
				Path:          "internal/config/settings.go",
				Lang:          "go",
				Module:        "config",
				Symbols:       []string{"LoadConfig", "SaveConfig", "Config"},
				Neighbors:     []string{"internal/config/defaults.go", "internal/config/validate.go"},
				DiscoveredBy:  []string{"seed", "same-module"},
				HeuristicTags: []string{"tests", "default_config"},
				Scores: &FileScores{
					SeedWeight:     1.0,
					ModuleWeight:   0.9,
					HeuristicScore: 0.7,
					ProfileScore:   0.6,
					LLMLabel:       "main_chain",
					LLMConfidence:  0.85,
					FinalScore:     0.88,
				},
			},
		},
		{
			name: "file card with nil slices",
			fc: &FileCard{
				Path:   "test.go",
				Lang:   "go",
				Scores: &FileScores{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.fc.ToJSON()
			if err != nil {
				t.Errorf("ToJSON() error: %v", err)
				return
			}

			var parsed FileCard
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Errorf("Unmarshal() error: %v", err)
				return
			}

			if parsed.Path != tt.fc.Path {
				t.Errorf("Path mismatch: got %v, want %v", parsed.Path, tt.fc.Path)
			}
		})
	}
}

func TestFileCardList_Sort(t *testing.T) {
	list := FileCardList{
		&FileCard{Path: "a.go", Scores: &FileScores{FinalScore: 0.5}},
		&FileCard{Path: "b.go", Scores: &FileScores{FinalScore: 0.9}},
		&FileCard{Path: "c.go", Scores: &FileScores{FinalScore: 0.3}},
		&FileCard{Path: "d.go", Scores: &FileScores{FinalScore: 0.7}},
	}

	sort.Sort(list)

	expected := []string{"b.go", "d.go", "a.go", "c.go"}
	for i, fc := range list {
		if fc.Path != expected[i] {
			t.Errorf("list[%d].Path = %v, want %v", i, fc.Path, expected[i])
		}
	}
}

func TestFileCardList_Paths(t *testing.T) {
	list := FileCardList{
		&FileCard{Path: "a.go"},
		&FileCard{Path: "b.go"},
		&FileCard{Path: "c.go"},
	}

	paths := list.Paths()
	if len(paths) != 3 {
		t.Errorf("Paths() length = %v, want 3", len(paths))
		return
	}

	expected := []string{"a.go", "b.go", "c.go"}
	for i, p := range paths {
		if p != expected[i] {
			t.Errorf("paths[%d] = %v, want %v", i, p, expected[i])
		}
	}
}
