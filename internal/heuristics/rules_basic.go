// Package heuristics provides heuristic rules for file analysis.
package heuristics

import (
	"path/filepath"
	"strings"
)

// FocusCheck constants represent the types of checks that can be performed.
const (
	FocusCheckTests              = "tests"
	FocusCheckDefaultConfig      = "default_config"
	FocusCheckResourcesOrStrings = "resources_or_strings"
	FocusCheckBuildRegistration  = "build_registration"
	FocusCheckFeatureFlag        = "feature_flag"
)

// HeuristicTag constants represent the tags that can be applied to files.
const (
	TagTestFile          = "test_file"
	TagDefaultConfig     = "default_config"
	TagResourcesOrString = "resources_or_strings"
	TagBuildRegistration = "build_registration"
	TagFeatureFlag       = "feature_flag"
)

// RuleResult represents the result of applying a heuristic rule.
type RuleResult struct {
	// Tags are the heuristic tags applied by the rule.
	Tags []string `json:"tags,omitempty"`

	// Score is the heuristic score for this file (0.0 to 1.0).
	Score float64 `json:"score,omitempty"`

	// DiscoveredBy records which rules matched this file.
	DiscoveredBy []string `json:"discovered_by,omitempty"`
}

// BasicRulesConfig holds configuration for basic heuristic rules.
type BasicRulesConfig struct {
	// TestPatterns are path patterns that indicate test files.
	TestPatterns []string

	// ConfigPatterns are path patterns that indicate configuration files.
	ConfigPatterns []string

	// ResourcePatterns are path patterns that indicate resource/string files.
	ResourcePatterns []string

	// BuildPatterns are path patterns that indicate build registration files.
	BuildPatterns []string

	// FeatureFlagPatterns are path patterns that indicate feature flag files.
	FeatureFlagPatterns []string

	// TestScore is the score added when a test pattern matches.
	TestScore float64

	// ConfigScore is the score added when a config pattern matches.
	ConfigScore float64

	// ResourceScore is the score added when a resource pattern matches.
	ResourceScore float64

	// BuildScore is the score added when a build pattern matches.
	BuildScore float64

	// FeatureFlagScore is the score added when a feature flag pattern matches.
	FeatureFlagScore float64
}

// DefaultBasicRulesConfig returns the default configuration for basic rules.
func DefaultBasicRulesConfig() *BasicRulesConfig {
	return &BasicRulesConfig{
		TestPatterns: []string{
			"_test.", "_tests.", "test_", "tests_",
			"_unittest.", "_unittests.", "unittest_", "unittests_",
			"_spec.", "_specs.", "spec_", "specs_",
			"/test/", "/tests/", "/testing/",
			"\\test\\", "\\tests\\", "\\testing\\",
		},
		ConfigPatterns: []string{
			"config", "default", "pref", "settings",
			"option", "property", "properties",
		},
		ResourcePatterns: []string{
			"strings", "resources", "i18n", "locale", "locales",
			"localization", "translations", "messages",
		},
		BuildPatterns: []string{
			"BUILD", "BUILD.gn", "BUILD.bazel", "CMakeLists",
			"Makefile", "CMakeLists.txt", "meson.build",
		},
		FeatureFlagPatterns: []string{
			"feature_flag", "featureflag", "feature-flag",
			"flags", "flag", "feature_switch", "featureswitch",
			"feature_switch", "experiment", "ab_test",
		},
		TestScore:        0.3,
		ConfigScore:      0.4,
		ResourceScore:    0.3,
		BuildScore:       0.35,
		FeatureFlagScore: 0.35,
	}
}

// BasicRuleEngine applies basic heuristic rules to file paths.
type BasicRuleEngine struct {
	config *BasicRulesConfig
}

// NewBasicRuleEngine creates a new BasicRuleEngine with the given configuration.
func NewBasicRuleEngine(config *BasicRulesConfig) *BasicRuleEngine {
	if config == nil {
		config = DefaultBasicRulesConfig()
	}
	return &BasicRuleEngine{config: config}
}

