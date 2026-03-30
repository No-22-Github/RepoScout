// Package runner orchestrates the full reconnaissance pipeline.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/no22/repo-scout/internal/config"
	"github.com/no22/repo-scout/internal/llm"
	"github.com/no22/repo-scout/internal/ranking"
	"github.com/no22/repo-scout/internal/schema"
)

type recordingProgress struct {
	messages []string
}

func (p *recordingProgress) Start(string) {}

func (p *recordingProgress) Startf(string, ...any) {}

func (p *recordingProgress) Done() {}

func (p *recordingProgress) DoneWithCount(int, string) {}

func (p *recordingProgress) Infof(format string, args ...any) {
	p.messages = append(p.messages, fmt.Sprintf(format, args...))
}

// createTestRepo creates a temporary repository structure for testing.
func createTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create directory structure
	dirs := []string{
		"cmd/server",
		"internal/auth",
		"internal/config",
		"pkg/utils",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create files
	files := map[string]string{
		"cmd/server/main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}`,
		"internal/auth/handler.go": `package auth

import (
	"errors"

	"test-repo/internal/config"
)

type Handler struct {
	Name string
}

func (h *Handler) Login(cfg config.Config) error {
	if cfg.Port == 0 {
		return errors.New("invalid config")
	}
	return nil
}`,
		"internal/auth/middleware.go": `package auth

func Middleware() {
	// Auth middleware
}`,
		"internal/config/config.go": `package config

type Config struct {
	Port int
}`,
		"pkg/utils/helper.go": `package utils

func Helper() string {
	return "helper"
}`,
		"README.md": "# Test Repo",
		"go.mod":    "module test-repo\n\ngo 1.23",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	return tmpDir
}

func TestRunner_Run(t *testing.T) {
	// Create test repo
	repoRoot := createTestRepo(t)

	// Create runner with default config
	cfg := config.DefaultConfig()
	r := NewRunner(cfg)

	tests := []struct {
		name    string
		req     *schema.ReconRequest
		wantErr bool
	}{
		{
			name: "basic request with seed files",
			req: &schema.ReconRequest{
				Task:      "Test task",
				RepoRoot:  repoRoot,
				SeedFiles: []string{"cmd/server/main.go"},
			},
			wantErr: false,
		},
		{
			name: "request with profile",
			req: &schema.ReconRequest{
				Task:      "Add authentication",
				RepoRoot:  repoRoot,
				Profile:   "browser_settings",
				SeedFiles: []string{"internal/auth/handler.go"},
			},
			wantErr: false,
		},
		{
			name: "request without seed files",
			req: &schema.ReconRequest{
				Task:     "Understand the codebase",
				RepoRoot: repoRoot,
			},
			wantErr: false,
		},
		{
			name: "invalid repo root",
			req: &schema.ReconRequest{
				Task:     "Test task",
				RepoRoot: "/nonexistent/path",
			},
			wantErr: true,
		},
		{
			name: "missing task",
			req: &schema.ReconRequest{
				RepoRoot: repoRoot,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pack, err := r.Run(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Runner.Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if pack == nil {
					t.Error("Expected non-nil ContextPack")
					return
				}

				// Verify pack structure
				if pack.Task != tt.req.Task {
					t.Errorf("Expected task %q, got %q", tt.req.Task, pack.Task)
				}

				if pack.Stats == nil {
					t.Error("Expected non-nil Stats")
				}

				// Verify JSON serialization
				jsonData, err := json.MarshalIndent(pack, "", "  ")
				if err != nil {
					t.Errorf("Failed to serialize ContextPack to JSON: %v", err)
				}
				if len(jsonData) == 0 {
					t.Error("Expected non-empty JSON output")
				}
			}
		})
	}
}

func TestRunner_RunFromPath(t *testing.T) {
	repoRoot := createTestRepo(t)

	// Create a temp request file
	tmpFile := filepath.Join(t.TempDir(), "request.json")
	req := &schema.ReconRequest{
		Task:      "Test from file",
		RepoRoot:  repoRoot,
		SeedFiles: []string{"cmd/server/main.go"},
	}
	reqData, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	if err := os.WriteFile(tmpFile, reqData, 0644); err != nil {
		t.Fatalf("Failed to write request file: %v", err)
	}

	// Run from path
	r := NewRunner(nil)
	pack, err := r.RunFromPath(tmpFile)
	if err != nil {
		t.Fatalf("RunFromPath failed: %v", err)
	}

	if pack == nil {
		t.Fatal("Expected non-nil ContextPack")
	}

	if pack.Task != req.Task {
		t.Errorf("Expected task %q, got %q", req.Task, pack.Task)
	}
}

func TestRunner_BudgetLimits(t *testing.T) {
	repoRoot := createTestRepo(t)

	cfg := config.DefaultConfig()
	cfg.Runtime.MaxCandidates = 3 // Limit candidates

	r := NewRunner(cfg)

	req := &schema.ReconRequest{
		Task:      "Budget test",
		RepoRoot:  repoRoot,
		SeedFiles: []string{"cmd/server/main.go"},
	}

	pack, err := r.Run(req)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if pack == nil {
		t.Fatal("Expected non-nil ContextPack")
	}

	// The pack should have limited files
	totalFiles := len(pack.MainChain) + len(pack.CompanionFiles) + len(pack.UncertainNodes)
	if totalFiles > 3 {
		t.Errorf("Expected at most 3 files due to budget, got %d", totalFiles)
	}
}

func TestRunner_EmptyRepo(t *testing.T) {
	// Create empty repo
	tmpDir := t.TempDir()

	r := NewRunner(nil)
	req := &schema.ReconRequest{
		Task:     "Empty repo test",
		RepoRoot: tmpDir,
	}

	pack, err := r.Run(req)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if pack == nil {
		t.Fatal("Expected non-nil ContextPack")
	}

	// Empty repo should produce empty results
	if len(pack.MainChain) > 0 || len(pack.CompanionFiles) > 0 {
		t.Error("Expected empty results for empty repo")
	}
}

func TestRunner_LLMRerank(t *testing.T) {
	repoRoot := createTestRepo(t)

	cfg := config.DefaultConfig()
	cfg.Runtime.EnableModelRerank = true
	cfg.Runtime.MaxConcurrency = 2
	cfg.Runtime.MaxInputTokens = 256

	r := NewRunner(cfg)
	mockAdapter := llm.NewMockAdapter()
	mockAdapter.ExecuteFunc = func(ctx context.Context, card *llm.TaskCard) (*llm.TaskResult, error) {
		switch card.FilePath {
		case "internal/auth/handler.go":
			return &llm.TaskResult{
				Type:           llm.TaskClassifyFileRole,
				Classification: "main_chain",
				Confidence:     1.0,
			}, nil
		default:
			return &llm.TaskResult{
				Type:           llm.TaskClassifyFileRole,
				Classification: "irrelevant",
				Confidence:     1.0,
			}, nil
		}
	}
	r.adapter = mockAdapter

	req := &schema.ReconRequest{
		Task:      "Add authentication",
		RepoRoot:  repoRoot,
		SeedFiles: []string{"internal/auth/middleware.go"},
		Budget: &schema.Budget{
			MaxLLMJobs: 4,
		},
	}

	pack, err := r.Run(req)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !pack.Stats.ModelEnhanced {
		t.Fatal("expected model_enhanced to be true")
	}

	found := false
	for _, path := range pack.MainChain {
		if path == "internal/auth/handler.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected LLM rerank to promote internal/auth/handler.go, got main_chain=%v", pack.MainChain)
	}
}

