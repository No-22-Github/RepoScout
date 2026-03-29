package pack

import (
	"testing"

	"github.com/no22/repo-scout/internal/ranking"
	"github.com/no22/repo-scout/internal/schema"
)

func TestDefaultBuilderConfig(t *testing.T) {
	config := DefaultBuilderConfig()
	if config.MainChainThreshold != 0.5 {
		t.Errorf("expected MainChainThreshold 0.5, got %f", config.MainChainThreshold)
	}
	if config.CompanionThreshold != 0.3 {
		t.Errorf("expected CompanionThreshold 0.3, got %f", config.CompanionThreshold)
	}
	if config.UncertainThreshold != 0.1 {
		t.Errorf("expected UncertainThreshold 0.1, got %f", config.UncertainThreshold)
	}
	if config.MaxMainChain != 10 {
		t.Errorf("expected MaxMainChain 10, got %d", config.MaxMainChain)
	}
	if config.MaxCompanion != 20 {
		t.Errorf("expected MaxCompanion 20, got %d", config.MaxCompanion)
	}
	if config.MaxUncertain != 10 {
		t.Errorf("expected MaxUncertain 10, got %d", config.MaxUncertain)
	}
}

func TestNewBuilder(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		b := NewBuilder(nil)
		if b == nil {
			t.Fatal("expected builder, got nil")
		}
		if b.config == nil {
			t.Fatal("expected config, got nil")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &BuilderConfig{
			MainChainThreshold: 0.7,
			MaxMainChain:       5,
		}
		b := NewBuilder(config)
		if b.config.MainChainThreshold != 0.7 {
			t.Errorf("expected MainChainThreshold 0.7, got %f", b.config.MainChainThreshold)
		}
	})
}

func TestBuild_EmptyInput(t *testing.T) {
	b := NewBuilder(nil)

	t.Run("nil rank result", func(t *testing.T) {
		pack := b.Build(&BuildInput{Task: "test task"})
		if pack == nil {
			t.Fatal("expected pack, got nil")
		}
		if pack.Task != "test task" {
			t.Errorf("expected task 'test task', got %s", pack.Task)
		}
		if len(pack.MainChain) != 0 {
			t.Errorf("expected empty main chain, got %d files", len(pack.MainChain))
		}
	})

	t.Run("empty cards", func(t *testing.T) {
		result := &ranking.RankResult{Cards: []*schema.FileCard{}}
		pack := b.Build(&BuildInput{Task: "test", RankResult: result})
		if len(pack.MainChain) != 0 {
			t.Errorf("expected empty main chain, got %d files", len(pack.MainChain))
		}
	})
}

func TestBuild_Classification(t *testing.T) {
	b := NewBuilder(nil)

	// Create test cards with different scores
	cards := []*schema.FileCard{
		createCardWithScore("main1.go", 0.8),
		createCardWithScore("main2.go", 0.6),
		createCardWithScore("companion1.go", 0.4),
		createCardWithScore("companion2.go", 0.35),
		createCardWithScore("uncertain1.go", 0.2),
		createCardWithScore("filtered.go", 0.05), // Below threshold, should be filtered
	}

	result := &ranking.RankResult{Cards: cards}
	pack := b.Build(&BuildInput{Task: "test task", RankResult: result})

	// Check main chain (score >= 0.5)
	if len(pack.MainChain) != 2 {
		t.Errorf("expected 2 main chain files, got %d: %v", len(pack.MainChain), pack.MainChain)
	}

	// Check companion (0.3 <= score < 0.5)
	if len(pack.CompanionFiles) != 2 {
		t.Errorf("expected 2 companion files, got %d: %v", len(pack.CompanionFiles), pack.CompanionFiles)
	}

	// Check uncertain (0.1 <= score < 0.3)
	if len(pack.UncertainNodes) != 1 {
		t.Errorf("expected 1 uncertain file, got %d: %v", len(pack.UncertainNodes), pack.UncertainNodes)
	}

	// Verify filtered file is not present
	for _, f := range pack.AllFiles() {
		if f == "filtered.go" {
			t.Error("filtered.go should have been filtered out")
		}
	}
}

func TestBuild_Limits(t *testing.T) {
	config := &BuilderConfig{
		MainChainThreshold: 0.5,
		CompanionThreshold: 0.3,
		UncertainThreshold: 0.1,
		MaxMainChain:       2,
		MaxCompanion:       1,
		MaxUncertain:       0, // No limit
	}
	b := NewBuilder(config)

	// Create more files than limits allow
	cards := []*schema.FileCard{
		createCardWithScore("main1.go", 0.9),
		createCardWithScore("main2.go", 0.8),
		createCardWithScore("main3.go", 0.7), // Should be cut
		createCardWithScore("comp1.go", 0.4),
		createCardWithScore("comp2.go", 0.35), // Should be cut
		createCardWithScore("unc1.go", 0.2),
		createCardWithScore("unc2.go", 0.15),
	}

	result := &ranking.RankResult{Cards: cards}
	pack := b.Build(&BuildInput{Task: "test", RankResult: result})

	if len(pack.MainChain) != 2 {
		t.Errorf("expected 2 main chain files (limit), got %d", len(pack.MainChain))
	}
	if len(pack.CompanionFiles) != 1 {
		t.Errorf("expected 1 companion file (limit), got %d", len(pack.CompanionFiles))
	}
	if len(pack.UncertainNodes) != 2 {
		t.Errorf("expected 2 uncertain files (no limit), got %d", len(pack.UncertainNodes))
	}
}

