// Package heuristics provides heuristic rules for file analysis.
package heuristics

import (
	"path/filepath"
	"strings"

	"github.com/no22/repo-scout/internal/analysis"
	"github.com/no22/repo-scout/internal/scanner"
)

// ImportGraph maps each file (relative path) to the set of files it imports.
// Keys and values are all relative paths from repo root.
type ImportGraph struct {
	// Deps maps file → []imported files (relative paths)
	Deps map[string][]string
	// RevDeps maps file → []files that import it
	RevDeps map[string][]string
}

// ImportGraphBuilder builds an ImportGraph for a repository.
type ImportGraphBuilder struct {
	repoRoot  string
	extractor *scanner.ImportExtractor
	sourceIdx *analysis.SourceIndex
}

// NewImportGraphBuilder creates a builder for the given repo root.
func NewImportGraphBuilder(repoRoot string) *ImportGraphBuilder {
	return &ImportGraphBuilder{
		repoRoot:  repoRoot,
		extractor: scanner.NewImportExtractor(),
	}
}

// WithSourceIndex reuses a shared source cache for reading file contents.
func (b *ImportGraphBuilder) WithSourceIndex(idx *analysis.SourceIndex) *ImportGraphBuilder {
	b.sourceIdx = idx
	return b
}

// Build constructs the ImportGraph for the given set of files.
// allFiles must be relative paths from repo root.
func (b *ImportGraphBuilder) Build(allFiles []string) *ImportGraph {
	// Build a lookup: basename (no ext) → []relpath, and fullpath → relpath
	baseIndex := buildBaseIndex(allFiles)
	pathIndex := buildPathIndex(allFiles)
	dirIndex := buildDirIndex(allFiles)

	deps := make(map[string][]string, len(allFiles))
	revDeps := make(map[string][]string, len(allFiles))

	for _, relPath := range allFiles {
		lang := LangDetect(relPath)
		content, ok := b.readContent(relPath)
		if !ok {
			continue
		}

		imports := b.extractor.ExtractImports(content, lang)
		var resolved []string
		for _, imp := range imports {
			targets := resolveImport(imp, relPath, lang, baseIndex, pathIndex, dirIndex)
			for _, t := range targets {
				if t != relPath {
					resolved = append(resolved, t)
					revDeps[t] = appendUnique(revDeps[t], relPath)
				}
			}
		}
		if len(resolved) > 0 {
			deps[relPath] = resolved
		}
	}

	return &ImportGraph{Deps: deps, RevDeps: revDeps}
}

func (b *ImportGraphBuilder) readContent(relPath string) (string, bool) {
	if b.sourceIdx != nil {
		if content, ok := b.sourceIdx.Content(relPath); ok {
			return content, true
		}
	}

	if b.repoRoot == "" {
		return "", false
	}

	fallbackIdx := analysis.NewSourceIndex(b.repoRoot)
	return fallbackIdx.Content(relPath)
}

// Neighbors returns files that the given file imports or that import it.
func (g *ImportGraph) Neighbors(relPath string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, d := range g.Deps[relPath] {
		if !seen[d] {
			seen[d] = true
			result = append(result, d)
		}
	}
	for _, d := range g.RevDeps[relPath] {
		if !seen[d] {
			seen[d] = true
			result = append(result, d)
		}
	}
	return result
}

// resolveImport tries to map a raw import string to one or more repo-relative paths.
func resolveImport(imp, fromFile, lang string, baseIndex map[string][]string, pathIndex map[string]string, dirIndex map[string][]string) []string {
	switch lang {
	case "js", "jsx", "ts", "tsx", "rb":
		return resolveRelativeOrBase(imp, fromFile, baseIndex, pathIndex)
	case "c", "cpp", "h", "hpp":
		return resolveCInclude(imp, fromFile, baseIndex, pathIndex)
	case "go":
		return resolveGoImport(imp, dirIndex, baseIndex)
	case "py":
		return resolvePython(imp, fromFile, baseIndex, pathIndex)
	case "java":
		return resolveJava(imp, baseIndex)
	case "rust":
		return resolveRust(imp, fromFile, baseIndex)
	case "php":
		return resolveRelativeOrBase(imp, fromFile, baseIndex, pathIndex)
	}
	return nil
}

// resolveRelativeOrBase handles JS/TS/Ruby style: './foo', '../bar', or bare names.
func resolveRelativeOrBase(imp, fromFile string, baseIndex map[string][]string, pathIndex map[string]string) []string {
	if strings.HasPrefix(imp, ".") {
		// relative path
		dir := filepath.Dir(fromFile)
		candidate := filepath.ToSlash(filepath.Join(dir, imp))
		// exact match
		if _, ok := pathIndex[candidate]; ok {
			return []string{candidate}
		}
		// try with common extensions
		for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".go", ".py", ".rb", ".php"} {
			if t, ok := pathIndex[candidate+ext]; ok {
				return []string{t}
			}
		}
		// try index file
		for _, idx := range []string{"/index.ts", "/index.tsx", "/index.js"} {
			if t, ok := pathIndex[candidate+idx]; ok {
				return []string{t}
			}
		}
		// fallback: match by basename
		base := filepath.Base(candidate)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		return baseIndex[base]
	}
	// bare module name — match last segment
	seg := lastSegment(imp, "/")
	seg = strings.TrimSuffix(seg, filepath.Ext(seg))
	return baseIndex[seg]
}