// ApplyRules applies all basic rules to a file path and returns the result.
// focusChecks filters which rules to apply. If empty, all rules are applied.
func (e *BasicRuleEngine) ApplyRules(filePath string, focusChecks []string) *RuleResult {
	result := &RuleResult{
		Tags:         []string{},
		DiscoveredBy: []string{},
	}

	// Normalize path
	normalizedPath := filepath.ToSlash(filePath)
	lowerPath := strings.ToLower(normalizedPath)

	// Determine which checks to apply
	applyAll := len(focusChecks) == 0
	checks := make(map[string]bool)
	for _, check := range focusChecks {
		checks[check] = true
	}

	// Apply test rules
	if applyAll || checks[FocusCheckTests] {
		if e.matchTestFile(lowerPath) {
			result.Tags = append(result.Tags, TagTestFile)
			result.DiscoveredBy = append(result.DiscoveredBy, "test_rule")
			result.Score += e.config.TestScore
		}
	}

	// Apply config rules
	if applyAll || checks[FocusCheckDefaultConfig] {
		if e.matchConfigFile(lowerPath, normalizedPath) {
			result.Tags = append(result.Tags, TagDefaultConfig)
			result.DiscoveredBy = append(result.DiscoveredBy, "config_rule")
			result.Score += e.config.ConfigScore
		}
	}

	// Apply resource/string rules
	if applyAll || checks[FocusCheckResourcesOrStrings] {
		if e.matchResourceFile(lowerPath) {
			result.Tags = append(result.Tags, TagResourcesOrString)
			result.DiscoveredBy = append(result.DiscoveredBy, "resource_rule")
			result.Score += e.config.ResourceScore
		}
	}

	// Apply build registration rules
	if applyAll || checks[FocusCheckBuildRegistration] {
		if e.matchBuildFile(normalizedPath) {
			result.Tags = append(result.Tags, TagBuildRegistration)
			result.DiscoveredBy = append(result.DiscoveredBy, "build_rule")
			result.Score += e.config.BuildScore
		}
	}

	// Apply feature flag rules
	if applyAll || checks[FocusCheckFeatureFlag] {
		if e.matchFeatureFlagFile(lowerPath) {
			result.Tags = append(result.Tags, TagFeatureFlag)
			result.DiscoveredBy = append(result.DiscoveredBy, "feature_flag_rule")
			result.Score += e.config.FeatureFlagScore
		}
	}

	// Cap score at 1.0
	if result.Score > 1.0 {
		result.Score = 1.0
	}

	return result
}