func TestRunner_LLMRerankLogsSummary(t *testing.T) {
	repoRoot := createTestRepo(t)

	cfg := config.DefaultConfig()
	cfg.Runtime.EnableModelRerank = true

	progress := &recordingProgress{}
	r := NewRunnerWithProgress(cfg, progress)
	r.adapter = llm.NewMockAdapter()

	req := &schema.ReconRequest{
		Task:      "Inspect auth flow",
		RepoRoot:  repoRoot,
		SeedFiles: []string{"internal/auth/handler.go"},
	}

	_, err := r.Run(req)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	found := false
	for _, msg := range progress.messages {
		if strings.Contains(msg, "llm rerank: requested=") &&
			strings.Contains(msg, "succeeded=") &&
			strings.Contains(msg, "failed=") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected LLM rerank summary log, got %v", progress.messages)
	}
}

func TestAvailableContextTokens_ReservesSystemPromptAndContextHeader(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.MaxInputTokens = 200
	cfg.Provider.SystemPromptPath = filepath.Join(t.TempDir(), "system.txt")
	if err := os.WriteFile(cfg.Provider.SystemPromptPath, []byte("custom system prompt for tests"), 0644); err != nil {
		t.Fatalf("failed to write system prompt: %v", err)
	}

	card := schema.NewFileCard("internal/auth/handler.go")
	taskCard := llm.NewTaskCardFromRequest(llm.TaskClassifyFileRole, &schema.ReconRequest{
		Task:      "Inspect auth flow",
		SeedFiles: []string{"internal/auth/handler.go"},
	}, card)

	got := availableContextTokens(cfg, taskCard)
	want := cfg.Runtime.MaxInputTokens -
		estimateTokenCount(taskCard.ToPrompt()) -
		estimateTokenCount(llm.LoadSystemPrompt(cfg.Provider.SystemPromptPath)) -
		estimateTokenCount("## Context\n\n")
	if want < 0 {
		want = 0
	}

	if got != want {
		t.Fatalf("availableContextTokens() = %d, want %d", got, want)
	}
}