// resolveCInclude handles #include "foo/bar.h"
func resolveCInclude(imp, fromFile string, baseIndex map[string][]string, pathIndex map[string]string) []string {
	// relative include
	dir := filepath.Dir(fromFile)
	candidate := filepath.ToSlash(filepath.Join(dir, imp))
	if t, ok := pathIndex[candidate]; ok {
		return []string{t}
	}
	// search by basename
	base := filepath.Base(imp)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return baseIndex[base]
}

// resolveGoImport handles Go imports: the last path segment is the package directory name.
// It returns all .go files in the matching directory.
// Falls back to basename matching if no directory match is found.
func resolveGoImport(imp string, dirIndex map[string][]string, baseIndex map[string][]string) []string {
	// Try progressively longer suffixes of the import path to find a matching dir.
	// e.g. "github.com/foo/bar/baz" → try "bar/baz", then "baz"
	parts := strings.Split(imp, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		suffix := strings.ToLower(strings.Join(parts[i:], "/"))
		if files, ok := dirIndex[suffix]; ok {
			return files
		}
	}
	// fallback: basename match on last segment
	seg := strings.ToLower(parts[len(parts)-1])
	return baseIndex[seg]
}

// resolvePython handles Python imports.
// Relative imports (leading dots) are resolved relative to fromFile's directory.
// Each dot beyond the first goes one directory up.
// Absolute imports match by last dotted segment via baseIndex.
func resolvePython(imp, fromFile string, baseIndex map[string][]string, pathIndex map[string]string) []string {
	if !strings.HasPrefix(imp, ".") {
		// absolute import: match last segment
		parts := strings.Split(imp, ".")
		seg := strings.ToLower(parts[len(parts)-1])
		return baseIndex[seg]
	}

	// count leading dots to determine how many dirs to go up
	dots := 0
	for _, c := range imp {
		if c == '.' {
			dots++
		} else {
			break
		}
	}
	module := imp[dots:] // module name after dots (may be empty for "from . import")

	// start from fromFile's directory, go up (dots-1) levels
	dir := filepath.ToSlash(filepath.Dir(fromFile))
	for i := 1; i < dots; i++ {
		dir = filepath.ToSlash(filepath.Dir(dir))
	}
	if dir == "." {
		dir = ""
	}

	if module == "" {
		// "from . import X" — can't resolve without knowing X; skip
		return nil
	}

	// convert dotted module to path candidate
	modPath := strings.ReplaceAll(module, ".", "/")
	var candidate string
	if dir == "" {
		candidate = modPath
	} else {
		candidate = dir + "/" + modPath
	}

	// try exact file match with Python extensions
	for _, ext := range []string{".py", ".pyx", ".pyi"} {
		if t, ok := pathIndex[candidate+ext]; ok {
			return []string{t}
		}
	}
	// try as package directory (__init__.py)
	for _, init := range []string{"/__init__.py", "/__init__.pyi"} {
		if t, ok := pathIndex[candidate+init]; ok {
			return []string{t}
		}
	}

	// fallback: match last segment via baseIndex
	parts := strings.Split(modPath, "/")
	seg := strings.ToLower(parts[len(parts)-1])
	return baseIndex[seg]
}

// resolveJava handles "com.example.Foo" → look for Foo.
func resolveJava(imp string, baseIndex map[string][]string) []string {
	seg := lastSegment(imp, ".")
	if seg == "*" {
		// wildcard — match package dir
		parts := strings.Split(imp, ".")
		if len(parts) >= 2 {
			seg = parts[len(parts)-2]
		}
	}
	return baseIndex[seg]
}

// resolveRust handles "crate::foo::Bar" or "mod foo".
func resolveRust(imp, fromFile string, baseIndex map[string][]string) []string {
	// strip crate:: / super:: / self::
	clean := imp
	for _, prefix := range []string{"crate::", "super::", "self::"} {
		clean = strings.TrimPrefix(clean, prefix)
	}
	seg := lastSegment(clean, "::")
	// strip trailing {…}
	if idx := strings.Index(seg, "{"); idx >= 0 {
		seg = seg[:idx]
	}
	seg = strings.TrimSpace(seg)
	if seg == "" {
		return nil
	}
	return baseIndex[seg]
}

// buildBaseIndex maps lowercase basename-without-ext → []relpath.
func buildBaseIndex(allFiles []string) map[string][]string {
	idx := make(map[string][]string, len(allFiles))
	for _, f := range allFiles {
		base := filepath.Base(f)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		key := strings.ToLower(base)
		idx[key] = append(idx[key], f)
	}
	return idx
}

// buildDirIndex maps lowercase relative directory path → []relpath of files in that dir.
// Also indexes all suffix sub-paths so "bar/baz" matches "github.com/foo/bar/baz".
func buildDirIndex(allFiles []string) map[string][]string {
	idx := make(map[string][]string)
	for _, f := range allFiles {
		dir := strings.ToLower(filepath.ToSlash(filepath.Dir(f)))
		if dir == "." {
			dir = ""
		}
		idx[dir] = append(idx[dir], f)
		// index all suffixes: "a/b/c" → also index "b/c" and "c"
		parts := strings.Split(dir, "/")
		for i := 1; i < len(parts); i++ {
			suffix := strings.Join(parts[i:], "/")
			if suffix != dir {
				idx[suffix] = appendUnique(idx[suffix], f)
			}
		}
	}
	return idx
}

// buildPathIndex maps normalized relpath → relpath (identity, for existence checks).
func buildPathIndex(allFiles []string) map[string]string {
	idx := make(map[string]string, len(allFiles))
	for _, f := range allFiles {
		idx[filepath.ToSlash(f)] = f
	}
	return idx
}

func lastSegment(s, sep string) string {
	parts := strings.Split(s, sep)
	return strings.ToLower(parts[len(parts)-1])
}

func appendUnique(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}
