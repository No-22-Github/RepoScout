// Package scanner provides repository file scanning functionality.
package scanner

import (
	"regexp"
	"strings"
)

// ImportExtractor extracts import paths from source code using regex.
// It does not resolve paths — callers are responsible for resolution.
type ImportExtractor struct{}

// NewImportExtractor creates a new ImportExtractor.
func NewImportExtractor() *ImportExtractor {
	return &ImportExtractor{}
}

// ExtractImports extracts raw import path strings from source content.
// The lang parameter selects the extraction rules.
// Returns an empty slice for unsupported languages.
func (e *ImportExtractor) ExtractImports(content, lang string) []string {
	switch lang {
	case "go":
		return extractGoImports(content)
	case "js", "jsx", "ts", "tsx":
		return extractJSImports(content)
	case "py":
		return extractPythonImports(content)
	case "java":
		return extractJavaImports(content)
	case "rust":
		return extractRustImports(content)
	case "c", "cpp", "h", "hpp":
		return extractCImports(content)
	case "rb":
		return extractRubyImports(content)
	case "php":
		return extractPHPImports(content)
	default:
		return nil
	}
}

// Go: import "pkg" or import ( "pkg" )
var (
	goSingleImport = regexp.MustCompile(`(?m)^\s*import\s+"([^"]+)"`)
	goBlockImport  = regexp.MustCompile(`"([^"]+)"`)
	goBlockRegion  = regexp.MustCompile(`(?s)import\s*\(([^)]+)\)`)
)

func extractGoImports(content string) []string {
	seen := make(map[string]bool)
	var result []string

	// single-line imports
	for _, m := range goSingleImport.FindAllStringSubmatch(content, -1) {
		if p := m[1]; !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	// block imports
	for _, block := range goBlockRegion.FindAllStringSubmatch(content, -1) {
		for _, m := range goBlockImport.FindAllStringSubmatch(block[1], -1) {
			if p := m[1]; !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}
	return result
}

// JS/TS: import ... from '...' / require('...') / export ... from '...'
var (
	jsFromPattern    = regexp.MustCompile(`(?m)(?:import|export)[^'"` + "`" + `]*from\s+['"]([^'"` + "`" + `]+)['"]`)
	jsRequirePattern = regexp.MustCompile(`require\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	jsDynImport      = regexp.MustCompile(`import\s*\(\s*['"]([^'"]+)['"]\s*\)`)
)

func extractJSImports(content string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, pat := range []*regexp.Regexp{jsFromPattern, jsRequirePattern, jsDynImport} {
		for _, m := range pat.FindAllStringSubmatch(content, -1) {
			if p := m[1]; !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}
	return result
}

// Python: import foo / from foo import bar / from .foo import bar / from . import bar
var (
	pyFromPattern   = regexp.MustCompile(`(?m)^\s*from\s+(\.+[\w.]*)\s+import`)   // relative: from .foo import / from . import
	pyFromAbsPattern = regexp.MustCompile(`(?m)^\s*from\s+([a-zA-Z][\w.]*)\s+import`) // absolute: from foo.bar import
	pyImportPattern  = regexp.MustCompile(`(?m)^\s*import\s+([\w.,][^\n]*)`) // import foo, bar — no newline
)

func extractPythonImports(content string) []string {
	seen := make(map[string]bool)
	var result []string

	add := func(p string) {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}

	// relative imports — preserve leading dots
	for _, m := range pyFromPattern.FindAllStringSubmatch(content, -1) {
		add(m[1])
	}
	// absolute from-imports
	for _, m := range pyFromAbsPattern.FindAllStringSubmatch(content, -1) {
		add(m[1])
	}
	// bare imports
	for _, m := range pyImportPattern.FindAllStringSubmatch(content, -1) {
		for _, part := range strings.Split(m[1], ",") {
			add(strings.Split(strings.TrimSpace(part), " ")[0])
		}
	}
	return result
}

// Java: import com.example.Foo;
var javaImportPattern = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([\w.]+)\s*;`)

func extractJavaImports(content string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, m := range javaImportPattern.FindAllStringSubmatch(content, -1) {
		if p := m[1]; !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}

// Rust: use crate::foo; / mod foo;
var (
	rustUsePattern = regexp.MustCompile(`(?m)^\s*use\s+([\w:]+)`)
	rustModPattern = regexp.MustCompile(`(?m)^\s*mod\s+(\w+)\s*;`)
)

func extractRustImports(content string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, pat := range []*regexp.Regexp{rustUsePattern, rustModPattern} {
		for _, m := range pat.FindAllStringSubmatch(content, -1) {
			if p := m[1]; !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}
	return result
}

// C/C++: #include "foo.h" or #include <foo.h>
var cIncludePattern = regexp.MustCompile(`(?m)^\s*#\s*include\s+["<]([^">]+)[">]`)

func extractCImports(content string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, m := range cIncludePattern.FindAllStringSubmatch(content, -1) {
		if p := m[1]; !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}

// Ruby: require 'foo' / require_relative 'foo'
var (
	rubyRequirePattern         = regexp.MustCompile(`(?m)^\s*require\s+['"]([^'"]+)['"]`)
	rubyRequireRelativePattern = regexp.MustCompile(`(?m)^\s*require_relative\s+['"]([^'"]+)['"]`)
)

func extractRubyImports(content string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, pat := range []*regexp.Regexp{rubyRequirePattern, rubyRequireRelativePattern} {
		for _, m := range pat.FindAllStringSubmatch(content, -1) {
			if p := m[1]; !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}
	return result
}

// PHP: require/include 'foo.php'
var phpIncludePattern = regexp.MustCompile(`(?m)(?:require|include)(?:_once)?\s*\(?['"]([^'"]+)['"]\)?`)

func extractPHPImports(content string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, m := range phpIncludePattern.FindAllStringSubmatch(content, -1) {
		if p := m[1]; !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}
