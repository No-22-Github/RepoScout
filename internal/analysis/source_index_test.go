package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceIndex_ContentAndLines(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "pkg"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	content := "line1\nline2\nline3"
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg/file.txt"), []byte(content), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	index := NewSourceIndex(repoRoot)

	got, ok := index.Content("pkg/file.txt")
	if !ok || got != content {
		t.Fatalf("Content() = (%q, %v), want (%q, true)", got, ok, content)
	}

	lines, ok := index.Lines("pkg/file.txt", 0)
	if !ok || len(lines) != 3 {
		t.Fatalf("Lines() = (%v, %v), want 3 lines", lines, ok)
	}

	lines, ok = index.Lines("pkg/file.txt", len("line1\nli"))
	if !ok || len(lines) != 2 || lines[0] != "line1" || lines[1] != "li" {
		t.Fatalf("Lines() with limit = (%v, %v), want [line1 li]", lines, ok)
	}
}

func TestSourceIndex_MissingFile(t *testing.T) {
	index := NewSourceIndex(t.TempDir())
	if _, ok := index.Content("missing.txt"); ok {
		t.Fatal("expected missing file lookup to fail")
	}
	if _, ok := index.Lines("missing.txt", 0); ok {
		t.Fatal("expected missing file lines lookup to fail")
	}
}

func TestSourceIndex_SymbolLine(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "internal/auth"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	content := `package auth

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}
`
	if err := os.WriteFile(filepath.Join(repoRoot, "internal/auth/handler.go"), []byte(content), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	index := NewSourceIndex(repoRoot)
	index.PrecomputeSymbolLines("internal/auth/handler.go", "go", []string{"Handler", "NewHandler"})

	line, ok := index.SymbolLine("internal/auth/handler.go", "go", "NewHandler")
	if !ok || line != 4 {
		t.Fatalf("SymbolLine(NewHandler) = (%d, %v), want (4, true)", line, ok)
	}

	line, ok = index.SymbolLine("internal/auth/handler.go", "go", "Handler")
	if !ok || line != 2 {
		t.Fatalf("SymbolLine(Handler) = (%d, %v), want (2, true)", line, ok)
	}

	if _, ok := index.SymbolLine("internal/auth/handler.go", "go", "Missing"); ok {
		t.Fatal("expected missing symbol lookup to fail")
	}
}
