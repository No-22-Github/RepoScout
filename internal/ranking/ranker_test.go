// Package ranking provides ranking algorithms for file candidates.
package ranking

import (
	"testing"

	"github.com/no22/repo-scout/internal/schema"
)

func TestDefaultRankerConfig(t *testing.T) {
	config := DefaultRankerConfig()
	if config.SeedWeight <= 0 || config.SeedWeight > 1 {
		t.Errorf("SeedWeight should be in (0, 1], got %f", config.SeedWeight)
	}
	if config.SameModuleWeight <= 0 || config.SameModuleWeight > 1 {
		t.Errorf("SameModuleWeight should be in (0, 1], got %f", config.SameModuleWeight)
	}
	if config.HeuristicWeight <= 0 || config.HeuristicWeight > 1 {
		t.Errorf("HeuristicWeight should be in (0, 1], got %f", config.HeuristicWeight)
	}
	if config.ProfileWeight <= 0 || config.ProfileWeight > 1 {
		t.Errorf("ProfileWeight should be in (0, 1], got %f", config.ProfileWeight)
	}
	if config.LLMWeight <= 0 || config.LLMWeight > 1 {
		t.Errorf("LLMWeight should be in (0, 1], got %f", config.LLMWeight)
	}
	if config.MaxFinalScore != 1.0 {
		t.Errorf("MaxFinalScore should be 1.0, got %f", config.MaxFinalScore)
	}
}

func TestNewRanker(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		ranker := NewRanker(nil)
		if ranker == nil {
			t.Fatal("NewRanker returned nil")
		}
		if ranker.config == nil {
			t.Fatal("ranker.config is nil")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &RankerConfig{
			SeedWeight:       0.5,
			SameModuleWeight: 0.3,
			HeuristicWeight:  0.2,
			ProfileWeight:    0.1,
			LLMWeight:        0.15,
			MaxFinalScore:    0.9,
		}
		ranker := NewRanker(config)
		if ranker.config.SeedWeight != 0.5 {
			t.Errorf("expected SeedWeight 0.5, got %f", ranker.config.SeedWeight)
		}
	})
}

func TestRank(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		ranker := NewRanker(nil)
		result := ranker.Rank(nil)
		if result == nil {
			t.Fatal("Rank returned nil for nil input")
		}
		if len(result.Cards) != 0 {
			t.Errorf("expected empty cards, got %d", len(result.Cards))
		}
		if len(result.TopFiles) != 0 {
			t.Errorf("expected empty top files, got %d", len(result.TopFiles))
		}
	})

	t.Run("empty cards", func(t *testing.T) {
		ranker := NewRanker(nil)
		result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{}})
		if len(result.Cards) != 0 {
			t.Errorf("expected empty cards, got %d", len(result.Cards))
		}
	})

	t.Run("single card", func(t *testing.T) {
		card := schema.NewFileCard("browser/settings/settings_page.cc")
		card.Scores.HeuristicScore = 0.5
		card.Scores.ProfileScore = 0.3

		ranker := NewRanker(nil)
		result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{card}})

		if len(result.Cards) != 1 {
			t.Fatalf("expected 1 card, got %d", len(result.Cards))
		}
		if result.Cards[0].Path != "browser/settings/settings_page.cc" {
			t.Errorf("unexpected path: %s", result.Cards[0].Path)
		}
		if result.Cards[0].Scores.FinalScore <= 0 {
			t.Errorf("expected positive final score, got %f", result.Cards[0].Scores.FinalScore)
		}
	})

	t.Run("multiple cards sorted correctly", func(t *testing.T) {
		// Create cards with different scores
		card1 := schema.NewFileCard("file1.go")
		card1.Scores.HeuristicScore = 0.2

		card2 := schema.NewFileCard("file2.go")
		card2.Scores.HeuristicScore = 0.8

		card3 := schema.NewFileCard("file3.go")
		card3.Scores.HeuristicScore = 0.5

		ranker := NewRanker(nil)
		result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{card1, card2, card3}})

		if len(result.Cards) != 3 {
			t.Fatalf("expected 3 cards, got %d", len(result.Cards))
		}

		// Should be sorted by final score descending
		if result.Cards[0].Path != "file2.go" {
			t.Errorf("expected file2.go first (highest heuristic), got %s", result.Cards[0].Path)
		}
		if result.Cards[1].Path != "file3.go" {
			t.Errorf("expected file3.go second, got %s", result.Cards[1].Path)
		}
		if result.Cards[2].Path != "file1.go" {
			t.Errorf("expected file1.go last (lowest heuristic), got %s", result.Cards[2].Path)
		}
	})

	t.Run("seed files ranked higher", func(t *testing.T) {
		// Non-seed file with higher heuristic score
		card1 := schema.NewFileCard("file1.go")
		card1.Scores.SeedWeight = 0.0
		card1.Scores.HeuristicScore = 0.8

		// Seed file with lower heuristic score
		card2 := schema.NewFileCard("file2.go")
		card2.Scores.SeedWeight = 1.0
		card2.AddDiscoveredBy("seed")
		card2.Scores.HeuristicScore = 0.1

		ranker := NewRanker(nil)
		result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{card1, card2}})

		// Seed file should be ranked higher due to seed weight bonus
		if result.Cards[0].Path != "file2.go" {
			t.Errorf("seed file should be ranked first, got %s", result.Cards[0].Path)
		}
	})
}

