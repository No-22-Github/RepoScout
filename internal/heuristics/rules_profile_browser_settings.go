// Package heuristics provides heuristic rules for file analysis.
package heuristics

import (
	"path/filepath"
	"strings"
)

// Profile name constant for browser settings.
const ProfileBrowserSettings = "browser_settings"

// Browser settings profile-specific heuristic tags.
const (
	TagSettingsPage        = "settings_page"
	TagHandler             = "handler"
	TagPrefsRegistration   = "prefs_registration"
	TagBrowserSettingsTest = "browser_settings_test"
)

// BrowserSettingsProfileConfig holds configuration for browser settings profile rules.
type BrowserSettingsProfileConfig struct {
	// SettingsPagePatterns are path patterns that indicate settings page files.
	SettingsPagePatterns []string

	// HandlerPatterns are path patterns that indicate handler/controller files.
	HandlerPatterns []string

	// PrefsPatterns are path patterns that indicate prefs registration files.
	PrefsPatterns []string

	// TestPatterns are path patterns for browser settings test files.
	TestPatterns []string

	// Scores for each pattern type.
	SettingsPageScore float64
	HandlerScore      float64
	PrefsScore        float64
	TestScore         float64
}

// DefaultBrowserSettingsProfileConfig returns the default configuration for browser settings profile.
func DefaultBrowserSettingsProfileConfig() *BrowserSettingsProfileConfig {
	return &BrowserSettingsProfileConfig{
		SettingsPagePatterns: []string{
			// Settings page UI components
			"settings_page",
			"settings_ui",
			"settings_dialog",
			"settings_component",
			"settings_view",
			"settings_container",
			// Chrome-style settings
			"options_page",
			"options_ui",
			"preferences_page",
			"prefs_page",
			// File naming patterns
			"_page.tsx",
			"_page.jsx",
			"page.tsx",
			"page.jsx",
			"page.cc",
			"page.h",
			"page.mm",
		},
		HandlerPatterns: []string{
			// Handler/controller patterns
			"handler",
			"controller",
			"delegate",
			"manager",
			"service",
			"bridge",
			// Specific to settings
			"settings_handler",
			"prefs_handler",
			"options_handler",
			"pref_service",
			"settings_service",
			"settings_manager",
		},
		PrefsPatterns: []string{
			// Prefs registration patterns
			"prefs",
			"pref_register",
			"pref_registration",
			"pref_service",
			"pref_delegate",
			"default_pref",
			"pref_store",
			// Chrome-style prefs
			"pref_change",
			"pref_notifier",
			"pref_registry",
			// Configuration patterns
			"config_store",
			"config_service",
		},
		TestPatterns: []string{
			// Browser settings test patterns
			"settings_test",
			"settings_unittest",
			"settings_browsertest",
			"prefs_test",
			"prefs_unittest",
			"pref_test",
			"handler_test",
			"controller_test",
		},
		SettingsPageScore: 0.5,
		HandlerScore:      0.45,
		PrefsScore:        0.4,
		TestScore:         0.3,
	}
}

// BrowserSettingsProfileRuleEngine applies browser settings profile-specific rules.
type BrowserSettingsProfileRuleEngine struct {
	config *BrowserSettingsProfileConfig
}

// NewBrowserSettingsProfileRuleEngine creates a new BrowserSettingsProfileRuleEngine.
func NewBrowserSettingsProfileRuleEngine(config *BrowserSettingsProfileConfig) *BrowserSettingsProfileRuleEngine {
	if config == nil {
		config = DefaultBrowserSettingsProfileConfig()
	}
	return &BrowserSettingsProfileRuleEngine{config: config}
}

// ApplyRules applies browser settings profile rules to a file path.
func (e *BrowserSettingsProfileRuleEngine) ApplyRules(filePath string) *RuleResult {
	result := &RuleResult{
		Tags:         []string{},
		DiscoveredBy: []string{},
	}

	// Normalize path
	normalizedPath := filepath.ToSlash(filePath)
	lowerPath := strings.ToLower(normalizedPath)

	// Check for settings page
	if e.matchSettingsPage(lowerPath, normalizedPath) {
		result.Tags = append(result.Tags, TagSettingsPage)
		result.DiscoveredBy = append(result.DiscoveredBy, "browser_settings:settings_page")
		result.Score += e.config.SettingsPageScore
	}

	// Check for handler/controller
	if e.matchHandler(lowerPath, normalizedPath) {
		result.Tags = append(result.Tags, TagHandler)
		result.DiscoveredBy = append(result.DiscoveredBy, "browser_settings:handler")
		result.Score += e.config.HandlerScore
	}

	// Check for prefs registration
	if e.matchPrefs(lowerPath, normalizedPath) {
		result.Tags = append(result.Tags, TagPrefsRegistration)
		result.DiscoveredBy = append(result.DiscoveredBy, "browser_settings:prefs_registration")
		result.Score += e.config.PrefsScore
	}

	// Check for test files related to browser settings
	if e.matchTest(lowerPath, normalizedPath) {
		result.Tags = append(result.Tags, TagBrowserSettingsTest)
		result.DiscoveredBy = append(result.DiscoveredBy, "browser_settings:test")
		result.Score += e.config.TestScore
	}

	// Cap score at 1.0
	if result.Score > 1.0 {
		result.Score = 1.0
	}

	return result
}

