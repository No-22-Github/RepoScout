package schema

import (
	"encoding/json"
	"testing"
)

func TestNewContextPack(t *testing.T) {
	cp := NewContextPack("Add authentication to the API")

	if cp.Task != "Add authentication to the API" {
		t.Errorf("Task = %v, want 'Add authentication to the API'", cp.Task)
	}
	if cp.MainChain == nil {
		t.Error("MainChain should not be nil")
	}
	if cp.CompanionFiles == nil {
		t.Error("CompanionFiles should not be nil")
	}
	if cp.UncertainNodes == nil {
		t.Error("UncertainNodes should not be nil")
	}
	if cp.ReadingOrder == nil {
		t.Error("ReadingOrder should not be nil")
	}
	if cp.RiskHints == nil {
		t.Error("RiskHints should not be nil")
	}
	if cp.Stats == nil {
		t.Error("Stats should not be nil")
	}
}

func TestContextPack_AddMainChain(t *testing.T) {
	cp := NewContextPack("test task")

	cp.AddMainChain("internal/auth/handler.go")
	if len(cp.MainChain) != 1 || cp.MainChain[0] != "internal/auth/handler.go" {
		t.Errorf("MainChain = %v, want [internal/auth/handler.go]", cp.MainChain)
	}

	// Adding duplicate should not increase length
	cp.AddMainChain("internal/auth/handler.go")
	if len(cp.MainChain) != 1 {
		t.Errorf("MainChain = %v, should still have 1 element", cp.MainChain)
	}

	// Adding different file
	cp.AddMainChain("internal/auth/middleware.go")
	if len(cp.MainChain) != 2 {
		t.Errorf("MainChain = %v, should have 2 elements", cp.MainChain)
	}
}

func TestContextPack_AddCompanion(t *testing.T) {
	cp := NewContextPack("test task")

	cp.AddCompanion("internal/auth/types.go")
	if len(cp.CompanionFiles) != 1 || cp.CompanionFiles[0] != "internal/auth/types.go" {
		t.Errorf("CompanionFiles = %v, want [internal/auth/types.go]", cp.CompanionFiles)
	}

	// Adding duplicate should not increase length
	cp.AddCompanion("internal/auth/types.go")
	if len(cp.CompanionFiles) != 1 {
		t.Errorf("CompanionFiles = %v, should still have 1 element", cp.CompanionFiles)
	}

	// Adding different file
	cp.AddCompanion("internal/config/auth.go")
	if len(cp.CompanionFiles) != 2 {
		t.Errorf("CompanionFiles = %v, should have 2 elements", cp.CompanionFiles)
	}
}

func TestContextPack_AddUncertain(t *testing.T) {
	cp := NewContextPack("test task")

	cp.AddUncertain("internal/legacy/auth.go")
	if len(cp.UncertainNodes) != 1 || cp.UncertainNodes[0] != "internal/legacy/auth.go" {
		t.Errorf("UncertainNodes = %v, want [internal/legacy/auth.go]", cp.UncertainNodes)
	}

	// Adding duplicate should not increase length
	cp.AddUncertain("internal/legacy/auth.go")
	if len(cp.UncertainNodes) != 1 {
		t.Errorf("UncertainNodes = %v, should still have 1 element", cp.UncertainNodes)
	}

	// Adding different file
	cp.AddUncertain("internal/utils/auth.go")
	if len(cp.UncertainNodes) != 2 {
		t.Errorf("UncertainNodes = %v, should have 2 elements", cp.UncertainNodes)
	}
}

func TestContextPack_AddRiskHint(t *testing.T) {
	cp := NewContextPack("test task")

	cp.AddRiskHint("warning", "test-coverage", "No tests for auth handler", "internal/auth/handler.go")

	if len(cp.RiskHints) != 1 {
		t.Errorf("RiskHints length = %v, want 1", len(cp.RiskHints))
		return
	}

	hint := cp.RiskHints[0]
	if hint.Level != "warning" {
		t.Errorf("Level = %v, want warning", hint.Level)
	}
	if hint.Category != "test-coverage" {
		t.Errorf("Category = %v, want test-coverage", hint.Category)
	}
	if hint.Message != "No tests for auth handler" {
		t.Errorf("Message = %v, want 'No tests for auth handler'", hint.Message)
	}
	if len(hint.AffectedFiles) != 1 || hint.AffectedFiles[0] != "internal/auth/handler.go" {
		t.Errorf("AffectedFiles = %v, want [internal/auth/handler.go]", hint.AffectedFiles)
	}

	// Add another risk hint
	cp.AddRiskHint("error", "security", "Potential SQL injection", "internal/db/query.go", "internal/db/builder.go")
	if len(cp.RiskHints) != 2 {
		t.Errorf("RiskHints length = %v, want 2", len(cp.RiskHints))
	}
}