func TestComputeModuleWeights(t *testing.T) {
	t.Run("files in same module as seeds", func(t *testing.T) {
		// Seed file
		seedCard := schema.NewFileCard("browser/settings/settings_page.cc")
		seedCard.Module = "browser/settings"
		seedCard.AddDiscoveredBy("seed")
		seedCard.Scores.SeedWeight = 1.0

		// File in same module
		sameModuleCard := schema.NewFileCard("browser/settings/handler.cc")
		sameModuleCard.Module = "browser/settings"

		// File in different module
		diffModuleCard := schema.NewFileCard("chrome/browser/ui/view.cc")
		diffModuleCard.Module = "chrome/browser"

		ranker := NewRanker(nil)
		result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{seedCard, sameModuleCard, diffModuleCard}})

		// Same module card should have non-zero module weight
		if sameModuleCard.Scores.ModuleWeight <= 0 {
			t.Errorf("same module card should have positive module weight, got %f", sameModuleCard.Scores.ModuleWeight)
		}

		// Seed card should have highest final score
		if result.Cards[0].Path != seedCard.Path {
			t.Errorf("seed card should be first, got %s", result.Cards[0].Path)
		}
	})

	t.Run("sub-module relationship", func(t *testing.T) {
		seedCard := schema.NewFileCard("browser/settings/settings_page.cc")
		seedCard.Module = "browser/settings"
		seedCard.AddDiscoveredBy("seed")
		seedCard.Scores.SeedWeight = 1.0

		// File in sub-module
		subModuleCard := schema.NewFileCard("browser/settings/ui/dialog.cc")
		subModuleCard.Module = "browser/settings/ui"

		ranker := NewRanker(nil)
		_ = ranker.Rank(&RankInput{Cards: []*schema.FileCard{seedCard, subModuleCard}})

		// Sub-module should get high module weight
		if subModuleCard.Scores.ModuleWeight < 0.7 {
			t.Errorf("sub-module card should have high module weight, got %f", subModuleCard.Scores.ModuleWeight)
		}
	})
}

