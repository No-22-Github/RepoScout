// Package cli provides common utilities for the reposcout CLI.
package cli

import (
	"fmt"
	"os"
	"strings"
)

// ColorEnabled controls whether color output is enabled.
// It can be disabled via --no-color flag or NO_COLOR environment variable.
var ColorEnabled = true

func init() {
	// Respect NO_COLOR environment variable (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		ColorEnabled = false
	}
	// Disable color if output is not a terminal
	if !isTerminal(os.Stdout) {
		ColorEnabled = false
	}
}

// isTerminal checks if the file descriptor is a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Color codes.
const (
	reset   = "\033[0m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	gray    = "\033[90m"
	bold    = "\033[1m"
)

// ColorFunc is a function that wraps text with ANSI color codes.
type ColorFunc func(string) string

// Color functions.
var (
	// Red wraps text in red.
	Red ColorFunc = color(red)
	// Green wraps text in green.
	Green ColorFunc = color(green)
	// Yellow wraps text in yellow.
	Yellow ColorFunc = color(yellow)
	// Blue wraps text in blue.
	Blue ColorFunc = color(blue)
	// Magenta wraps text in magenta.
	Magenta ColorFunc = color(magenta)
	// Cyan wraps text in cyan.
	Cyan ColorFunc = color(cyan)
	// Gray wraps text in gray.
	Gray ColorFunc = color(gray)
	// Bold wraps text in bold.
	Bold ColorFunc = color(bold)
)

// color returns a ColorFunc for the given ANSI code.
func color(code string) ColorFunc {
	return func(s string) string {
		if !ColorEnabled {
			return s
		}
		return code + s + reset
	}
}

// HighlightJSON adds syntax highlighting to JSON text.
func HighlightJSON(json string) string {
	if !ColorEnabled {
		return json
	}

	// Simple syntax highlighting: color keys, strings, numbers, booleans, null
	var result strings.Builder
	inString := false
	escape := false

	for _, ch := range json {
		if escape {
			escape = false
			result.WriteRune(ch)
			continue
		}

		switch ch {
		case '\\':
			escape = true
			result.WriteRune(ch)
		case '"':
			if inString {
				inString = false
				result.WriteString(Green(`"`))
			} else {
				inString = true
				result.WriteString(Green(`"`))
			}
		default:
			if inString {
				result.WriteRune(ch)
			} else {
				result.WriteRune(ch)
			}
		}
	}

	return result.String()
}

// PrintError prints an error message in red to stderr.
func PrintError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", Red("error:"), msg)
}

// PrintWarning prints a warning message in yellow to stderr.
func PrintWarning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", Yellow("warning:"), msg)
}

// PrintSuccess prints a success message in green to stdout.
func PrintSuccess(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "%s\n", Green(msg))
}

// PrintInfo prints an info message in cyan to stdout.
func PrintInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "%s\n", Cyan(msg))
}