func TestContextPack_SetReadingOrder(t *testing.T) {
	cp := NewContextPack("test task")
	order := []string{"a.go", "b.go", "c.go"}

	cp.SetReadingOrder(order)

	if len(cp.ReadingOrder) != 3 {
		t.Errorf("ReadingOrder length = %v, want 3", len(cp.ReadingOrder))
		return
	}

	for i, p := range order {
		if cp.ReadingOrder[i] != p {
			t.Errorf("ReadingOrder[%d] = %v, want %v", i, cp.ReadingOrder[i], p)
		}
	}
}

func TestContextPack_UpdateStats(t *testing.T) {
	cp := NewContextPack("test task")
	cp.AddMainChain("a.go", "b.go")
	cp.AddCompanion("c.go", "d.go", "e.go")
	cp.AddUncertain("f.go")

	cp.UpdateStats()

	if cp.Stats.MainChainCount != 2 {
		t.Errorf("MainChainCount = %v, want 2", cp.Stats.MainChainCount)
	}
	if cp.Stats.CompanionCount != 3 {
		t.Errorf("CompanionCount = %v, want 3", cp.Stats.CompanionCount)
	}
	if cp.Stats.UncertainCount != 1 {
		t.Errorf("UncertainCount = %v, want 1", cp.Stats.UncertainCount)
	}
	if cp.Stats.TotalFiles != 6 {
		t.Errorf("TotalFiles = %v, want 6", cp.Stats.TotalFiles)
	}
}

func TestContextPack_ToJSON(t *testing.T) {
	cp := &ContextPack{
		Task:           "Add authentication",
		RepoFamily:     "go-web",
		MainChain:      []string{"internal/auth/handler.go"},
		CompanionFiles: []string{"internal/auth/types.go"},
		UncertainNodes: []string{"internal/legacy/auth.go"},
		ReadingOrder:   []string{"internal/auth/handler.go", "internal/auth/types.go"},
		RiskHints: []*RiskHint{
			{
				Level:         "warning",
				Category:      "test-coverage",
				Message:       "No tests",
				AffectedFiles: []string{"internal/auth/handler.go"},
			},
		},
		SummaryMarkdown: "# Analysis Summary\n\nKey files identified.",
		Stats: &PackStats{
			TotalFiles:     3,
			MainChainCount: 1,
			CompanionCount: 1,
			UncertainCount: 1,
			AnalysisTimeMs: 150,
			ModelEnhanced:  false,
		},
	}

	data, err := cp.ToJSON()
	if err != nil {
		t.Errorf("ToJSON() unexpected error: %v", err)
		return
	}

	// Verify we can parse it back
	var parsed ContextPack
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Failed to parse ToJSON output: %v", err)
		return
	}

	if parsed.Task != cp.Task {
		t.Errorf("Task mismatch: got %v, want %v", parsed.Task, cp.Task)
	}
	if parsed.RepoFamily != cp.RepoFamily {
		t.Errorf("RepoFamily mismatch: got %v, want %v", parsed.RepoFamily, cp.RepoFamily)
	}
	if len(parsed.MainChain) != len(cp.MainChain) {
		t.Errorf("MainChain length mismatch: got %v, want %v", len(parsed.MainChain), len(cp.MainChain))
	}
	if len(parsed.RiskHints) != len(cp.RiskHints) {
		t.Errorf("RiskHints length mismatch: got %v, want %v", len(parsed.RiskHints), len(cp.RiskHints))
	}
}