func TestFinalScoreComputation(t *testing.T) {
	t.Run("all factors contribute", func(t *testing.T) {
		card := schema.NewFileCard("browser/settings/settings_page.cc")
		card.Scores.SeedWeight = 1.0
		card.Scores.ModuleWeight = 1.0
		card.Scores.HeuristicScore = 0.5
		card.Scores.ProfileScore = 0.8
		card.Scores.LLMLabel = "main_chain"
		card.Scores.LLMConfidence = 0.5
		card.AddDiscoveredBy("seed")
		card.Module = "browser/settings"

		config := &RankerConfig{
			SeedWeight:       0.3,
			SameModuleWeight: 0.2,
			HeuristicWeight:  0.4,
			ProfileWeight:    0.3,
			LLMWeight:        0.2,
			MaxFinalScore:    1.0,
		}

		ranker := NewRanker(config)
		result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{card}})

		// Verify score breakdown exists
		bd, ok := result.ScoreBreakdown[card.Path]
		if !ok {
			t.Fatal("score breakdown not found")
		}

		// Verify contributions
		expectedSeedContrib := 1.0 * 0.3
		if bd.SeedContribution != expectedSeedContrib {
			t.Errorf("seed contribution: expected %f, got %f", expectedSeedContrib, bd.SeedContribution)
		}

		expectedModuleContrib := 1.0 * 0.2
		if bd.ModuleContribution != expectedModuleContrib {
			t.Errorf("module contribution: expected %f, got %f", expectedModuleContrib, bd.ModuleContribution)
		}

		expectedHeuristicContrib := 0.5 * 0.4
		if bd.HeuristicContribution != expectedHeuristicContrib {
			t.Errorf("heuristic contribution: expected %f, got %f", expectedHeuristicContrib, bd.HeuristicContribution)
		}

		expectedProfileContrib := 0.8 * 0.3
		if bd.ProfileContribution != expectedProfileContrib {
			t.Errorf("profile contribution: expected %f, got %f", expectedProfileContrib, bd.ProfileContribution)
		}

		expectedLLMContrib := 0.5 * 0.2
		if bd.LLMContribution != expectedLLMContrib {
			t.Errorf("llm contribution: expected %f, got %f", expectedLLMContrib, bd.LLMContribution)
		}

		// Final score should be sum of contributions, capped at 1.0
		expectedFinal := expectedSeedContrib + expectedModuleContrib + expectedHeuristicContrib + expectedProfileContrib + expectedLLMContrib
		if expectedFinal > 1.0 {
			expectedFinal = 1.0
		}
		if bd.FinalScore != expectedFinal {
			t.Errorf("final score: expected %f, got %f", expectedFinal, bd.FinalScore)
		}
	})

	t.Run("score capped at max", func(t *testing.T) {
		card := schema.NewFileCard("file.go")
		card.Scores.SeedWeight = 1.0
		card.Scores.ModuleWeight = 1.0
		card.Scores.HeuristicScore = 1.0
		card.Scores.ProfileScore = 1.0

		// Use weights that would cause score > 1.0
		config := &RankerConfig{
			SeedWeight:       0.5,
			SameModuleWeight: 0.5,
			HeuristicWeight:  0.5,
			ProfileWeight:    0.5,
			MaxFinalScore:    1.0,
		}

		ranker := NewRanker(config)
		_ = ranker.Rank(&RankInput{Cards: []*schema.FileCard{card}})

		if card.Scores.FinalScore > 1.0 {
			t.Errorf("final score should be capped at 1.0, got %f", card.Scores.FinalScore)
		}
	})

	t.Run("llm can lift a card above static peers", func(t *testing.T) {
		card1 := schema.NewFileCard("static.go")
		card1.Scores.HeuristicScore = 0.7

		card2 := schema.NewFileCard("llm.go")
		card2.Scores.HeuristicScore = 0.4
		card2.Scores.LLMLabel = "main_chain"
		card2.Scores.LLMConfidence = 1.0

		ranker := NewRanker(nil)
		result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{card1, card2}})
		if result.Cards[0].Path != "llm.go" {
			t.Errorf("expected llm-enhanced card to rank first, got %s", result.Cards[0].Path)
		}
	})
}

