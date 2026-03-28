// Package scanner provides repository file scanning functionality.
package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultIgnoreDirs contains directories that are commonly ignored during scanning.
var DefaultIgnoreDirs = map[string]bool{
	".git":         true,
	".hg":          true,
	".svn":         true,
	".idea":        true,
	".vscode":      true,
	"node_modules": true,
	"vendor":       true,
	"out":          true,
	"dist":         true,
	"build":        true,
	"target":       true,
	"bin":          true,
	"__pycache__":  true,
	".cache":       true,
}

// Scanner scans a repository to build a list of file paths.
type Scanner struct {
	// Root is the repository root directory.
	Root string

	// IgnoreDirs are directories to skip during scanning.
	// If nil, DefaultIgnoreDirs is used.
	IgnoreDirs map[string]bool

	// MaxDepth limits the recursion depth (0 = unlimited).
	MaxDepth int
}

// New creates a new Scanner for the given repository root.
func New(root string) *Scanner {
	return &Scanner{
		Root:       root,
		IgnoreDirs: nil,
		MaxDepth:   0,
	}
}

// WithIgnoreDirs sets custom ignore directories.
func (s *Scanner) WithIgnoreDirs(dirs map[string]bool) *Scanner {
	s.IgnoreDirs = dirs
	return s
}

// WithMaxDepth sets the maximum recursion depth.
func (s *Scanner) WithMaxDepth(depth int) *Scanner {
	s.MaxDepth = depth
	return s
}

// Scan performs a recursive scan of the repository and returns relative file paths.
func (s *Scanner) Scan() ([]string, error) {
	ignoreDirs := s.IgnoreDirs
	if ignoreDirs == nil {
		ignoreDirs = DefaultIgnoreDirs
	}

	var files []string

	err := filepath.Walk(s.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from repo root
		relPath, err := filepath.Rel(s.Root, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Check depth limit
		if s.MaxDepth > 0 {
			depth := strings.Count(relPath, string(filepath.Separator))
			if depth >= s.MaxDepth {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Check if directory should be ignored
		if info.IsDir() {
			if ignoreDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Only include regular files
		if info.Mode().IsRegular() {
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort for stable output
	sort.Strings(files)

	return files, nil
}

// ScanRepo is a convenience function that creates a Scanner and scans the given root.
func ScanRepo(root string) ([]string, error) {
	return New(root).Scan()
}

// ScanRepoWithIgnore is a convenience function that scans with custom ignore directories.
func ScanRepoWithIgnore(root string, ignoreDirs map[string]bool) ([]string, error) {
	return New(root).WithIgnoreDirs(ignoreDirs).Scan()
}

// isIgnored checks if a directory name should be ignored.
func isIgnored(name string, ignoreDirs map[string]bool) bool {
	if ignoreDirs == nil {
		return DefaultIgnoreDirs[name]
	}
	return ignoreDirs[name]
}