func TestBuildTaskContext_FillsRemainingBudgetWithSnippets(t *testing.T) {
	repoRoot := createTestRepo(t)
	card := schema.NewFileCard("internal/auth/handler.go")
	card.Lang = "go"
	card.Symbols = []string{"Handler", "Login"}
	card.DiscoveredBy = []string{"seed"}
	card.HeuristicTags = []string{"auth"}

	context := buildTaskContext(repoRoot, card, []string{"Login"}, 128)

	if !strings.Contains(context, "Relevant code snippets:") {
		t.Fatalf("expected context to include code snippets, got: %s", context)
	}
	if !strings.Contains(context, "Login") {
		t.Fatalf("expected context to include Login snippet, got: %s", context)
	}
	if got := estimateTokenCount(context); got > 128 {
		t.Fatalf("expected context to stay within budget, got %d tokens", got)
	}
}

func TestBuildTaskContext_IncludesOutlineAndImports(t *testing.T) {
	repoRoot := createTestRepo(t)
	card := schema.NewFileCard("internal/auth/handler.go")
	card.Lang = "go"
	card.Symbols = []string{"Handler", "Login"}

	context := buildTaskContext(repoRoot, card, []string{"Login"}, 220)

	if !strings.Contains(context, "File outline:") {
		t.Fatalf("expected file outline in context, got: %s", context)
	}
	if !strings.Contains(context, "Package: auth") {
		t.Fatalf("expected package hint in context, got: %s", context)
	}
	if !strings.Contains(context, "test-repo/internal/config") {
		t.Fatalf("expected import hint in context, got: %s", context)
	}
	if !strings.Contains(context, "func (h *Handler) Login") {
		t.Fatalf("expected declaration hint in context, got: %s", context)
	}
}

func TestBuildTaskContext_EmptyWhenBudgetExhausted(t *testing.T) {
	repoRoot := createTestRepo(t)
	card := schema.NewFileCard("internal/auth/handler.go")
	card.Lang = "go"
	card.DiscoveredBy = []string{"seed"}
	card.HeuristicTags = []string{"auth"}
	card.Scores.DiscoveryScore = 1.0
	card.Scores.HeuristicScore = 0.6

	context := buildTaskContext(repoRoot, card, []string{"Login"}, 0)
	if context != "" {
		t.Fatalf("expected empty context with zero budget, got %q", context)
	}

	tinyBudget := estimateTokenCount(buildStaticHints(card)) - 1
	if tinyBudget < 1 {
		tinyBudget = 1
	}
	context = buildTaskContext(repoRoot, card, []string{"Login"}, tinyBudget)
	if context != "" {
		t.Fatalf("expected empty context when hints exceed budget, got %q", context)
	}
}

func TestFormatStructuralScore_UsesRankerFormula(t *testing.T) {
	scores := &schema.FileScores{
		DiscoveryScore: 0.7,
		ModuleWeight:   0.8,
		HeuristicScore: 0.5,
		ProfileScore:   0.4,
	}

	got := formatStructuralScore(scores)
	wantScore := ranking.StructuralScore(scores, ranking.DefaultRankerConfig())

	if !strings.Contains(got, "Structural score:") {
		t.Fatalf("expected structural score line, got %q", got)
	}
	if !strings.Contains(got, "discovery=0.70") ||
		!strings.Contains(got, "module=0.80") ||
		!strings.Contains(got, "heuristic=0.50") ||
		!strings.Contains(got, "profile=0.40") {
		t.Fatalf("expected component breakdown, got %q", got)
	}
	if !strings.Contains(got, fmt.Sprintf("Structural score: %.2f", wantScore)) {
		t.Fatalf("expected formatted score to match ranker formula (%0.2f), got %q", wantScore, got)
	}
}

func TestRunner_ContextPackContent(t *testing.T) {
	repoRoot := createTestRepo(t)

	r := NewRunner(nil)
	req := &schema.ReconRequest{
		Task:        "Test content generation",
		RepoRoot:    repoRoot,
		SeedFiles:   []string{"internal/auth/handler.go"},
		FocusChecks: []string{"security"},
	}

	pack, err := r.Run(req)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify the seed file is included
	found := false
	for _, f := range pack.MainChain {
		if f == "internal/auth/handler.go" {
			found = true
			break
		}
	}
	if !found {
		// Check companion files too
		for _, f := range pack.CompanionFiles {
			if f == "internal/auth/handler.go" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("Expected seed file to be included in results")
	}

	// Verify reading order
	if len(pack.ReadingOrder) == 0 && (len(pack.MainChain) > 0 || len(pack.CompanionFiles) > 0) {
		t.Error("Expected non-empty reading order when there are files")
	}
}
