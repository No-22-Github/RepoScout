// Package scanner provides repository file scanning functionality.
package scanner

import (
	"io"
	"os"
	"regexp"
	"strings"
)

// Symbol represents an extracted symbol from source code.
type Symbol struct {
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"` // e.g., "func", "class", "const", "var"
}

// SymbolExtractor extracts symbols from source code files.
// It uses lightweight regex-based parsing, not full AST.
type SymbolExtractor struct {
	// MaxFileSize limits the file size to process (in bytes).
	// Files larger than this are skipped.
	MaxFileSize int64

	// MaxSymbols limits the number of symbols to extract per file.
	MaxSymbols int
}

// NewSymbolExtractor creates a new SymbolExtractor with default settings.
func NewSymbolExtractor() *SymbolExtractor {
	return &SymbolExtractor{
		MaxFileSize: 500 * 1024, // 500KB
		MaxSymbols:  100,
	}
}

// WithMaxFileSize sets the maximum file size to process.
func (e *SymbolExtractor) WithMaxFileSize(size int64) *SymbolExtractor {
	e.MaxFileSize = size
	return e
}

// WithMaxSymbols sets the maximum number of symbols to extract.
func (e *SymbolExtractor) WithMaxSymbols(max int) *SymbolExtractor {
	e.MaxSymbols = max
	return e
}

// ExtractFromFile extracts symbols from a file at the given path.
// Returns an empty list on error (does not fail the pipeline).
func (e *SymbolExtractor) ExtractFromFile(filePath string, lang string) []Symbol {
	// Check file size first
	info, err := os.Stat(filePath)
	if err != nil {
		return nil
	}
	if info.Size() > e.MaxFileSize {
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	return e.Extract(file, lang)
}

// Extract extracts symbols from the given reader.
// The lang parameter determines which extraction rules to use.
// Returns an empty list on error (does not fail the pipeline).
func (e *SymbolExtractor) Extract(r io.Reader, lang string) []Symbol {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil
	}

	return e.ExtractFromContent(string(content), lang)
}

// ExtractFromContent extracts symbols from source code content.
// This is the main extraction logic that dispatches to language-specific extractors.
func (e *SymbolExtractor) ExtractFromContent(content string, lang string) []Symbol {
	var symbols []Symbol

	switch lang {
	case "go":
		symbols = e.extractGo(content)
	case "js", "jsx", "ts", "tsx":
		symbols = e.extractJSOrTS(content)
	case "c", "cpp", "h", "hpp":
		symbols = e.extractCpp(content)
	case "py":
		symbols = e.extractPython(content)
	case "java":
		symbols = e.extractJava(content)
	case "rust":
		symbols = e.extractRust(content)
	default:
		// No extraction for unknown languages
		return nil
	}

	// Limit the number of symbols
	if len(symbols) > e.MaxSymbols {
		symbols = symbols[:e.MaxSymbols]
	}

	return symbols
}

// ExtractSymbolNames extracts just the symbol names (without kind info).
func (e *SymbolExtractor) ExtractSymbolNames(content string, lang string) []string {
	symbols := e.ExtractFromContent(content, lang)
	names := make([]string, len(symbols))
	for i, s := range symbols {
		names[i] = s.Name
	}
	return names
}

// Go symbol patterns
var (
	// func Name(...) or func (receiver) Name(...)
	goFuncPattern = regexp.MustCompile(`(?m)^\s*func\s+(?:\([^)]+\)\s+)?([A-Z][a-zA-Z0-9_]*)\s*[\(]`)
	// type Name struct or type Name interface
	goTypePattern = regexp.MustCompile(`(?m)^\s*type\s+([A-Z][a-zA-Z0-9_]*)\s+(?:struct|interface)`)
	// const Name = or const ( block
	goConstPattern = regexp.MustCompile(`(?m)^\s*const\s+([A-Z][a-zA-Z0-9_]*)\s*=`)
	// var Name = (module level)
	goVarPattern = regexp.MustCompile(`(?m)^\s*var\s+([A-Z][a-zA-Z0-9_]*)\s*[=\s]`)
)

