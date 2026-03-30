// Package ranking provides ranking algorithms for file candidates.
package ranking

import (
	"math"
	"testing"

	"github.com/no22/repo-scout/internal/schema"
)

func TestDefaultRankerConfig(t *testing.T) {
	config := DefaultRankerConfig()
	if config.DiscoveryWeight <= 0 || config.DiscoveryWeight > 1 {
		t.Errorf("DiscoveryWeight should be in (0, 1], got %f", config.DiscoveryWeight)
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
			DiscoveryWeight:  0.5,
			SameModuleWeight: 0.3,
			HeuristicWeight:  0.2,
			ProfileWeight:    0.1,
			LLMWeight:        0.65,
			MaxFinalScore:    0.9,
		}
		ranker := NewRanker(config)
		if ranker.config.DiscoveryWeight != 0.5 {
			t.Errorf("expected DiscoveryWeight 0.5, got %f", ranker.config.DiscoveryWeight)
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
		card1.Scores.DiscoveryScore = 0.0
		card1.Scores.HeuristicScore = 0.8

		// Seed file with lower heuristic score but high discovery score
		card2 := schema.NewFileCard("file2.go")
		card2.Scores.DiscoveryScore = 1.0
		card2.AddDiscoveredBy("seed")
		card2.Scores.HeuristicScore = 0.1

		ranker := NewRanker(nil)
		result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{card1, card2}})

		// Seed file should be ranked higher due to discovery score bonus
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
		card.Scores.DiscoveryScore = 1.0 // seed
		card.Scores.ModuleWeight = 1.0
		card.Scores.HeuristicScore = 0.5
		card.Scores.ProfileScore = 0.8
		card.Scores.LLMLabel = "main_chain"
		card.Scores.LLMConfidence = 0.5
		card.AddDiscoveredBy("seed")
		card.Module = "browser/settings"

		config := &RankerConfig{
			DiscoveryWeight:  0.35,
			SameModuleWeight: 0.15,
			HeuristicWeight:  0.20,
			ProfileWeight:    0.10,
			LLMWeight:        0.65,
			MaxFinalScore:    1.0,
		}

		ranker := NewRanker(config)
		result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{card}})

		bd, ok := result.ScoreBreakdown[card.Path]
		if !ok {
			t.Fatal("score breakdown not found")
		}

		const eps = 1e-9
		// Verify structural contributions
		expectedDiscoveryContrib := 1.0 * 0.35
		if math.Abs(bd.DiscoveryContribution-expectedDiscoveryContrib) > eps {
			t.Errorf("discovery contribution: expected %f, got %f", expectedDiscoveryContrib, bd.DiscoveryContribution)
		}

		expectedModuleContrib := 1.0 * 0.15
		if math.Abs(bd.ModuleContribution-expectedModuleContrib) > eps {
			t.Errorf("module contribution: expected %f, got %f", expectedModuleContrib, bd.ModuleContribution)
		}

		expectedHeuristicContrib := 0.5 * 0.20
		if math.Abs(bd.HeuristicContribution-expectedHeuristicContrib) > eps {
			t.Errorf("heuristic contribution: expected %f, got %f", expectedHeuristicContrib, bd.HeuristicContribution)
		}

		expectedProfileContrib := 0.8 * 0.10
		if math.Abs(bd.ProfileContribution-expectedProfileContrib) > eps {
			t.Errorf("profile contribution: expected %f, got %f", expectedProfileContrib, bd.ProfileContribution)
		}

		// LLM score = main_chain(1.0) * confidence(0.5) = 0.5
		// final = structural*(1-0.65) + llm*0.65
		llmScore := 1.0 * 0.5
		structural := expectedDiscoveryContrib + expectedModuleContrib + expectedHeuristicContrib + expectedProfileContrib
		expectedFinal := structural*(1-0.65) + llmScore*0.65
		if expectedFinal > 1.0 {
			expectedFinal = 1.0
		}
		if math.Abs(bd.FinalScore-expectedFinal) > eps {
			t.Errorf("final score: expected %f, got %f", expectedFinal, bd.FinalScore)
		}
	})

	t.Run("score capped at max", func(t *testing.T) {
		card := schema.NewFileCard("file.go")
		card.Scores.DiscoveryScore = 1.0
		card.Scores.ModuleWeight = 1.0
		card.Scores.HeuristicScore = 1.0
		card.Scores.ProfileScore = 1.0

		config := &RankerConfig{
			DiscoveryWeight:  0.5,
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

	t.Run("llm irrelevant can demote a noisy static match", func(t *testing.T) {
		card1 := schema.NewFileCard("noisy-static.go")
		card1.Scores.HeuristicScore = 0.8
		card1.Scores.LLMLabel = "irrelevant"
		card1.Scores.LLMConfidence = 1.0

		card2 := schema.NewFileCard("clean-static.go")
		card2.Scores.HeuristicScore = 0.5

		ranker := NewRanker(nil)
		result := ranker.Rank(&RankInput{Cards: []*schema.FileCard{card1, card2}})
		if result.Cards[0].Path != "clean-static.go" {
			t.Errorf("expected irrelevant judgment to demote noisy-static.go, got %s first", result.Cards[0].Path)
		}
		if !(card1.Scores.FinalScore < card2.Scores.FinalScore) {
			t.Errorf("expected irrelevant card score (%f) to be below clean card (%f)", card1.Scores.FinalScore, card2.Scores.FinalScore)
		}
	})
}

func TestRankResultMethods(t *testing.T) {
	// FinalScore (no LLM, no discovery) = HeuristicScore * HeuristicWeight
	// With default HeuristicWeight = 0.20:
	// card1: 0.9 * 0.20 = 0.18
	// card2: 0.5 * 0.20 = 0.10
	// card3: 0.3 * 0.20 = 0.06
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
		// card1: 0.9 * 0.20 = 0.18
		// card2: 0.5 * 0.20 = 0.10
		// card3: 0.3 * 0.20 = 0.06

		// Threshold 0.08: should get card1 and card2
		above08 := result.GetFilesAboveThreshold(0.08)
		if len(above08) != 2 {
			t.Errorf("expected 2 files above 0.08, got %d", len(above08))
		}

		// Threshold 0.15: should get only card1
		above15 := result.GetFilesAboveThreshold(0.15)
		if len(above15) != 1 {
			t.Errorf("expected 1 file above 0.15, got %d", len(above15))
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
			DiscoveryWeight: 0.5,
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
	if bd.DiscoveryScore != card.Scores.DiscoveryScore {
		t.Errorf("discovery score mismatch")
	}
	if bd.HeuristicScore != card.Scores.HeuristicScore {
		t.Errorf("heuristic score mismatch")
	}
	if bd.ProfileScore != card.Scores.ProfileScore {
		t.Errorf("profile score mismatch")
	}
}