func TestBuild_ReadingOrder(t *testing.T) {
	b := NewBuilder(nil)

	cards := []*schema.FileCard{
		createCardWithScore("main1.go", 0.9),
		createCardWithScore("main2.go", 0.6),
		createCardWithScore("comp1.go", 0.4),
	}

	result := &ranking.RankResult{Cards: cards}
	pack := b.Build(&BuildInput{Task: "test", RankResult: result})

	if len(pack.ReadingOrder) == 0 {
		t.Error("expected reading order to be generated")
	}

	// All files should be in reading order
	seen := make(map[string]bool)
	for _, f := range pack.ReadingOrder {
		seen[f] = true
	}

	for _, f := range pack.MainChain {
		if !seen[f] {
			t.Errorf("main chain file %s not in reading order", f)
		}
	}
	for _, f := range pack.CompanionFiles {
		if !seen[f] {
			t.Errorf("companion file %s not in reading order", f)
		}
	}
}

func TestBuild_ReadingOrderSeedFirst(t *testing.T) {
	b := NewBuilder(nil)

	cards := []*schema.FileCard{
		createCardWithScore("main.go", 0.8),
		createCardWithScore("seed-companion.go", 0.4),
		createCardWithScore("other-companion.go", 0.35),
	}

	result := &ranking.RankResult{Cards: cards}
	pack := b.Build(&BuildInput{
		Task:       "test",
		RankResult: result,
		Request: &schema.ReconRequest{
			SeedFiles: []string{"seed-companion.go"},
		},
	})

	if len(pack.ReadingOrder) < 2 {
		t.Fatalf("expected reading order to contain at least 2 files, got %v", pack.ReadingOrder)
	}
	if pack.ReadingOrder[1] != "seed-companion.go" {
		t.Errorf("expected seed companion to be read before other companions, got %v", pack.ReadingOrder)
	}
}

func TestBuild_RiskHints(t *testing.T) {
	b := NewBuilder(nil)

	t.Run("no test files warning", func(t *testing.T) {
		cards := []*schema.FileCard{
			createCardWithScore("main1.go", 0.8),
			createCardWithScore("main2.go", 0.7),
			createCardWithScore("main3.go", 0.6),
			createCardWithScore("main4.go", 0.55), // 4 main files, no tests
		}

		result := &ranking.RankResult{Cards: cards}
		pack := b.Build(&BuildInput{Task: "test", RankResult: result})

		// Should have test coverage warning
		found := false
		for _, hint := range pack.RiskHints {
			if hint.Category == "test-coverage" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected test-coverage risk hint when no tests present")
		}
	})

	t.Run("config files hint", func(t *testing.T) {
		configCard := createCardWithScore("config.yaml", 0.6)
		configCard.HeuristicTags = []string{"default_config"}

		cards := []*schema.FileCard{
			createCardWithScore("main.go", 0.8),
			configCard,
		}

		result := &ranking.RankResult{Cards: cards}
		pack := b.Build(&BuildInput{Task: "test", RankResult: result})

		// Should have config hint
		found := false
		for _, hint := range pack.RiskHints {
			if hint.Category == "configuration" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected configuration risk hint when config files present")
		}
	})
}

func TestBuild_Stats(t *testing.T) {
	b := NewBuilder(nil)

	cards := []*schema.FileCard{
		createCardWithScore("main1.go", 0.8),
		createCardWithScore("comp1.go", 0.4),
		createCardWithScore("unc1.go", 0.2),
	}

	result := &ranking.RankResult{Cards: cards}
	pack := b.Build(&BuildInput{Task: "test", RankResult: result})

	if pack.Stats == nil {
		t.Fatal("expected stats to be populated")
	}
	if pack.Stats.MainChainCount != 1 {
		t.Errorf("expected MainChainCount 1, got %d", pack.Stats.MainChainCount)
	}
	if pack.Stats.CompanionCount != 1 {
		t.Errorf("expected CompanionCount 1, got %d", pack.Stats.CompanionCount)
	}
	if pack.Stats.UncertainCount != 1 {
		t.Errorf("expected UncertainCount 1, got %d", pack.Stats.UncertainCount)
	}
	if pack.Stats.TotalFiles != 3 {
		t.Errorf("expected TotalFiles 3, got %d", pack.Stats.TotalFiles)
	}
	if pack.Stats.ModelEnhanced {
		t.Error("expected ModelEnhanced to be false for no-model version")
	}
}

