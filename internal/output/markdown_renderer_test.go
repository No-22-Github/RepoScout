package output

import (
	"strings"
	"testing"

	"github.com/no22/repo-scout/internal/schema"
)

func TestMarkdownRenderer_Render(t *testing.T) {
	tests := []struct {
		name     string
		pack     *schema.ContextPack
		opts     func(*MarkdownRenderer)
		contains []string
		missing  []string
	}{
		{
			name: "basic context pack",
			pack: &schema.ContextPack{
				Task:           "Find the settings page implementation",
				RepoFamily:     "browser-settings",
				MainChain:      []string{"browser/settings/page.ts", "browser/settings/handler.ts"},
				CompanionFiles: []string{"browser/settings/types.ts", "browser/settings/config.json"},
				UncertainNodes: []string{"browser/settings/legacy.ts"},
				ReadingOrder:   []string{"browser/settings/page.ts", "browser/settings/handler.ts", "browser/settings/types.ts"},
				RiskHints: []*schema.RiskHint{
					{Level: "warning", Category: "test-coverage", Message: "No test files found for main chain"},
				},
				Stats: &schema.PackStats{
					TotalFiles:     5,
					MainChainCount: 2,
					CompanionCount: 2,
					UncertainCount: 1,
					AnalysisTimeMs: 42,
					ModelEnhanced:  false,
				},
			},
			contains: []string{
				"# RepoScout ContextPack",
				"## Task Summary",
				"Find the settings page implementation",
				"**Repo Family:** browser-settings",
				"## Recommended First Reads",
				"browser/settings/page.ts",
				"browser/settings/handler.ts",
				"## High-Priority Companion Files",
				"browser/settings/types.ts",
				"browser/settings/config.json",
				"## Uncertain Points",
				"browser/settings/legacy.ts",
				"## Recommended Reading Order",
				"## Risk Hints",
				"[WARNING] test-coverage",
				"No test files found for main chain",
				"## Statistics",
				"**Total files analyzed:** 5",
				"**Main chain files:** 2",
				"**Companion files:** 2",
				"**Uncertain files:** 1",
			},
		},
		{
			name: "minimal context pack",
			pack: &schema.ContextPack{
				Task:      "Simple task",
				MainChain: []string{"main.go"},
			},
			contains: []string{
				"## Task Summary",
				"Simple task",
				"## Recommended First Reads",
				"main.go",
			},
			missing: []string{
				"## High-Priority Companion Files",
				"## Uncertain Points",
				"## Risk Hints",
			},
		},
		{
			name:     "nil context pack",
			pack:     nil,
			contains: []string{},
		},
		{
			name: "exclude uncertain section",
			pack: &schema.ContextPack{
				Task:           "Task",
				MainChain:      []string{"main.go"},
				UncertainNodes: []string{"uncertain.go"},
			},
			opts: func(r *MarkdownRenderer) {
				r.IncludeUncertain = false
			},
			missing: []string{
				"## Uncertain Points",
				"uncertain.go",
			},
		},
		{
			name: "exclude stats section",
			pack: &schema.ContextPack{
				Task: "Task",
				Stats: &schema.PackStats{
					TotalFiles: 10,
				},
			},
			opts: func(r *MarkdownRenderer) {
				r.IncludeStats = false
			},
			missing: []string{
				"## Statistics",
				"Total files analyzed: 10",
			},
		},
		{
			name: "limit main chain display",
			pack: &schema.ContextPack{
				Task:      "Task",
				MainChain: []string{"a.go", "b.go", "c.go", "d.go", "e.go"},
			},
			opts: func(r *MarkdownRenderer) {
				r.MaxMainChainDisplay = 2
			},
			contains: []string{
				"a.go",
				"b.go",
				"...and 3 more files",
			},
			missing: []string{
				"c.go",
				"d.go",
				"e.go",
			},
		},
		{
			name: "risk hints with affected files",
			pack: &schema.ContextPack{
				Task: "Task",
				RiskHints: []*schema.RiskHint{
					{
						Level:         "error",
						Category:      "security",
						Message:       "Potential security issue",
						AffectedFiles: []string{"auth.go", "session.go"},
					},
				},
			},
			contains: []string{
				"[ERROR] security",
				"Potential security issue",
				"Affected files:",
				"auth.go",
				"session.go",
			},
		},
		{
			name: "model enhanced stats",
			pack: &schema.ContextPack{
				Task: "Task",
				Stats: &schema.PackStats{
					ModelEnhanced: true,
				},
			},
			contains: []string{
				"**Model enhanced:** Yes",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewMarkdownRenderer()
			if tt.opts != nil {
				tt.opts(renderer)
			}

			result := renderer.Render(tt.pack)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", s, result)
				}
			}

			for _, s := range tt.missing {
				if strings.Contains(result, s) {
					t.Errorf("expected output to NOT contain %q, but it did.\nOutput:\n%s", s, result)
				}
			}
		})
	}
}