// matchTestFile checks if the file path matches test patterns.
func (e *BasicRuleEngine) matchTestFile(lowerPath string) bool {
	for _, pattern := range e.config.TestPatterns {
		if strings.Contains(lowerPath, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// matchConfigFile checks if the file path matches config patterns.
func (e *BasicRuleEngine) matchConfigFile(lowerPath, normalizedPath string) bool {
	// Check path components for config keywords
	parts := strings.Split(lowerPath, "/")
	for _, part := range parts {
		// Check if filename or directory contains config keywords
		base := strings.ToLower(strings.TrimSuffix(part, filepath.Ext(part)))
		for _, pattern := range e.config.ConfigPatterns {
			if strings.Contains(base, pattern) {
				// Avoid false positives like "configuration_manager_test"
				if strings.Contains(base, "test") {
					continue
				}
				return true
			}
		}
	}

	// Check for common config file extensions
	ext := strings.ToLower(filepath.Ext(normalizedPath))
	configExts := []string{".json", ".yaml", ".yml", ".toml", ".ini", ".conf", ".config"}
	for _, configExt := range configExts {
		if ext == configExt && strings.Contains(lowerPath, "config") {
			return true
		}
	}

	return false
}

// matchResourceFile checks if the file path matches resource/string patterns.
func (e *BasicRuleEngine) matchResourceFile(lowerPath string) bool {
	// Check for resource/string directories in path
	for _, pattern := range e.config.ResourcePatterns {
		if strings.Contains(lowerPath, "/"+pattern+"/") ||
			strings.Contains(lowerPath, "/"+pattern+"_") ||
			strings.Contains(lowerPath, "_"+pattern+"/") {
			return true
		}
	}

	// Check for common resource file patterns
	ext := strings.ToLower(filepath.Ext(lowerPath))
	resourceExts := []string{".grd", ".grdp", ".xtb", ".strings", ".po", ".mo"}
	for _, resExt := range resourceExts {
		if ext == resExt {
			return true
		}
	}

	// Check for locale patterns like en-US, zh-CN
	// Only check directory names that look like locale codes, not file extensions
	parts := strings.Split(lowerPath, "/")
	for i, part := range parts {
		// Skip the last part (filename) - locale codes should be in directory names
		if i < len(parts)-1 && isLocaleCode(part) {
			return true
		}
	}

	return false
}

// matchBuildFile checks if the file path matches build patterns.
func (e *BasicRuleEngine) matchBuildFile(normalizedPath string) bool {
	// Get the base filename
	base := filepath.Base(normalizedPath)

	// Check against build file patterns
	for _, pattern := range e.config.BuildPatterns {
		if base == pattern {
			return true
		}
	}

	// Check for GN build files (build.gn, BUILD.gn, *.gni)
	if strings.HasSuffix(base, ".gn") || strings.HasSuffix(base, ".gni") {
		return true
	}

	return false
}

// matchFeatureFlagFile checks if the file path matches feature flag patterns.
func (e *BasicRuleEngine) matchFeatureFlagFile(lowerPath string) bool {
	// Check for feature flag keywords in path components
	parts := strings.Split(lowerPath, "/")
	for _, part := range parts {
		base := strings.ToLower(strings.TrimSuffix(part, filepath.Ext(part)))
		for _, pattern := range e.config.FeatureFlagPatterns {
			if strings.Contains(base, pattern) {
				return true
			}
		}
	}

	return false
}

// isLocaleCode checks if a string looks like a locale code (e.g., "en-US", "zh_CN").
// It excludes common directory names that might match the pattern.
func isLocaleCode(s string) bool {
	// Common directory names that should NOT be treated as locale codes
	excludedNames := map[string]bool{
		"src":   true,
		"lib":   true,
		"bin":   true,
		"pkg":   true,
		"doc":   true,
		"docs":  true,
		"app":   true,
		"apps":  true,
		"web":   true,
		"tmp":   true,
		"opt":   true,
		"etc":   true,
		"var":   true,
		"usr":   true,
		"out":   true,
		"inc":   true,
		"test":  true,
		"util":  true,
		"utils": true,
		"tools": true,
		"core":  true,
		"base":  true,
		"main":  true,
		"cmd":   true,
		"api":   true,
	}

	s = strings.ToLower(s)
	if excludedNames[s] {
		return false
	}

	// Pattern: xx or xxx (language code)
	if len(s) == 2 || len(s) == 3 {
		for _, c := range s {
			if c < 'a' || c > 'z' {
				return false
			}
		}
		return true
	}

	// Pattern: xx-xx, xx_xx, xxx-xx, xxx_xx (language-region)
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_'
	})
	if len(parts) == 2 {
		for _, part := range parts {
			if len(part) < 2 || len(part) > 4 {
				return false
			}
			for _, c := range part {
				if c < 'a' || c > 'z' {
					return false
				}
			}
		}
		return true
	}

	return false
}

// ApplyBasicRules is a convenience function that applies basic rules using default configuration.
// It returns tags, score, and discovery methods for the given file path.
func ApplyBasicRules(filePath string, focusChecks []string) *RuleResult {
	return NewBasicRuleEngine(nil).ApplyRules(filePath, focusChecks)
}

// ApplyBasicRulesWithConfig applies basic rules using custom configuration.
func ApplyBasicRulesWithConfig(filePath string, focusChecks []string, config *BasicRulesConfig) *RuleResult {
	return NewBasicRuleEngine(config).ApplyRules(filePath, focusChecks)
}

// GetAllFocusChecks returns all valid focus check types.
func GetAllFocusChecks() []string {
	return []string{
		FocusCheckTests,
		FocusCheckDefaultConfig,
		FocusCheckResourcesOrStrings,
		FocusCheckBuildRegistration,
		FocusCheckFeatureFlag,
	}
}

// GetTagsForFocusCheck returns the heuristic tag corresponding to a focus check.
func GetTagsForFocusCheck(focusCheck string) string {
	switch focusCheck {
	case FocusCheckTests:
		return TagTestFile
	case FocusCheckDefaultConfig:
		return TagDefaultConfig
	case FocusCheckResourcesOrStrings:
		return TagResourcesOrString
	case FocusCheckBuildRegistration:
		return TagBuildRegistration
	case FocusCheckFeatureFlag:
		return TagFeatureFlag
	default:
		return ""
	}
}