func TestBuild_MaxTotalFiles(t *testing.T) {
	b := NewBuilder(&BuilderConfig{
		MainChainThreshold: 0.5,
		CompanionThreshold: 0.3,
		UncertainThreshold: 0.1,
		MaxMainChain:       10,
		MaxCompanion:       10,
		MaxUncertain:       10,
		MaxTotalFiles:      3,
	})

	cards := []*schema.FileCard{
		createCardWithScore("main1.go", 0.9),
		createCardWithScore("main2.go", 0.8),
		createCardWithScore("comp1.go", 0.4),
		createCardWithScore("unc1.go", 0.2),
	}

	result := &ranking.RankResult{Cards: cards}
	pack := b.Build(&BuildInput{Task: "test", RankResult: result})

	if len(pack.AllFiles()) != 3 {
		t.Fatalf("expected total files to be capped at 3, got %d", len(pack.AllFiles()))
	}
	if len(pack.UncertainNodes) != 0 {
		t.Errorf("expected lower-priority uncertain files to be trimmed first, got %v", pack.UncertainNodes)
	}
}

func TestBuild_SummaryMarkdown(t *testing.T) {
	b := NewBuilder(nil)

	cards := []*schema.FileCard{
		createCardWithScore("main.go", 0.8),
		createCardWithScore("comp.go", 0.4),
	}

	result := &ranking.RankResult{Cards: cards}
	pack := b.Build(&BuildInput{Task: "summarize", RankResult: result})

	if pack.SummaryMarkdown == "" {
		t.Fatal("expected summary_markdown to be populated")
	}
	if !containsString(pack.SummaryMarkdown, "main.go") {
		t.Errorf("expected summary_markdown to mention reading order files, got %q", pack.SummaryMarkdown)
	}
}

func TestBuild_ReproFamily(t *testing.T) {
	b := NewBuilder(nil)

	cards := []*schema.FileCard{createCardWithScore("main.go", 0.8)}
	result := &ranking.RankResult{Cards: cards}

	pack := b.Build(&BuildInput{
		Task:       "test",
		RepoFamily: "go-web",
		RankResult: result,
	})

	if pack.RepoFamily != "go-web" {
		t.Errorf("expected RepoFamily 'go-web', got %s", pack.RepoFamily)
	}
}

func TestBuildFromRankResult(t *testing.T) {
	cards := []*schema.FileCard{
		createCardWithScore("main.go", 0.8),
	}
	result := ranking.RankCards(cards)

	pack := BuildFromRankResult("test task", result)
	if pack == nil {
		t.Fatal("expected pack, got nil")
	}
	if pack.Task != "test task" {
		t.Errorf("expected task 'test task', got %s", pack.Task)
	}
}

func TestBuildFromCards(t *testing.T) {
	cards := []*schema.FileCard{
		createCardWithScore("main.go", 0.8),
		createCardWithScore("comp.go", 0.4),
	}

	pack := BuildFromCards("test task", cards)
	if pack == nil {
		t.Fatal("expected pack, got nil")
	}
	if pack.Task != "test task" {
		t.Errorf("expected task 'test task', got %s", pack.Task)
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"foo_test.go", true},
		{"bar.test.ts", true},
		{"baz.spec.tsx", true},
		{"test_foo.py", true},
		{"foo_test.py", true},
		{"src/test/main.go", true},
		{"src/tests/util.go", true},
		{"src/__tests__/app.ts", true},
		{"main.go", false},
		{"config.yaml", false},
		{"helper.go", false},
	}

	for _, tc := range tests {
		card := &schema.FileCard{Path: tc.path}
		result := isTestFile(card)
		if result != tc.expected {
			t.Errorf("isTestFile(%s) = %v, expected %v", tc.path, result, tc.expected)
		}
	}

	// Test with heuristic tags
	card := &schema.FileCard{
		Path:          "verify.go",
		HeuristicTags: []string{"tests"},
	}
	if !isTestFile(card) {
		t.Error("expected file with 'tests' tag to be identified as test file")
	}
}

func TestHasConfigTag(t *testing.T) {
	tests := []struct {
		tags     []string
		expected bool
	}{
		{[]string{"default_config"}, true},
		{[]string{"config"}, true},
		{[]string{"configuration"}, true},
		{[]string{"main", "handler"}, false},
		{[]string{}, false},
		{nil, false},
	}

	for _, tc := range tests {
		card := &schema.FileCard{HeuristicTags: tc.tags}
		result := hasConfigTag(card)
		if result != tc.expected {
			t.Errorf("hasConfigTag(%v) = %v, expected %v", tc.tags, result, tc.expected)
		}
	}
}

// Helper function to create a FileCard with a specific final score
func createCardWithScore(path string, score float64) *schema.FileCard {
	return &schema.FileCard{
		Path: path,
		Scores: &schema.FileScores{
			FinalScore: score,
		},
	}
}

func containsString(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && (s == substr || stringContainsAt(s, substr)))
}

func stringContainsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
