package heuristics

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates a file with content under dir.
func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestImportGraph_GoDirectoryResolution(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "cmd/main.go", `package main
import "github.com/example/myapp/internal/config"
`)
	writeFile(t, dir, "internal/config/config.go", `package config`)
	writeFile(t, dir, "internal/config/loader.go", `package config`)

	allFiles := []string{
		"cmd/main.go",
		"internal/config/config.go",
		"internal/config/loader.go",
	}

	g := NewImportGraphBuilder(dir).Build(allFiles)

	deps := g.Deps["cmd/main.go"]
	if len(deps) == 0 {
		t.Fatal("expected deps for cmd/main.go, got none")
	}
	depSet := make(map[string]bool)
	for _, d := range deps {
		depSet[d] = true
	}
	if !depSet["internal/config/config.go"] {
		t.Error("expected internal/config/config.go in deps")
	}
	if !depSet["internal/config/loader.go"] {
		t.Error("expected internal/config/loader.go in deps")
	}

	// reverse: both config files should point back to main
	for _, f := range []string{"internal/config/config.go", "internal/config/loader.go"} {
		found := false
		for _, r := range g.RevDeps[f] {
			if r == "cmd/main.go" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected cmd/main.go in RevDeps[%s]", f)
		}
	}
}

func TestImportGraph_PythonRelativeImport(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "pkg/service.py", `from .models import User
from ..utils import helper
`)
	writeFile(t, dir, "pkg/models.py", `class User: pass`)
	writeFile(t, dir, "utils.py", `def helper(): pass`)

	allFiles := []string{"pkg/service.py", "pkg/models.py", "utils.py"}

	g := NewImportGraphBuilder(dir).Build(allFiles)

	deps := g.Deps["pkg/service.py"]
	depSet := make(map[string]bool)
	for _, d := range deps {
		depSet[d] = true
	}
	if !depSet["pkg/models.py"] {
		t.Errorf("expected pkg/models.py in deps, got %v", deps)
	}
	if !depSet["utils.py"] {
		t.Errorf("expected utils.py in deps, got %v", deps)
	}
}

func TestImportGraph_Neighbors(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "a/a.go", `package a
import "github.com/x/repo/b"
`)
	writeFile(t, dir, "b/b.go", `package b`)

	allFiles := []string{"a/a.go", "b/b.go"}
	g := NewImportGraphBuilder(dir).Build(allFiles)

	neighbors := g.Neighbors("a/a.go")
	if len(neighbors) == 0 {
		t.Fatal("expected neighbors for a/a.go")
	}
	found := false
	for _, n := range neighbors {
		if n == "b/b.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected b/b.go in neighbors of a/a.go, got %v", neighbors)
	}

	// reverse neighbor
	neighbors = g.Neighbors("b/b.go")
	found = false
	for _, n := range neighbors {
		if n == "a/a.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a/a.go in neighbors of b/b.go, got %v", neighbors)
	}
}

func TestNeighborExpander_SourceImport(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "cmd/main.go", `package main
import "github.com/x/repo/internal/handler"
`)
	writeFile(t, dir, "internal/handler/handler.go", `package handler`)

	allFiles := []string{"cmd/main.go", "internal/handler/handler.go"}
	g := NewImportGraphBuilder(dir).Build(allFiles)

	expander := NewNeighborExpander(nil).WithImportGraph(g)
	result := expander.ExpandWithSources([]string{"cmd/main.go"}, allFiles)

	found := false
	for _, c := range result.Candidates {
		if c == "internal/handler/handler.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected internal/handler/handler.go in candidates, got %v", result.Candidates)
	}

	sources := result.Sources["internal/handler/handler.go"]
	hasImport := false
	for _, s := range sources {
		if s == SourceImport {
			hasImport = true
			break
		}
	}
	if !hasImport {
		t.Errorf("expected SourceImport in sources for handler.go, got %v", sources)
	}
}

func TestNeighborExpander_Expand_IncludesImport(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "main.go", `package main
import "github.com/x/repo/util"
`)
	writeFile(t, dir, "util/util.go", `package util`)

	allFiles := []string{"main.go", "util/util.go"}
	g := NewImportGraphBuilder(dir).Build(allFiles)

	expander := NewNeighborExpander(nil).WithImportGraph(g)
	candidates := expander.Expand([]string{"main.go"}, allFiles)

	found := false
	for _, c := range candidates {
		if c == "util/util.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected util/util.go in Expand candidates, got %v", candidates)
	}
}
