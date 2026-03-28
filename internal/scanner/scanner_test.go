package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestScanRepo(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "scanner_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test directory structure
	dirs := []string{
		"src",
		"src/subdir",
		".git/objects",
		"node_modules/package",
		"dist",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create test files
	files := []string{
		"main.go",
		"src/app.go",
		"src/subdir/helper.go",
		"README.md",
		".git/config",
		"node_modules/package/index.js",
		"dist/bundle.js",
	}
	for _, file := range files {
		path := filepath.Join(tmpDir, file)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", file, err)
		}
	}

	// Scan the directory
	result, err := ScanRepo(tmpDir)
	if err != nil {
		t.Fatalf("ScanRepo failed: %v", err)
	}

	// Expected files (excluding ignored directories)
	expected := []string{
		"README.md",
		"main.go",
		"src/app.go",
		"src/subdir/helper.go",
	}

	// Verify count
	if len(result) != len(expected) {
		t.Errorf("expected %d files, got %d: %v", len(expected), len(result), result)
	}

	// Verify content
	sort.Strings(expected)
	for i, exp := range expected {
		if i >= len(result) || result[i] != exp {
			t.Errorf("expected file %q at position %d, got %q", exp, i, result[i])
		}
	}
}

func TestScannerWithIgnoreDirs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directories
	dirs := []string{
		"src",
		"custom_ignore",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create files
	files := []string{
		"main.go",
		"src/app.go",
		"custom_ignore/skip.go",
	}
	for _, file := range files {
		path := filepath.Join(tmpDir, file)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", file, err)
		}
	}

	// Scan with custom ignore
	customIgnore := map[string]bool{
		"custom_ignore": true,
	}
	result, err := ScanRepoWithIgnore(tmpDir, customIgnore)
	if err != nil {
		t.Fatalf("ScanRepoWithIgnore failed: %v", err)
	}

	expected := []string{
		"main.go",
		"src/app.go",
	}

	if len(result) != len(expected) {
		t.Errorf("expected %d files, got %d: %v", len(expected), len(result), result)
	}
}

func TestScannerWithMaxDepth(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested directories
	dirs := []string{
		"level1",
		"level1/level2",
		"level1/level2/level3",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create files at different levels
	files := []string{
		"root.go",
		"level1/one.go",
		"level1/level2/two.go",
		"level1/level2/level3/three.go",
	}
	for _, file := range files {
		path := filepath.Join(tmpDir, file)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", file, err)
		}
	}

	// Scan with max depth 1 (only root level)
	scanner := New(tmpDir).WithMaxDepth(1)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	expected := []string{
		"root.go",
	}

	if len(result) != len(expected) {
		t.Errorf("expected %d files with max depth 1, got %d: %v", len(expected), len(result), result)
	}

	// Scan with max depth 2
	scanner = New(tmpDir).WithMaxDepth(2)
	result, err = scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	expected = []string{
		"level1/one.go",
		"root.go",
	}

	if len(result) != len(expected) {
		t.Errorf("expected %d files with max depth 2, got %d: %v", len(expected), len(result), result)
	}
}

func TestScanRepoNonExistent(t *testing.T) {
	_, err := ScanRepo("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestScannerEmptyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result, err := ScanRepo(tmpDir)
	if err != nil {
		t.Fatalf("ScanRepo failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty result for empty directory, got %d files", len(result))
	}
}

func TestScannerStableOutput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple files
	files := []string{
		"z.txt",
		"a.txt",
		"m.txt",
	}
	for _, file := range files {
		path := filepath.Join(tmpDir, file)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", file, err)
		}
	}

	// Scan multiple times and verify consistent ordering
	for i := 0; i < 3; i++ {
		result, err := ScanRepo(tmpDir)
		if err != nil {
			t.Fatalf("ScanRepo failed: %v", err)
		}

		expected := []string{"a.txt", "m.txt", "z.txt"}
		for j, exp := range expected {
			if result[j] != exp {
				t.Errorf("iteration %d: expected %q at position %d, got %q", i, exp, j, result[j])
			}
		}
	}
}

func TestDefaultIgnoreDirs(t *testing.T) {
	// Verify that DefaultIgnoreDirs contains expected entries
	expectedIgnores := []string{
		".git",
		"node_modules",
		"dist",
		"out",
		"build",
		"target",
		"vendor",
	}

	for _, dir := range expectedIgnores {
		if !DefaultIgnoreDirs[dir] {
			t.Errorf("DefaultIgnoreDirs should contain %q", dir)
		}
	}
}

func TestIsIgnored(t *testing.T) {
	tests := []struct {
		name       string
		dirName    string
		ignoreDirs map[string]bool
		want       bool
	}{
		{
			name:       "default ignore .git",
			dirName:    ".git",
			ignoreDirs: nil,
			want:       true,
		},
		{
			name:       "default does not ignore src",
			dirName:    "src",
			ignoreDirs: nil,
			want:       false,
		},
		{
			name:    "custom ignore",
			dirName: "custom",
			ignoreDirs: map[string]bool{
				"custom": true,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isIgnored(tt.dirName, tt.ignoreDirs)
			if got != tt.want {
				t.Errorf("isIgnored(%q, %v) = %v, want %v", tt.dirName, tt.ignoreDirs, got, tt.want)
			}
		})
	}
}