func (e *SymbolExtractor) extractGo(content string) []Symbol {
	var symbols []Symbol
	seen := make(map[string]bool)

	// Extract functions
	matches := goFuncPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "func"})
			seen[m[1]] = true
		}
	}

	// Extract types (structs, interfaces)
	matches = goTypePattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "type"})
			seen[m[1]] = true
		}
	}

	// Extract constants
	matches = goConstPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "const"})
			seen[m[1]] = true
		}
	}

	// Extract vars
	matches = goVarPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "var"})
			seen[m[1]] = true
		}
	}

	return symbols
}

// JavaScript/TypeScript symbol patterns
var (
	// function name(...) or function name<T>(...)
	jsFuncPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+([a-zA-Z_$][a-zA-Z0-9_$]*)`)
	// const name = or const name: Type =
	jsConstPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?const\s+([A-Z_][A-Z0-9_$]*)\s*[=:]`)
	// class Name
	jsClassPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:default\s+)?class\s+([a-zA-Z_$][a-zA-Z0-9_$]*)`)
	// interface Name (TypeScript)
	jsInterfacePattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?interface\s+([a-zA-Z_$][a-zA-Z0-9_$]*)`)
	// type Name = (TypeScript)
	jsTypePattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?type\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=`)
	// const name = () => or const name = async () =>
	jsArrowFuncPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?const\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=\s*(?:async\s+)?(?:\([^)]*\)|[a-zA-Z_$][a-zA-Z0-9_$]*)\s*=>`)
)

func (e *SymbolExtractor) extractJSOrTS(content string) []Symbol {
	var symbols []Symbol
	seen := make(map[string]bool)

	// Extract classes
	matches := jsClassPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "class"})
			seen[m[1]] = true
		}
	}

	// Extract functions
	matches = jsFuncPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "func"})
			seen[m[1]] = true
		}
	}

	// Extract arrow functions
	matches = jsArrowFuncPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "func"})
			seen[m[1]] = true
		}
	}

	// Extract interfaces (TypeScript)
	matches = jsInterfacePattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "interface"})
			seen[m[1]] = true
		}
	}

	// Extract type aliases (TypeScript)
	matches = jsTypePattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "type"})
			seen[m[1]] = true
		}
	}

	// Extract constants (uppercase only, convention for true constants)
	matches = jsConstPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "const"})
			seen[m[1]] = true
		}
	}

	return symbols
}

// C++ symbol patterns
var (
	// class Name { or class Name : public
	cppClassPattern = regexp.MustCompile(`(?m)^\s*class\s+([A-Z][a-zA-Z0-9_]*)\s*[{:]`)
	// struct Name { (for C-style structs with capital names)
	cppStructPattern = regexp.MustCompile(`(?m)^\s*struct\s+([A-Z][a-zA-Z0-9_]*)\s*{`)
	// Type Name( or void Name( (function signature)
	cppFuncPattern = regexp.MustCompile(`(?m)^\s*(?:static\s+)?(?:inline\s+)?(?:virtual\s+)?[a-zA-Z0-9_:<>\*\&]+\s+([A-Z][a-zA-Z0-9_]*)\s*\([^)]*\)\s*(?:const\s*)?(?:override\s*)?[{;]`)
	// #define NAME
	cppDefinePattern = regexp.MustCompile(`(?m)^\s*#define\s+([A-Z_][A-Z0-9_]*)\b`)
	// constexpr Type Name = or constexpr Type kName =
	cppConstexprPattern = regexp.MustCompile(`(?m)^\s*(?:static\s+)?constexpr\s+[a-zA-Z0-9_:<>\*]+\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*=`)
)

func (e *SymbolExtractor) extractCpp(content string) []Symbol {
	var symbols []Symbol
	seen := make(map[string]bool)

	// Extract classes
	matches := cppClassPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "class"})
			seen[m[1]] = true
		}
	}

	// Extract structs
	matches = cppStructPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "struct"})
			seen[m[1]] = true
		}
	}

	// Extract functions
	matches = cppFuncPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "func"})
			seen[m[1]] = true
		}
	}

	// Extract #define macros
	matches = cppDefinePattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "macro"})
			seen[m[1]] = true
		}
	}

	// Extract constexpr values
	matches = cppConstexprPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "const"})
			seen[m[1]] = true
		}
	}

	return symbols
}