func TestMarkdownRenderer_RenderHeader(t *testing.T) {
	renderer := NewMarkdownRenderer()

	tests := []struct {
		name     string
		pack     *schema.ContextPack
		contains []string
	}{
		{
			name: "with repo family",
			pack: &schema.ContextPack{
				Task:       "Test task",
				RepoFamily: "go-web",
			},
			contains: []string{
				"# RepoScout ContextPack",
				"## Task Summary",
				"Test task",
				"**Repo Family:** go-web",
			},
		},
		{
			name: "without repo family",
			pack: &schema.ContextPack{
				Task: "Test task",
			},
			contains: []string{
				"# RepoScout ContextPack",
				"## Task Summary",
				"Test task",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderer.renderHeader(tt.pack)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("expected header to contain %q", s)
				}
			}
		})
	}
}

func TestMarkdownRenderer_RenderRiskHint(t *testing.T) {
	renderer := NewMarkdownRenderer()

	tests := []struct {
		name     string
		hint     *schema.RiskHint
		contains []string
	}{
		{
			name: "error level",
			hint: &schema.RiskHint{
				Level:    "error",
				Category: "security",
				Message:  "Critical issue",
			},
			contains: []string{
				"[ERROR] security",
				"Critical issue",
			},
		},
		{
			name: "warning level",
			hint: &schema.RiskHint{
				Level:    "warning",
				Category: "test-coverage",
				Message:  "No tests",
			},
			contains: []string{
				"[WARNING] test-coverage",
				"No tests",
			},
		},
		{
			name: "info level",
			hint: &schema.RiskHint{
				Level:    "info",
				Category: "complexity",
				Message:  "Large file detected",
			},
			contains: []string{
				"[INFO] complexity",
				"Large file detected",
			},
		},
		{
			name: "unknown level",
			hint: &schema.RiskHint{
				Level:    "unknown",
				Category: "misc",
				Message:  "Something",
			},
			contains: []string{
				"[UNKNOWN] misc",
				"Something",
			},
		},
		{
			name: "with affected files",
			hint: &schema.RiskHint{
				Level:         "warning",
				Category:      "deps",
				Message:       "Outdated dependency",
				AffectedFiles: []string{"go.mod", "go.sum"},
			},
			contains: []string{
				"Affected files:",
				"go.mod",
				"go.sum",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderer.renderRiskHint(tt.hint)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("expected risk hint to contain %q, got:\n%s", s, result)
				}
			}
		})
	}
}

func TestMarkdownRenderer_GetLevelEmoji(t *testing.T) {
	renderer := NewMarkdownRenderer()

	tests := []struct {
		level    string
		expected string
	}{
		{"error", "\u274c"},
		{"ERROR", "\u274c"},
		{"warning", "\u26a0\ufe0f"},
		{"WARNING", "\u26a0\ufe0f"},
		{"info", "\u2139\ufe0f"},
		{"INFO", "\u2139\ufe0f"},
		{"unknown", "\u2022"},
		{"", "\u2022"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			result := renderer.getLevelEmoji(tt.level)
			if result != tt.expected {
				t.Errorf("getLevelEmoji(%q) = %q, want %q", tt.level, result, tt.expected)
			}
		})
	}
}

func TestRenderMarkdown(t *testing.T) {
	pack := &schema.ContextPack{
		Task:      "Test task",
		MainChain: []string{"main.go"},
	}

	result := RenderMarkdown(pack)

	if result == "" {
		t.Error("RenderMarkdown returned empty string")
	}

	if !strings.Contains(result, "Test task") {
		t.Error("RenderMarkdown missing task")
	}

	if !strings.Contains(result, "main.go") {
		t.Error("RenderMarkdown missing main.go")
	}
}