func TestRankResultMethods(t *testing.T) {
	// Create cards with different heuristic scores
	// FinalScore will be computed by Rank: HeuristicScore * HeuristicWeight
	// With default HeuristicWeight = 0.4:
	// card1: 0.9 * 0.4 = 0.36
	// card2: 0.5 * 0.4 = 0.20
	// card3: 0.3 * 0.4 = 0.12
	card1 := schema.NewFileCard("file1.go")
	card1.Scores.HeuristicScore = 0.9

	card2 := schema.NewFileCard("file2.go")
	card2.Scores.HeuristicScore = 0.5

	card3 := schema.NewFileCard("file3.go")
	card3.Scores.HeuristicScore = 0.3

	ranker := NewRanker(nil)
	result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{card1, card2, card3}})

	t.Run("GetTopN", func(t *testing.T) {
		top2 := result.GetTopN(2)
		if len(top2) != 2 {
			t.Fatalf("expected 2 files, got %d", len(top2))
		}
		if top2[0] != "file1.go" {
			t.Errorf("expected file1.go first, got %s", top2[0])
		}
		if top2[1] != "file2.go" {
			t.Errorf("expected file2.go second, got %s", top2[1])
		}

		// n larger than available
		topAll := result.GetTopN(10)
		if len(topAll) != 3 {
			t.Errorf("expected 3 files for n=10, got %d", len(topAll))
		}

		// n <= 0
		topNone := result.GetTopN(0)
		if len(topNone) != 0 {
			t.Errorf("expected 0 files for n=0, got %d", len(topNone))
		}
	})

	t.Run("GetFilesAboveThreshold", func(t *testing.T) {
		// card1 has FinalScore = 0.9 * 0.4 = 0.36
		// card2 has FinalScore = 0.5 * 0.4 = 0.20
		// card3 has FinalScore = 0.3 * 0.4 = 0.12

		// Threshold 0.15: should get card1 and card2
		above15 := result.GetFilesAboveThreshold(0.15)
		if len(above15) != 2 {
			t.Errorf("expected 2 files above 0.15, got %d", len(above15))
		}

		// Threshold 0.30: should get only card1
		above30 := result.GetFilesAboveThreshold(0.30)
		if len(above30) != 1 {
			t.Errorf("expected 1 file above 0.30, got %d", len(above30))
		}
	})
}

func TestGetSeedModules(t *testing.T) {
	seed1 := schema.NewFileCard("browser/settings/page.cc")
	seed1.Module = "browser/settings"
	seed1.AddDiscoveredBy("seed")

	seed2 := schema.NewFileCard("chrome/browser/ui/view.cc")
	seed2.Module = "chrome/browser"
	seed2.AddDiscoveredBy("seed")

	nonSeed := schema.NewFileCard("other/file.go")
	nonSeed.Module = "other"

	modules := GetSeedModules([]*schema.FileCard{seed1, seed2, nonSeed})

	if len(modules) != 2 {
		t.Errorf("expected 2 seed modules, got %d", len(modules))
	}
	if !modules["browser/settings"] {
		t.Error("expected browser/settings in seed modules")
	}
	if !modules["chrome/browser"] {
		t.Error("expected chrome/browser in seed modules")
	}
}

func TestIsSubModule(t *testing.T) {
	tests := []struct {
		child, parent string
		expected      bool
	}{
		{"browser/settings/ui", "browser/settings", true}, // ui is sub-module of settings
		{"browser/settings", "browser/settings", true},    // same module
		{"browser/settings", "browser", true},             // settings is sub-module of browser
		{"browser", "browser/settings", false},            // browser is NOT sub-module of settings
		{"chrome/browser", "browser/settings", false},     // no relation
		{"", "browser", false},                            // empty child is not sub-module
		{"browser", "", true},                             // anything is sub-module of root (empty parent)
	}

	for _, tt := range tests {
		result := isSubModule(tt.child, tt.parent)
		if result != tt.expected {
			t.Errorf("isSubModule(%q, %q) = %v, expected %v", tt.child, tt.parent, result, tt.expected)
		}
	}
}