// Python symbol patterns
var (
	// def name( or async def name(
	pyFuncPattern = regexp.MustCompile(`(?m)^\s*(?:async\s+)?def\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
	// class Name:
	pyClassPattern = regexp.MustCompile(`(?m)^\s*class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*[:\(]`)
)

func (e *SymbolExtractor) extractPython(content string) []Symbol {
	var symbols []Symbol
	seen := make(map[string]bool)

	// Extract classes
	matches := pyClassPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "class"})
			seen[m[1]] = true
		}
	}

	// Extract functions (skip private/dunder methods)
	matches = pyFuncPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			name := m[1]
			// Skip dunder methods and private methods
			if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
				continue
			}
			if strings.HasPrefix(name, "_") {
				continue
			}
			symbols = append(symbols, Symbol{Name: name, Kind: "func"})
			seen[name] = true
		}
	}

	return symbols
}

// Java symbol patterns
var (
	// class Name { or class Name extends
	javaClassPattern = regexp.MustCompile(`(?m)^\s*(?:public\s+|private\s+|protected\s+)?(?:abstract\s+|final\s+)?class\s+([A-Z][a-zA-Z0-9_]*)\s*(?:extends|implements|\{)`)
	// interface Name {
	javaInterfacePattern = regexp.MustCompile(`(?m)^\s*(?:public\s+)?interface\s+([A-Z][a-zA-Z0-9_]*)\s*\{`)
	// Type name( (method signature)
	javaMethodPattern = regexp.MustCompile(`(?m)^\s*(?:public\s+|private\s+|protected\s+)?(?:static\s+)?(?:final\s+)?[a-zA-Z0-9_<>]+\s+([a-z][a-zA-Z0-9_]*)\s*\(`)
)

func (e *SymbolExtractor) extractJava(content string) []Symbol {
	var symbols []Symbol
	seen := make(map[string]bool)

	// Extract classes
	matches := javaClassPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "class"})
			seen[m[1]] = true
		}
	}

	// Extract interfaces
	matches = javaInterfacePattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "interface"})
			seen[m[1]] = true
		}
	}

	// Extract methods (public methods only, camelCase naming convention)
	matches = javaMethodPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			name := m[1]
			// Skip common false positives
			if name == "if" || name == "while" || name == "for" || name == "switch" {
				continue
			}
			symbols = append(symbols, Symbol{Name: name, Kind: "func"})
			seen[name] = true
		}
	}

	return symbols
}

// Rust symbol patterns
var (
	// fn name( or pub fn name( or async fn name(
	rustFnPattern = regexp.MustCompile(`(?m)^\s*(?:pub\s+)?(?:async\s+)?fn\s+([a-z_][a-zA-Z0-9_]*)\s*[<(]`)
	// struct Name { or struct Name;
	rustStructPattern = regexp.MustCompile(`(?m)^\s*(?:pub\s+)?struct\s+([A-Z][a-zA-Z0-9_]*)\s*[{\;]`)
	// enum Name {
	rustEnumPattern = regexp.MustCompile(`(?m)^\s*(?:pub\s+)?enum\s+([A-Z][a-zA-Z0-9_]*)\s*\{`)
	// trait Name {
	rustTraitPattern = regexp.MustCompile(`(?m)^\s*(?:pub\s+)?trait\s+([A-Z][a-zA-Z0-9_]*)\s*\{`)
	// const NAME:
	rustConstPattern = regexp.MustCompile(`(?m)^\s*(?:pub\s+)?const\s+([A-Z_][A-Z0-9_]*)\s*:`)
)

func (e *SymbolExtractor) extractRust(content string) []Symbol {
	var symbols []Symbol
	seen := make(map[string]bool)

	// Extract structs
	matches := rustStructPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "struct"})
			seen[m[1]] = true
		}
	}

	// Extract enums
	matches = rustEnumPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "enum"})
			seen[m[1]] = true
		}
	}

	// Extract traits
	matches = rustTraitPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "trait"})
			seen[m[1]] = true
		}
	}

	// Extract functions
	matches = rustFnPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "func"})
			seen[m[1]] = true
		}
	}

	// Extract constants
	matches = rustConstPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			symbols = append(symbols, Symbol{Name: m[1], Kind: "const"})
			seen[m[1]] = true
		}
	}

	return symbols
}
