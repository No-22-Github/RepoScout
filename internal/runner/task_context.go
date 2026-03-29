package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/no22/repo-scout/internal/schema"
	"github.com/tiktoken-go/tokenizer"
)

const (
	contextReadLimit       = 256 * 1024
	snippetPaddingBefore   = 2
	snippetPaddingAfter    = 18
	maxSnippetSymbolHints  = 12
	maxOutlineImports      = 8
	maxOutlineSymbols      = 6
	maxStructuredBlockLine = 48
	minSnippetBudgetTokens = 48
	minOutlineBudgetTokens = 24
)

type snippetWindow struct {
	label string
	start int
	end   int
}

var (
	cl100kEncodingOnce sync.Once
	cl100kEncoding     tokenizer.Codec
)

func buildTaskContext(repoRoot string, card *schema.FileCard, focusSymbols []string, maxContextTokens int) string {
	if card == nil {
		return ""
	}

	sections := make([]string, 0, 3)

	if hints := buildStaticHints(card); hints != "" {
		sections = append(sections, hints)
	}

	remaining := maxContextTokens - estimateTokenCount(strings.Join(sections, "\n\n"))
	if maxContextTokens <= 0 {
		remaining = 0
	}

	if remaining <= 0 {
		return strings.TrimSpace(strings.Join(sections, "\n\n"))
	}

	if codeContext := buildCodeContext(repoRoot, card, focusSymbols, remaining); codeContext != "" {
		sections = append(sections, codeContext)
	}

	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func buildStaticHints(card *schema.FileCard) string {
	lines := make([]string, 0, 2)
	if len(card.DiscoveredBy) > 0 {
		lines = append(lines, "Discovered by: "+joinLimited(card.DiscoveredBy, 4))
	}
	if len(card.HeuristicTags) > 0 {
		lines = append(lines, "Heuristic tags: "+joinLimited(card.HeuristicTags, 4))
	}
	return strings.Join(lines, "\n")
}

func buildCodeContext(repoRoot string, card *schema.FileCard, focusSymbols []string, budgetTokens int) string {
	if repoRoot == "" || card == nil || card.Path == "" || budgetTokens <= 0 {
		return ""
	}

	lines, ok := readContextLines(repoRoot, card.Path)
	if !ok {
		return ""
	}

	parts := make([]string, 0, 2)
	usedTokens := 0

	if budgetTokens >= minOutlineBudgetTokens {
		if outline := buildFileOutline(lines, card, focusSymbols); outline != "" {
			outlineTokens := estimateTokenCount(outline)
			if outlineTokens <= budgetTokens {
				parts = append(parts, outline)
				usedTokens += outlineTokens
			}
		}
	}

	remaining := budgetTokens - usedTokens
	if remaining >= minSnippetBudgetTokens {
		if snippets := buildRelevantSnippetsFromLines(lines, card, focusSymbols, remaining); snippets != "" {
			parts = append(parts, snippets)
		}
	}

	return strings.Join(parts, "\n\n")
}

func readContextLines(repoRoot, filePath string) ([]string, bool) {
	if repoRoot == "" || filePath == "" {
		return nil, false
	}

	absPath := filepath.Join(repoRoot, filePath)
	data, err := os.ReadFile(absPath)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	if len(data) > contextReadLimit {
		data = data[:contextReadLimit]
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return nil, false
	}
	return lines, true
}

func buildFileOutline(lines []string, card *schema.FileCard, focusSymbols []string) string {
	if card == nil || len(lines) == 0 {
		return ""
	}

	outlineLines := []string{"File outline:"}

	if unit := detectCodeUnit(lines, card.Lang); unit != "" {
		outlineLines = append(outlineLines, unit)
	}

	if imports := extractImportHints(lines, card.Lang); len(imports) > 0 {
		outlineLines = append(outlineLines, "Imports: "+strings.Join(limitStrings(imports, maxOutlineImports), ", "))
	}

	if sigs := extractDeclarationHints(lines, card, focusSymbols); len(sigs) > 0 {
		outlineLines = append(outlineLines, "Relevant declarations:")
		for _, sig := range limitStrings(sigs, maxOutlineSymbols) {
			outlineLines = append(outlineLines, "- "+sig)
		}
	}

	if len(outlineLines) == 1 {
		return ""
	}

	return strings.Join(outlineLines, "\n")
}

func buildRelevantSnippetsFromLines(lines []string, card *schema.FileCard, focusSymbols []string, budgetTokens int) string {
	if card == nil || len(lines) == 0 || budgetTokens <= 0 {
		return ""
	}

	windows := collectSnippetWindows(lines, focusSymbols, card.Symbols)
	if len(windows) == 0 {
		windows = append(windows, snippetWindow{
			label: "file_excerpt",
			start: 0,
			end:   minInt(len(lines), snippetPaddingAfter+6),
		})
	}

	header := "Relevant code snippets:"
	parts := []string{header}
	usedTokens := estimateTokenCount(header)

	for _, window := range windows {
		remaining := budgetTokens - usedTokens
		if remaining <= 0 {
			break
		}

		block := renderSnippetWindow(window, lines, card.Lang)
		blockTokens := estimateTokenCount(block)
		if blockTokens <= remaining {
			parts = append(parts, block)
			usedTokens += blockTokens
			continue
		}

		// Last resort: shrink a single snippet window to fit the remaining budget.
		if shrunk := shrinkSnippetBlock(window, lines, card.Lang, remaining); shrunk != "" {
			parts = append(parts, shrunk)
			usedTokens += estimateTokenCount(shrunk)
			break
		}
	}

	if len(parts) == 1 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

func collectSnippetWindows(lines []string, focusSymbols, fileSymbols []string) []snippetWindow {
	symbols := prioritizedSymbols(focusSymbols, fileSymbols)
	windows := make([]snippetWindow, 0, len(symbols))

	for _, symbol := range symbols {
		window, ok := findSymbolWindow(lines, symbol)
		if !ok {
			continue
		}
		window.label = symbol
		if overlapsWindow(windows, window) {
			continue
		}
		windows = append(windows, window)
	}

	return windows
}

func prioritizedSymbols(focusSymbols, fileSymbols []string) []string {
	seen := make(map[string]bool)
	symbols := make([]string, 0, maxSnippetSymbolHints)

	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		symbols = append(symbols, value)
	}

	for _, symbol := range focusSymbols {
		add(symbol)
	}
	for _, symbol := range fileSymbols {
		if len(symbols) >= maxSnippetSymbolHints {
			break
		}
		add(symbol)
	}

	return symbols
}

func findSymbolWindow(lines []string, symbol string) (snippetWindow, bool) {
	lineIndex := findSymbolDeclarationLine(lines, symbol)
	if lineIndex < 0 {
		lineIndex = findSymbolLine(lines, symbol)
	}
	if lineIndex < 0 {
		return snippetWindow{}, false
	}

	if block, ok := extractStructuredBlockWindow(lines, lineIndex); ok {
		block.start = maxInt(0, block.start-snippetPaddingBefore)
		block.end = minInt(len(lines), block.end+1)
		return block, true
	}

	window := snippetWindow{
		start: maxInt(0, lineIndex-snippetPaddingBefore),
		end:   minInt(len(lines), lineIndex+snippetPaddingAfter),
	}
	window = expandWindowToCodeBoundary(lines, window)
	return window, true
}

func findSymbolDeclarationLine(lines []string, symbol string) int {
	if symbol == "" {
		return -1
	}

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\bfunc\b[^{\n]*\b` + regexp.QuoteMeta(symbol) + `\b`),
		regexp.MustCompile(`\b(type|class|struct|interface|enum|trait)\b[^{\n]*\b` + regexp.QuoteMeta(symbol) + `\b`),
		regexp.MustCompile(`\b(const|var|let)\b[^{\n=]*\b` + regexp.QuoteMeta(symbol) + `\b`),
		regexp.MustCompile(`\b(def|fn)\b[^{\n]*\b` + regexp.QuoteMeta(symbol) + `\b`),
	}
	for i, line := range lines {
		for _, pattern := range patterns {
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

	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(symbol) + `\b`)
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

func extractStructuredBlockWindow(lines []string, lineIndex int) (snippetWindow, bool) {
	if lineIndex < 0 || lineIndex >= len(lines) {
		return snippetWindow{}, false
	}

	if block, ok := extractBraceBlockWindow(lines, lineIndex); ok {
		return block, true
	}
	if block, ok := extractIndentedBlockWindow(lines, lineIndex); ok {
		return block, true
	}
	return snippetWindow{}, false
}

func extractBraceBlockWindow(lines []string, lineIndex int) (snippetWindow, bool) {
	start := lineIndex
	openSeen := false
	balance := 0

	for i := lineIndex; i < len(lines) && i < lineIndex+maxStructuredBlockLine; i++ {
		line := lines[i]
		balance += strings.Count(line, "{")
		if strings.Contains(line, "{") {
			openSeen = true
		}
		balance -= strings.Count(line, "}")
		if openSeen && balance <= 0 {
			return snippetWindow{start: start, end: i + 1}, true
		}
	}

	return snippetWindow{}, false
}

func extractIndentedBlockWindow(lines []string, lineIndex int) (snippetWindow, bool) {
	declLine := strings.TrimSpace(lines[lineIndex])
	if !strings.HasSuffix(declLine, ":") {
		return snippetWindow{}, false
	}

	baseIndent := leadingWhitespace(lines[lineIndex])
	bodyStart := -1
	for i := lineIndex + 1; i < len(lines) && i < lineIndex+maxStructuredBlockLine; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if leadingWhitespace(lines[i]) <= baseIndent {
			return snippetWindow{}, false
		}
		bodyStart = i
		break
	}
	if bodyStart < 0 {
		return snippetWindow{}, false
	}

	end := bodyStart + 1
	for end < len(lines) && end < lineIndex+maxStructuredBlockLine {
		trimmed := strings.TrimSpace(lines[end])
		if trimmed == "" {
			end++
			continue
		}
		if leadingWhitespace(lines[end]) <= baseIndent {
			break
		}
		end++
	}

	return snippetWindow{start: lineIndex, end: end}, true
}

func expandWindowToCodeBoundary(lines []string, window snippetWindow) snippetWindow {
	for window.start > 0 && strings.TrimSpace(lines[window.start-1]) != "" {
		window.start--
	}

	for window.end < len(lines) && strings.TrimSpace(lines[window.end]) != "" {
		window.end++
	}

	window.start = maxInt(0, window.start)
	window.end = minInt(len(lines), window.end)
	return window
}

func overlapsWindow(existing []snippetWindow, candidate snippetWindow) bool {
	for _, window := range existing {
		if candidate.start < window.end && candidate.end > window.start {
			return true
		}
	}
	return false
}

func renderSnippetWindow(window snippetWindow, lines []string, lang string) string {
	body := strings.Join(trimBlankEdgeLines(lines[window.start:window.end]), "\n")
	if body == "" {
		return ""
	}

	if lang != "" {
		return fmt.Sprintf("[%s]\n```%s\n%s\n```", window.label, lang, body)
	}
	return fmt.Sprintf("[%s]\n```\n%s\n```", window.label, body)
}

func shrinkSnippetBlock(window snippetWindow, lines []string, lang string, budgetTokens int) string {
	if budgetTokens <= 0 {
		return ""
	}

	for end := window.end; end > window.start+3; end-- {
		block := renderSnippetWindow(snippetWindow{
			label: window.label,
			start: window.start,
			end:   end,
		}, lines, lang)
		if block != "" && estimateTokenCount(block) <= budgetTokens {
			return block
		}
	}

	return ""
}

func trimBlankEdgeLines(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	return lines[start:end]
}

func extractDeclarationHints(lines []string, card *schema.FileCard, focusSymbols []string) []string {
	symbols := prioritizedSymbols(focusSymbols, card.Symbols)
	hints := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		lineIndex := findSymbolDeclarationLine(lines, symbol)
		if lineIndex < 0 {
			continue
		}
		line := strings.TrimSpace(lines[lineIndex])
		if line == "" {
			continue
		}
		hints = append(hints, collapseWhitespace(line))
	}
	return dedupeStrings(hints)
}

func detectCodeUnit(lines []string, lang string) string {
	switch lang {
	case "go":
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "package ") {
				return "Package: " + strings.TrimSpace(strings.TrimPrefix(trimmed, "package "))
			}
		}
	case "py":
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "class ") || strings.HasPrefix(trimmed, "def ") {
				return "Entrypoint: " + collapseWhitespace(trimmed)
			}
		}
	}
	return ""
}

func extractImportHints(lines []string, lang string) []string {
	switch lang {
	case "go":
		return extractGoImports(lines)
	case "js", "jsx", "ts", "tsx":
		return extractMatchingLines(lines, func(trimmed string) bool {
			return strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "export ") && strings.Contains(trimmed, " from ")
		}, maxOutlineImports)
	case "py":
		return extractMatchingLines(lines, func(trimmed string) bool {
			return strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ")
		}, maxOutlineImports)
	case "java":
		return extractMatchingLines(lines, func(trimmed string) bool {
			return strings.HasPrefix(trimmed, "import ")
		}, maxOutlineImports)
	case "rust":
		return extractMatchingLines(lines, func(trimmed string) bool {
			return strings.HasPrefix(trimmed, "use ")
		}, maxOutlineImports)
	case "c", "cpp", "h", "hpp":
		return extractMatchingLines(lines, func(trimmed string) bool {
			return strings.HasPrefix(trimmed, "#include ")
		}, maxOutlineImports)
	default:
		return nil
	}
}

func extractGoImports(lines []string) []string {
	imports := make([]string, 0, maxOutlineImports)
	inBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "import (":
			inBlock = true
		case inBlock && trimmed == ")":
			inBlock = false
		case inBlock:
			if imp := extractQuotedValue(trimmed); imp != "" {
				imports = append(imports, imp)
			}
		case strings.HasPrefix(trimmed, "import "):
			if imp := extractQuotedValue(trimmed); imp != "" {
				imports = append(imports, imp)
			}
		}
		if len(imports) >= maxOutlineImports {
			break
		}
	}

	return dedupeStrings(imports)
}

func extractMatchingLines(lines []string, match func(string) bool, limit int) []string {
	result := make([]string, 0, limit)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !match(trimmed) {
			continue
		}
		result = append(result, collapseWhitespace(trimmed))
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return dedupeStrings(result)
}

func extractQuotedValue(line string) string {
	start := strings.IndexByte(line, '"')
	end := strings.LastIndexByte(line, '"')
	if start >= 0 && end > start {
		return line[start+1 : end]
	}
	return ""
}

func leadingWhitespace(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' || ch == '\t' {
			count++
			continue
		}
		break
	}
	return count
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func limitStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func estimateTokenCount(text string) int {
	if text == "" {
		return 0
	}

	if enc := getTokenEncoder(); enc != nil {
		ids, _, err := enc.Encode(text)
		if err == nil {
			return len(ids)
		}
	}

	return estimateTokenCountFallback(text)
}

func getTokenEncoder() tokenizer.Codec {
	cl100kEncodingOnce.Do(func() {
		enc, err := tokenizer.Get(tokenizer.Cl100kBase)
		if err == nil {
			cl100kEncoding = enc
		}
	})
	return cl100kEncoding
}

func estimateTokenCountFallback(text string) int {
	if text == "" {
		return 0
	}

	runeCount := utf8.RuneCountInString(text)
	if runeCount <= 0 {
		return 0
	}

	tokens := runeCount / 4
	if runeCount%4 != 0 {
		tokens++
	}
	if tokens == 0 {
		return 1
	}
	return tokens
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