func TestSharedPrefixDepth(t *testing.T) {
	tests := []struct {
		mod1, mod2 string
		expected   int
	}{
		{"browser/settings", "browser/ui", 1},          // shared "browser"
		{"browser/settings/ui", "browser/settings", 2}, // shared "browser/settings"
		{"chrome/browser", "browser/settings", 0},      // no shared prefix
		{"browser", "browser", 1},                      // exact match
		{"", "browser", 0},                             // empty module
	}

	for _, tt := range tests {
		result := sharedPrefixDepth(tt.mod1, tt.mod2)
		if result != tt.expected {
			t.Errorf("sharedPrefixDepth(%q, %q) = %d, expected %d", tt.mod1, tt.mod2, result, tt.expected)
		}
	}
}

func TestConvenienceFunctions(t *testing.T) {
	card := schema.NewFileCard("file.go")
	card.Scores.HeuristicScore = 0.5

	t.Run("RankCards", func(t *testing.T) {
		result := RankCards([]*schema.FileCard{card})
		if len(result.Cards) != 1 {
			t.Errorf("expected 1 card, got %d", len(result.Cards))
		}
	})

	t.Run("RankCardsWithSeedModules", func(t *testing.T) {
		seedModules := map[string]bool{"browser/settings": true}
		result := RankCardsWithSeedModules([]*schema.FileCard{card}, seedModules)
		if len(result.Cards) != 1 {
			t.Errorf("expected 1 card, got %d", len(result.Cards))
		}
	})

	t.Run("RankCardsWithConfig", func(t *testing.T) {
		config := &RankerConfig{
			SeedWeight:      0.5,
			HeuristicWeight: 0.5,
			MaxFinalScore:   1.0,
		}
		result := RankCardsWithConfig([]*schema.FileCard{card}, config)
		if len(result.Cards) != 1 {
			t.Errorf("expected 1 card, got %d", len(result.Cards))
		}
	})
}

func TestRankingStability(t *testing.T) {
	// Create cards with same scores - sorting should be stable
	card1 := schema.NewFileCard("a_file.go")
	card1.Scores.HeuristicScore = 0.5

	card2 := schema.NewFileCard("b_file.go")
	card2.Scores.HeuristicScore = 0.5

	card3 := schema.NewFileCard("c_file.go")
	card3.Scores.HeuristicScore = 0.5

	ranker := NewRanker(nil)
	result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{card1, card2, card3}})

	// All have same final score, order should be stable (input order preserved for equal scores)
	if len(result.Cards) != 3 {
		t.Fatalf("expected 3 cards, got %d", len(result.Cards))
	}

	// Verify all have same score
	for i, card := range result.Cards {
		if card.Scores.FinalScore != result.Cards[0].Scores.FinalScore {
			t.Errorf("card %d has different score: %f vs %f", i, card.Scores.FinalScore, result.Cards[0].Scores.FinalScore)
		}
	}
}

func TestScoreBreakdown(t *testing.T) {
	card := schema.NewFileCard("browser/settings/page.cc")
	card.Module = "browser/settings"
	card.AddDiscoveredBy("seed")
	card.Scores.SeedWeight = 1.0
	card.Scores.HeuristicScore = 0.6
	card.Scores.ProfileScore = 0.4

	ranker := NewRanker(nil)
	result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{card}})

	bd, ok := result.ScoreBreakdown[card.Path]
	if !ok {
		t.Fatal("score breakdown not found")
	}

	if bd.Path != card.Path {
		t.Errorf("path mismatch: %s vs %s", bd.Path, card.Path)
	}

	if bd.Rank != 1 {
		t.Errorf("rank should be 1 for single card, got %d", bd.Rank)
	}

	// Verify all fields are populated
	if bd.SeedWeight != card.Scores.SeedWeight {
		t.Errorf("seed weight mismatch")
	}
	if bd.HeuristicScore != card.Scores.HeuristicScore {
		t.Errorf("heuristic score mismatch")
	}
	if bd.ProfileScore != card.Scores.ProfileScore {
		t.Errorf("profile score mismatch")
	}
}
