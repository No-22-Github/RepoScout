// Package output provides rendering functionality for ContextPack.
package output

import (
	"fmt"
	"strings"

	"github.com/no22/repo-scout/internal/schema"
)

// MarkdownRenderer renders a ContextPack as Markdown text.
type MarkdownRenderer struct {
	// IncludeStats controls whether to include statistics section.
	IncludeStats bool

	// IncludeUncertain controls whether to include uncertain files section.
	IncludeUncertain bool

	// MaxMainChainDisplay limits the number of main chain files to display.
	// 0 means no limit.
	MaxMainChainDisplay int

	// MaxCompanionDisplay limits the number of companion files to display.
	// 0 means no limit.
	MaxCompanionDisplay int

	// MaxReadingOrderDisplay limits the number of reading order items to display.
	// 0 means no limit.
	MaxReadingOrderDisplay int
}

// NewMarkdownRenderer creates a new MarkdownRenderer with default settings.
func NewMarkdownRenderer() *MarkdownRenderer {
	return &MarkdownRenderer{
		IncludeStats:           true,
		IncludeUncertain:       true,
		MaxMainChainDisplay:    0,
		MaxCompanionDisplay:    0,
		MaxReadingOrderDisplay: 0,
	}
}

// Render renders a ContextPack as Markdown text.
func (r *MarkdownRenderer) Render(pack *schema.ContextPack) string {
	if pack == nil {
		return ""
	}

	var sb strings.Builder

	// Title and task summary
	sb.WriteString(r.renderHeader(pack))

	// Main chain files (recommended first reads)
	sb.WriteString(r.renderMainChain(pack))

	// High-priority companion files
	sb.WriteString(r.renderCompanionFiles(pack))

	// Uncertain files
	if r.IncludeUncertain && len(pack.UncertainNodes) > 0 {
		sb.WriteString(r.renderUncertainFiles(pack))
	}

	// Reading order
	sb.WriteString(r.renderReadingOrder(pack))

	// Risk hints
	sb.WriteString(r.renderRiskHints(pack))

	// Statistics
	if r.IncludeStats && pack.Stats != nil {
		sb.WriteString(r.renderStats(pack))
	}

	return sb.String()
}

// renderHeader renders the title and task summary section.
func (r *MarkdownRenderer) renderHeader(pack *schema.ContextPack) string {
	var sb strings.Builder

	sb.WriteString("# RepoScout ContextPack\n\n")

	// Task summary
	sb.WriteString("## Task Summary\n\n")
	sb.WriteString(pack.Task)
	sb.WriteString("\n\n")

	// Repo family if present
	if pack.RepoFamily != "" {
		sb.WriteString("**Repo Family:** ")
		sb.WriteString(pack.RepoFamily)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// renderMainChain renders the main chain files section.
func (r *MarkdownRenderer) renderMainChain(pack *schema.ContextPack) string {
	if len(pack.MainChain) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("## Recommended First Reads\n\n")
	sb.WriteString("These files form the **main execution path** relevant to your task. ")
	sb.WriteString("Start here to understand the core logic:\n\n")

	files := pack.MainChain
	if r.MaxMainChainDisplay > 0 && len(files) > r.MaxMainChainDisplay {
		files = files[:r.MaxMainChainDisplay]
	}

	for i, file := range files {
		sb.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, file))
	}

	if r.MaxMainChainDisplay > 0 && len(pack.MainChain) > r.MaxMainChainDisplay {
		sb.WriteString(fmt.Sprintf("\n*...and %d more files*\n", len(pack.MainChain)-r.MaxMainChainDisplay))
	}

	sb.WriteString("\n")
	return sb.String()
}

// renderCompanionFiles renders the companion files section.
func (r *MarkdownRenderer) renderCompanionFiles(pack *schema.ContextPack) string {
	if len(pack.CompanionFiles) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("## High-Priority Companion Files\n\n")
	sb.WriteString("These files provide **supporting context** such as configurations, types, and helpers:\n\n")

	files := pack.CompanionFiles
	if r.MaxCompanionDisplay > 0 && len(files) > r.MaxCompanionDisplay {
		files = files[:r.MaxCompanionDisplay]
	}

	for i, file := range files {
		sb.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, file))
	}

	if r.MaxCompanionDisplay > 0 && len(pack.CompanionFiles) > r.MaxCompanionDisplay {
		sb.WriteString(fmt.Sprintf("\n*...and %d more files*\n", len(pack.CompanionFiles)-r.MaxCompanionDisplay))
	}

	sb.WriteString("\n")
	return sb.String()
}

