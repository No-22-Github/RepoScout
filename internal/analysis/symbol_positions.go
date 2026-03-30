package analysis

import (
	"regexp"
	"strings"
	"sync"
)

var symbolPatternCache sync.Map // key: symbol string -> []*regexp.Regexp

func getSymbolPatterns(symbol string) []*regexp.Regexp {
	if v, ok := symbolPatternCache.Load(symbol); ok {
		return v.([]*regexp.Regexp)
	}
	q := regexp.QuoteMeta(symbol)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\bfunc\b[^{\n]*\b` + q + `\b`),
		regexp.MustCompile(`\b(type|class|struct|interface|enum|trait)\b[^{\n]*\b` + q + `\b`),
		regexp.MustCompile(`\b(const|var|let)\b[^{\n=]*\b` + q + `\b`),
		regexp.MustCompile(`\b(def|fn)\b[^{\n]*\b` + q + `\b`),
		regexp.MustCompile(`\b` + q + `\b`),
	}
	symbolPatternCache.Store(symbol, patterns)
	return patterns
}

// PrecomputeSymbolLines scans the file once during static analysis and caches
// declaration line numbers for the provided symbols. Missing symbols are cached
// as -1 to avoid repeated rescans later during rerank.
func (s *SourceIndex) PrecomputeSymbolLines(relPath, lang string, symbols []string) {
	if s == nil || relPath == "" || len(symbols) == 0 {
		return
	}

	lines, ok := s.Lines(relPath, 0)
	if !ok {
		return
	}

	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		if _, ok := s.cachedSymbolLine(relPath, lang, symbol); ok {
			continue
		}

		line := findSymbolDeclarationLine(lines, symbol)
		if line < 0 {
			line = findSymbolLine(lines, symbol)
		}
		s.storeSymbolLine(relPath, lang, symbol, line)
	}
}

// SymbolLine returns the cached or lazily computed line number for a symbol.
func (s *SourceIndex) SymbolLine(relPath, lang, symbol string) (int, bool) {
	if s == nil || relPath == "" || symbol == "" {
		return 0, false
	}

	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return 0, false
	}

	if line, ok := s.cachedSymbolLine(relPath, lang, symbol); ok {
		return line, line >= 0
	}

	lines, ok := s.Lines(relPath, 0)
	if !ok {
		return 0, false
	}

	line := findSymbolDeclarationLine(lines, symbol)
	if line < 0 {
		line = findSymbolLine(lines, symbol)
	}
	s.storeSymbolLine(relPath, lang, symbol, line)
	return line, line >= 0
}

func (s *SourceIndex) cachedSymbolLine(relPath, lang, symbol string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	byPath, ok := s.symbolLines[relPath]
	if !ok {
		return 0, false
	}
	byLang, ok := byPath[lang]
	if !ok {
		return 0, false
	}
	line, ok := byLang[symbol]
	return line, ok
}

func (s *SourceIndex) storeSymbolLine(relPath, lang, symbol string, line int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	byPath, ok := s.symbolLines[relPath]
	if !ok {
		byPath = make(map[string]map[string]int)
		s.symbolLines[relPath] = byPath
	}
	byLang, ok := byPath[lang]
	if !ok {
		byLang = make(map[string]int)
		byPath[lang] = byLang
	}
	byLang[symbol] = line
}

func findSymbolDeclarationLine(lines []string, symbol string) int {
	if symbol == "" {
		return -1
	}
	patterns := getSymbolPatterns(symbol)
	for i, line := range lines {
		for _, pattern := range patterns[:4] {
			if pattern.MatchString(line) {
				return i
			}
		}
	}
	return -1
}

func findSymbolLine(lines []string, symbol string) int {
	if symbol == "" {
		return -1
	}

	pattern := getSymbolPatterns(symbol)[4]
	for i, line := range lines {
		if pattern.MatchString(line) {
			return i
		}
	}

	lowerSymbol := strings.ToLower(symbol)
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), lowerSymbol) {
			return i
		}
	}

	return -1
}