func TestContextPack_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		cp   *ContextPack
	}{
		{
			name: "minimal context pack",
			cp:   NewContextPack("simple task"),
		},
		{
			name: "full context pack",
			cp: &ContextPack{
				Task:           "Add authentication to the API",
				RepoFamily:     "go-web",
				MainChain:      []string{"internal/auth/handler.go", "internal/auth/middleware.go"},
				CompanionFiles: []string{"internal/auth/types.go", "internal/config/auth.go"},
				UncertainNodes: []string{"internal/legacy/auth.go"},
				ReadingOrder:   []string{"internal/auth/handler.go", "internal/auth/middleware.go", "internal/auth/types.go"},
				RiskHints: []*RiskHint{
					{
						Level:         "warning",
						Category:      "test-coverage",
						Message:       "No tests for auth handler",
						AffectedFiles: []string{"internal/auth/handler.go"},
					},
					{
						Level:         "info",
						Category:      "complexity",
						Message:       "High cyclomatic complexity",
						AffectedFiles: []string{"internal/auth/middleware.go"},
					},
				},
				SummaryMarkdown: "# Auth Analysis\n\nKey files: handler.go, middleware.go",
				Stats: &PackStats{
					TotalFiles:     5,
					MainChainCount: 2,
					CompanionCount: 2,
					UncertainCount: 1,
					AnalysisTimeMs: 250,
					ModelEnhanced:  true,
				},
			},
		},
		{
			name: "context pack with empty slices",
			cp: &ContextPack{
				Task:           "empty task",
				MainChain:      []string{},
				CompanionFiles: []string{},
				ReadingOrder:   []string{},
				Stats:          &PackStats{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.cp.ToJSON()
			if err != nil {
				t.Errorf("ToJSON() error: %v", err)
				return
			}

			var parsed ContextPack
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Errorf("Unmarshal() error: %v", err)
				return
			}

			if parsed.Task != tt.cp.Task {
				t.Errorf("Task mismatch: got %v, want %v", parsed.Task, tt.cp.Task)
			}
		})
	}
}

func TestContextPack_AllFiles(t *testing.T) {
	cp := NewContextPack("test task")
	cp.AddMainChain("a.go", "b.go")
	cp.AddCompanion("c.go")
	cp.AddUncertain("d.go", "e.go")

	files := cp.AllFiles()

	if len(files) != 5 {
		t.Errorf("AllFiles() length = %v, want 5", len(files))
		return
	}

	// Verify all files are present
	expected := map[string]bool{
		"a.go": false,
		"b.go": false,
		"c.go": false,
		"d.go": false,
		"e.go": false,
	}

	for _, f := range files {
		if _, ok := expected[f]; ok {
			expected[f] = true
		}
	}

	for f, found := range expected {
		if !found {
			t.Errorf("AllFiles() missing %v", f)
		}
	}
}

func TestRiskHint_Structure(t *testing.T) {
	// Test that RiskHint can be created and serialized correctly
	hint := &RiskHint{
		Level:         "error",
		Category:      "security",
		Message:       "Potential security vulnerability",
		AffectedFiles: []string{"a.go", "b.go"},
	}

	data, err := json.Marshal(hint)
	if err != nil {
		t.Errorf("Failed to marshal RiskHint: %v", err)
		return
	}

	var parsed RiskHint
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Failed to unmarshal RiskHint: %v", err)
		return
	}

	if parsed.Level != hint.Level {
		t.Errorf("Level = %v, want %v", parsed.Level, hint.Level)
	}
	if parsed.Category != hint.Category {
		t.Errorf("Category = %v, want %v", parsed.Category, hint.Category)
	}
	if parsed.Message != hint.Message {
		t.Errorf("Message = %v, want %v", parsed.Message, hint.Message)
	}
}

func TestPackStats_Structure(t *testing.T) {
	// Test that PackStats can be created and serialized correctly
	stats := &PackStats{
		TotalFiles:     10,
		MainChainCount: 3,
		CompanionCount: 5,
		UncertainCount: 2,
		AnalysisTimeMs: 500,
		ModelEnhanced:  true,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Errorf("Failed to marshal PackStats: %v", err)
		return
	}

	var parsed PackStats
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Failed to unmarshal PackStats: %v", err)
		return
	}

	if parsed.TotalFiles != stats.TotalFiles {
		t.Errorf("TotalFiles = %v, want %v", parsed.TotalFiles, stats.TotalFiles)
	}
	if parsed.ModelEnhanced != stats.ModelEnhanced {
		t.Errorf("ModelEnhanced = %v, want %v", parsed.ModelEnhanced, stats.ModelEnhanced)
	}
}