// renderUncertainFiles renders the uncertain files section.
func (r *MarkdownRenderer) renderUncertainFiles(pack *schema.ContextPack) string {
	if len(pack.UncertainNodes) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("## Uncertain Points\n\n")
	sb.WriteString("These files **might be relevant** but require manual review:\n\n")

	for _, file := range pack.UncertainNodes {
		sb.WriteString(fmt.Sprintf("- `%s`\n", file))
	}

	sb.WriteString("\n")
	return sb.String()
}

// renderReadingOrder renders the recommended reading order section.
func (r *MarkdownRenderer) renderReadingOrder(pack *schema.ContextPack) string {
	if len(pack.ReadingOrder) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("## Recommended Reading Order\n\n")
	sb.WriteString("For best results, read files in this order:\n\n")

	files := pack.ReadingOrder
	if r.MaxReadingOrderDisplay > 0 && len(files) > r.MaxReadingOrderDisplay {
		files = files[:r.MaxReadingOrderDisplay]
	}

	for i, file := range files {
		sb.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, file))
	}

	if r.MaxReadingOrderDisplay > 0 && len(pack.ReadingOrder) > r.MaxReadingOrderDisplay {
		sb.WriteString(fmt.Sprintf("\n*...and %d more files*\n", len(pack.ReadingOrder)-r.MaxReadingOrderDisplay))
	}

	sb.WriteString("\n")
	return sb.String()
}

// renderRiskHints renders the risk hints section.
func (r *MarkdownRenderer) renderRiskHints(pack *schema.ContextPack) string {
	if len(pack.RiskHints) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("## Risk Hints\n\n")

	for _, hint := range pack.RiskHints {
		sb.WriteString(r.renderRiskHint(hint))
	}

	sb.WriteString("\n")
	return sb.String()
}

// renderRiskHint renders a single risk hint.
func (r *MarkdownRenderer) renderRiskHint(hint *schema.RiskHint) string {
	var sb strings.Builder

	// Use emoji prefix based on level
	emoji := r.getLevelEmoji(hint.Level)

	// Format: <emoji> **[LEVEL] category:** message
	sb.WriteString(fmt.Sprintf("%s **[%s] %s:** %s\n",
		emoji,
		strings.ToUpper(hint.Level),
		hint.Category,
		hint.Message,
	))

	// List affected files if any
	if len(hint.AffectedFiles) > 0 {
		sb.WriteString("  - Affected files:\n")
		for _, f := range hint.AffectedFiles {
			sb.WriteString(fmt.Sprintf("    - `%s`\n", f))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// getLevelEmoji returns an emoji for the given risk level.
func (r *MarkdownRenderer) getLevelEmoji(level string) string {
	switch strings.ToLower(level) {
	case "error":
		return "\u274c" // X mark
	case "warning":
		return "\u26a0\ufe0f" // Warning sign
	case "info":
		return "\u2139\ufe0f" // Information source
	default:
		return "\u2022" // Bullet point
	}
}

// renderStats renders the statistics section.
func (r *MarkdownRenderer) renderStats(pack *schema.ContextPack) string {
	if pack.Stats == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("## Statistics\n\n")
	sb.WriteString(fmt.Sprintf("- **Total files analyzed:** %d\n", pack.Stats.TotalFiles))
	sb.WriteString(fmt.Sprintf("- **Main chain files:** %d\n", pack.Stats.MainChainCount))
	sb.WriteString(fmt.Sprintf("- **Companion files:** %d\n", pack.Stats.CompanionCount))
	sb.WriteString(fmt.Sprintf("- **Uncertain files:** %d\n", pack.Stats.UncertainCount))

	if pack.Stats.AnalysisTimeMs > 0 {
		sb.WriteString(fmt.Sprintf("- **Analysis time:** %dms\n", pack.Stats.AnalysisTimeMs))
	}

	if pack.Stats.ModelEnhanced {
		sb.WriteString("- **Model enhanced:** Yes\n")
	} else {
		sb.WriteString("- **Model enhanced:** No (static analysis only)\n")
	}

	sb.WriteString("\n")
	return sb.String()
}

// RenderMarkdown is a convenience function that renders a ContextPack using default settings.
func RenderMarkdown(pack *schema.ContextPack) string {
	return NewMarkdownRenderer().Render(pack)
}