// matchSettingsPage checks if the file matches settings page patterns.
func (e *BrowserSettingsProfileRuleEngine) matchSettingsPage(lowerPath, normalizedPath string) bool {
	// Must be in a settings/options/prefs context
	settingsContextPatterns := []string{
		"settings",
		"options",
		"preferences",
		"prefs",
	}
	inSettingsContext := false
	for _, ctx := range settingsContextPatterns {
		if strings.Contains(lowerPath, "/"+ctx+"/") ||
			strings.Contains(lowerPath, "/"+ctx+"_") ||
			strings.HasPrefix(lowerPath, ctx+"/") {
			inSettingsContext = true
			break
		}
	}

	if !inSettingsContext {
		return false
	}

	// Check for page patterns
	ext := strings.ToLower(filepath.Ext(normalizedPath))
	pageExtensions := map[string]bool{
		".tsx": true, ".jsx": true, ".ts": true, ".js": true,
		".cc": true, ".cpp": true, ".h": true, ".mm": true,
	}

	if !pageExtensions[ext] {
		return false
	}

	// Check filename for page patterns
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(normalizedPath), ext))
	for _, pattern := range e.config.SettingsPagePatterns {
		// Clean the pattern to remove extension if present
		cleanPattern := strings.TrimSuffix(pattern, filepath.Ext(pattern))
		if strings.Contains(base, strings.ToLower(cleanPattern)) {
			return true
		}
	}

	// Special case: files named "page.*" in settings context
	if base == "page" {
		return true
	}

	return false
}

// matchHandler checks if the file matches handler/controller patterns.
func (e *BrowserSettingsProfileRuleEngine) matchHandler(lowerPath, normalizedPath string) bool {
	// Must be in a settings/prefs context
	settingsContextPatterns := []string{
		"settings",
		"options",
		"preferences",
		"prefs",
		"config",
	}
	inSettingsContext := false
	for _, ctx := range settingsContextPatterns {
		if strings.Contains(lowerPath, "/"+ctx+"/") ||
			strings.Contains(lowerPath, ctx+"_handler") ||
			strings.Contains(lowerPath, ctx+"_controller") ||
			strings.Contains(lowerPath, ctx+"_delegate") ||
			strings.Contains(lowerPath, ctx+"_service") ||
			strings.Contains(lowerPath, ctx+"_manager") {
			inSettingsContext = true
			break
		}
	}

	if !inSettingsContext {
		return false
	}

	// Check source file extensions
	ext := strings.ToLower(filepath.Ext(normalizedPath))
	sourceExtensions := map[string]bool{
		".cc": true, ".cpp": true, ".h": true, ".mm": true,
		".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".go": true, ".py": true, ".java": true,
	}

	if !sourceExtensions[ext] {
		return false
	}

	// Check filename for handler patterns
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(normalizedPath), ext))
	for _, pattern := range e.config.HandlerPatterns {
		if strings.Contains(base, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check common handler suffixes
	handlerSuffixes := []string{"handler", "controller", "delegate", "service", "manager", "bridge"}
	for _, suffix := range handlerSuffixes {
		if strings.HasSuffix(base, "_"+suffix) || strings.HasSuffix(base, suffix) {
			return true
		}
	}

	return false
}

// matchPrefs checks if the file matches prefs registration patterns.
func (e *BrowserSettingsProfileRuleEngine) matchPrefs(lowerPath, normalizedPath string) bool {
	ext := strings.ToLower(filepath.Ext(normalizedPath))
	sourceExtensions := map[string]bool{
		".cc": true, ".cpp": true, ".h": true, ".mm": true,
		".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".go": true, ".py": true, ".java": true,
	}

	if !sourceExtensions[ext] {
		return false
	}

	// Check filename for prefs patterns
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(normalizedPath), ext))
	for _, pattern := range e.config.PrefsPatterns {
		if strings.Contains(base, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check path for prefs context
	prefsPathPatterns := []string{
		"/prefs/",
		"/pref/",
		"/preferences/",
		"/preference/",
	}
	for _, pattern := range prefsPathPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// matchTest checks if the file is a test file related to browser settings.
func (e *BrowserSettingsProfileRuleEngine) matchTest(lowerPath, normalizedPath string) bool {
	// Must have test indicator in path/filename
	testIndicators := []string{
		"_test.", "_tests.",
		"test_", "tests_",
		"_unittest.", "_unittests.",
		"_spec.", "_specs.",
		"_browsertest.", "_browsertests.",
	}

	hasTestIndicator := false
	for _, indicator := range testIndicators {
		if strings.Contains(lowerPath, indicator) {
			hasTestIndicator = true
			break
		}
	}

	if !hasTestIndicator {
		return false
	}

	// Must be related to settings/prefs
	settingsContextPatterns := []string{
		"settings",
		"options",
		"preferences",
		"prefs",
		"pref_",
		"config",
	}

	for _, ctx := range settingsContextPatterns {
		if strings.Contains(lowerPath, ctx) {
			return true
		}
	}

	return false
}

// ApplyBrowserSettingsProfileRules is a convenience function that applies
// browser settings profile rules using default configuration.
func ApplyBrowserSettingsProfileRules(filePath string) *RuleResult {
	return NewBrowserSettingsProfileRuleEngine(nil).ApplyRules(filePath)
}

// ApplyBrowserSettingsProfileRulesWithConfig applies browser settings profile
// rules using custom configuration.
func ApplyBrowserSettingsProfileRulesWithConfig(filePath string, config *BrowserSettingsProfileConfig) *RuleResult {
	return NewBrowserSettingsProfileRuleEngine(config).ApplyRules(filePath)
}

// GetBrowserSettingsProfileTags returns all browser settings profile-specific tags.
func GetBrowserSettingsProfileTags() []string {
	return []string{
		TagSettingsPage,
		TagHandler,
		TagPrefsRegistration,
		TagBrowserSettingsTest,
	}
}

// MatchesProfile checks if this rule engine applies to the given profile.
func MatchesProfile(profile string) bool {
	return strings.ToLower(profile) == ProfileBrowserSettings
}
