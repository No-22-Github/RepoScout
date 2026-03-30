// Package analysis provides reusable source-level analysis helpers.
package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SourceIndex caches candidate file contents so static-analysis stages can
// reuse the same source text without repeatedly reading from disk.
type SourceIndex struct {
	repoRoot string

	mu          sync.RWMutex
	content     map[string]string
	missing     map[string]bool
	symbolLines map[string]map[string]map[string]int
}

// NewSourceIndex creates a source cache rooted at the repository path.
func NewSourceIndex(repoRoot string) *SourceIndex {
	return &SourceIndex{
		repoRoot:    repoRoot,
		content:     make(map[string]string),
		missing:     make(map[string]bool),
		symbolLines: make(map[string]map[string]map[string]int),
	}
}

// Content returns the file content for the given repo-relative path.
func (s *SourceIndex) Content(relPath string) (string, bool) {
	if s == nil || s.repoRoot == "" || relPath == "" {
		return "", false
	}

	s.mu.RLock()
	if content, ok := s.content[relPath]; ok {
		s.mu.RUnlock()
		return content, true
	}
	if s.missing[relPath] {
		s.mu.RUnlock()
		return "", false
	}
	s.mu.RUnlock()

	data, err := os.ReadFile(filepath.Join(s.repoRoot, relPath))
	if err != nil || len(data) == 0 {
		s.mu.Lock()
		s.missing[relPath] = true
		s.mu.Unlock()
		return "", false
	}

	content := string(data)

	s.mu.Lock()
	s.content[relPath] = content
	delete(s.missing, relPath)
	s.mu.Unlock()

	return content, true
}

// Lines returns the file split into lines, optionally truncating by byte count.
func (s *SourceIndex) Lines(relPath string, byteLimit int) ([]string, bool) {
	content, ok := s.Content(relPath)
	if !ok {
		return nil, false
	}

	if byteLimit > 0 && len(content) > byteLimit {
		content = content[:byteLimit]
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil, false
	}
	return lines, true
}
