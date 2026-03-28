// Package runner orchestrates the full reconnaissance pipeline.
package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/no22/repo-scout/internal/config"
	"github.com/no22/repo-scout/internal/schema"
)

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

type Handler struct {
	Name string
}

func (h *Handler) Login() error {
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
