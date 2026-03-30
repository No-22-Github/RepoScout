package heuristics

import (
	"testing"
)

func TestDefaultBasicRulesConfig(t *testing.T) {
	config := DefaultBasicRulesConfig()

	if len(config.TestPatterns) == 0 {
		t.Error("TestPatterns should not be empty")
	}
	if len(config.ConfigPatterns) == 0 {
		t.Error("ConfigPatterns should not be empty")
	}
	if len(config.ResourcePatterns) == 0 {
		t.Error("ResourcePatterns should not be empty")
	}
	if len(config.BuildPatterns) == 0 {
		t.Error("BuildPatterns should not be empty")
	}
	if len(config.FeatureFlagPatterns) == 0 {
		t.Error("FeatureFlagPatterns should not be empty")
	}

	if config.TestScore <= 0 || config.TestScore > 1 {
		t.Errorf("TestScore should be between 0 and 1, got %f", config.TestScore)
	}
	if config.ConfigScore <= 0 || config.ConfigScore > 1 {
		t.Errorf("ConfigScore should be between 0 and 1, got %f", config.ConfigScore)
	}
}

func TestBasicRuleEngine_ApplyRules_Tests(t *testing.T) {
	engine := NewBasicRuleEngine(nil)

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		{"file with _test.go suffix", "src/foo_test.go", true},
		{"file with _tests.cc suffix", "src/bar_tests.cc", true},
		{"file with test_ prefix", "src/test_foo.py", true},
		{"file in test directory", "src/test/runner.go", true},
		{"file in tests directory", "src/tests/main.go", true},
		{"file with _spec.js suffix", "src/component_spec.js", true},
		{"regular file", "src/main.go", false},
		{"production code with testing in filename", "src/testing_lib.go", false}, // testing in filename but not test pattern
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.ApplyRules(tc.path, []string{FocusCheckTests})
			hasTestTag := containsTag(result.Tags, TagTestFile)
			if hasTestTag != tc.expected {
				t.Errorf("expected test tag %v for %s, got %v (tags: %v)", tc.expected, tc.path, hasTestTag, result.Tags)
			}
		})
	}
}

func TestBasicRuleEngine_ApplyRules_Config(t *testing.T) {
	engine := NewBasicRuleEngine(nil)

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		{"config.json file", "config/config.json", true},
		{"default settings file", "src/default_settings.cc", true},
		{"prefs file", "browser/prefs/pref_service.cc", true},
		{"settings file", "src/settings/app_settings.go", true},
		{"YAML config", "config/app.yaml", true},
		{"regular file", "src/main.go", false},
		{"config test file (should not match)", "src/config_test.go", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.ApplyRules(tc.path, []string{FocusCheckDefaultConfig})
			hasConfigTag := containsTag(result.Tags, TagDefaultConfig)
			if hasConfigTag != tc.expected {
				t.Errorf("expected config tag %v for %s, got %v (tags: %v)", tc.expected, tc.path, hasConfigTag, result.Tags)
			}
		})
	}
}

func TestBasicRuleEngine_ApplyRules_Resources(t *testing.T) {
	engine := NewBasicRuleEngine(nil)

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		{"strings directory", "src/strings/en_strings.xtb", true},
		{"resources directory", "src/resources/icon.png", true},
		{"i18n directory", "src/i18n/messages.json", true},
		{"locale directory", "src/locale/en-US.json", true},
		{"grd file", "resources/app_resources.grd", true},
		{"grdp file", "resources/strings.grdp", true},
		{"xtb file", "resources/en.xtb", true},
		{"locale code in path", "src/locales/en-US/messages.json", true},
		{"regular file", "src/main.go", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.ApplyRules(tc.path, []string{FocusCheckResourcesOrStrings})
			hasResourceTag := containsTag(result.Tags, TagResourcesOrString)
			if hasResourceTag != tc.expected {
				t.Errorf("expected resource tag %v for %s, got %v (tags: %v)", tc.expected, tc.path, hasResourceTag, result.Tags)
			}
		})
	}
}

func TestBasicRuleEngine_ApplyRules_Build(t *testing.T) {
	engine := NewBasicRuleEngine(nil)

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		{"BUILD file", "src/BUILD", true},
		{"BUILD.gn file", "src/BUILD.gn", true},
		{"CMakeLists.txt file", "CMakeLists.txt", true},
		{"Makefile", "Makefile", true},
		{"gni file", "src/build/config.gni", true},
		{"meson.build file", "meson.build", true},
		{"regular file", "src/main.go", false},
		{"build in filename but not build file", "src/builder.go", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.ApplyRules(tc.path, []string{FocusCheckBuildRegistration})
			hasBuildTag := containsTag(result.Tags, TagBuildRegistration)
			if hasBuildTag != tc.expected {
				t.Errorf("expected build tag %v for %s, got %v (tags: %v)", tc.expected, tc.path, hasBuildTag, result.Tags)
			}
		})
	}
}

func TestBasicRuleEngine_ApplyRules_FeatureFlags(t *testing.T) {
	engine := NewBasicRuleEngine(nil)

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		{"feature_flag file", "src/feature_flag.cc", true},
		{"featureflag file", "src/featureflag.go", true},
		{"feature-flag file", "src/feature-flag.js", true},
		{"flags file", "src/flags/flags.go", true},
		{"experiment file", "src/experiment/handler.cc", true},
		{"feature_switch file", "src/feature_switch.py", true},
		{"regular file", "src/main.go", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.ApplyRules(tc.path, []string{FocusCheckFeatureFlag})
			hasFlagTag := containsTag(result.Tags, TagFeatureFlag)
			if hasFlagTag != tc.expected {
				t.Errorf("expected feature flag tag %v for %s, got %v (tags: %v)", tc.expected, tc.path, hasFlagTag, result.Tags)
			}
		})
	}
}

func TestBasicRuleEngine_ApplyRules_AllChecks(t *testing.T) {
	engine := NewBasicRuleEngine(nil)

	// When focusChecks is empty, all rules should be applied
	result := engine.ApplyRules("src/test/config_test.go", []string{})
	// Should match test rule (has _test.go)
	if !containsTag(result.Tags, TagTestFile) {
		t.Error("expected test tag when applying all rules")
	}
	// Should NOT match config because config test files are excluded
	if containsTag(result.Tags, TagDefaultConfig) {
		t.Error("should not match config tag for test file")
	}

	// Test with multiple matches
	result = engine.ApplyRules("src/feature_flag_test.cc", []string{})
	if !containsTag(result.Tags, TagTestFile) {
		t.Error("expected test tag")
	}
	if !containsTag(result.Tags, TagFeatureFlag) {
		t.Error("expected feature flag tag")
	}
}

func TestBasicRuleEngine_ApplyRules_UnknownChecksFallBackToAll(t *testing.T) {
	engine := NewBasicRuleEngine(nil)

	result := engine.ApplyRules("src/feature_flag_test.go", []string{"security"})
	if !containsTag(result.Tags, TagTestFile) {
		t.Fatalf("expected unknown checks to fall back to all rules, got tags %v", result.Tags)
	}
	if !containsTag(result.Tags, TagFeatureFlag) {
		t.Fatalf("expected feature_flag rule to still apply on fallback, got tags %v", result.Tags)
	}
}

func TestBasicRuleEngine_ApplyRules_ScoreAccumulation(t *testing.T) {
	engine := NewBasicRuleEngine(nil)

	// File that matches multiple rules should accumulate score
	result := engine.ApplyRules("src/test/feature_flag_test.go", []string{})
	if result.Score <= 0 {
		t.Errorf("expected positive score for multi-match file, got %f", result.Score)
	}

	// Score should be capped at 1.0
	// Create a config with high scores to test capping
	config := &BasicRulesConfig{
		TestPatterns:        DefaultBasicRulesConfig().TestPatterns,
		ConfigPatterns:      DefaultBasicRulesConfig().ConfigPatterns,
		ResourcePatterns:    DefaultBasicRulesConfig().ResourcePatterns,
		BuildPatterns:       DefaultBasicRulesConfig().BuildPatterns,
		FeatureFlagPatterns: DefaultBasicRulesConfig().FeatureFlagPatterns,
		TestScore:           0.5,
		ConfigScore:         0.5,
		ResourceScore:       0.5,
		BuildScore:          0.5,
		FeatureFlagScore:    0.5,
	}
	engine = NewBasicRuleEngine(config)
	result = engine.ApplyRules("src/test/feature_flag_test.go", []string{})
	if result.Score > 1.0 {
		t.Errorf("score should be capped at 1.0, got %f", result.Score)
	}
}

func TestBasicRuleEngine_ApplyRules_DiscoveredBy(t *testing.T) {
	engine := NewBasicRuleEngine(nil)

	result := engine.ApplyRules("src/foo_test.go", []string{FocusCheckTests})
	if !containsDiscoveredBy(result.DiscoveredBy, "test_rule") {
		t.Error("expected 'test_rule' in discovered_by")
	}

	result = engine.ApplyRules("src/config.json", []string{FocusCheckDefaultConfig})
	if !containsDiscoveredBy(result.DiscoveredBy, "config_rule") {
		t.Error("expected 'config_rule' in discovered_by")
	}

	// Test multiple discovery methods
	result = engine.ApplyRules("src/test/config_test.go", []string{})
	if !containsDiscoveredBy(result.DiscoveredBy, "test_rule") {
		t.Error("expected 'test_rule' in discovered_by")
	}
}

func TestApplyBasicRules(t *testing.T) {
	// Test the convenience function
	result := ApplyBasicRules("src/foo_test.go", []string{FocusCheckTests})
	if !containsTag(result.Tags, TagTestFile) {
		t.Error("expected test tag from convenience function")
	}
}

func TestApplyBasicRulesWithConfig(t *testing.T) {
	// Test custom configuration
	config := &BasicRulesConfig{
		TestPatterns: []string{"custom_test"},
		TestScore:    0.8,
	}
	result := ApplyBasicRulesWithConfig("src/custom_test.go", []string{FocusCheckTests}, config)
	if !containsTag(result.Tags, TagTestFile) {
		t.Error("expected test tag with custom config")
	}
	if result.Score < 0.8 {
		t.Errorf("expected score >= 0.8 with custom config, got %f", result.Score)
	}
}

func TestGetAllFocusChecks(t *testing.T) {
	checks := GetAllFocusChecks()
	if len(checks) != 5 {
		t.Errorf("expected 5 focus checks, got %d", len(checks))
	}

	expectedChecks := []string{
		FocusCheckTests,
		FocusCheckDefaultConfig,
		FocusCheckResourcesOrStrings,
		FocusCheckBuildRegistration,
		FocusCheckFeatureFlag,
	}

	for _, expected := range expectedChecks {
		found := false
		for _, check := range checks {
			if check == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected focus check %s in GetAllFocusChecks result", expected)
		}
	}
}

func TestGetTagsForFocusCheck(t *testing.T) {
	testCases := []struct {
		focusCheck string
		expected   string
	}{
		{FocusCheckTests, TagTestFile},
		{FocusCheckDefaultConfig, TagDefaultConfig},
		{FocusCheckResourcesOrStrings, TagResourcesOrString},
		{FocusCheckBuildRegistration, TagBuildRegistration},
		{FocusCheckFeatureFlag, TagFeatureFlag},
		{"unknown", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.focusCheck, func(t *testing.T) {
			tag := GetTagsForFocusCheck(tc.focusCheck)
			if tag != tc.expected {
				t.Errorf("expected tag %s for focus check %s, got %s", tc.expected, tc.focusCheck, tag)
			}
		})
	}
}

func TestIsLocaleCode(t *testing.T) {
	testCases := []struct {
		code     string
		expected bool
	}{
		{"en", true},
		{"zh", true},
		{"en-US", true},
		{"en_US", true},
		{"zh-CN", true},
		{"zh_CN", true},
		{"pt-BR", true},
		{"sr-Latn", true},
		{"config", false},
		{"src", false}, // excluded directory name
		{"lib", false}, // excluded directory name
		{"bin", false}, // excluded directory name
		{"en-US-extra", false},
		{"", false},
		{"e", false}, // too short for non-locale
	}

	for _, tc := range testCases {
		t.Run(tc.code, func(t *testing.T) {
			result := isLocaleCode(tc.code)
			if result != tc.expected {
				t.Errorf("isLocaleCode(%s) = %v, expected %v", tc.code, result, tc.expected)
			}
		})
	}
}

// Helper functions

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func containsDiscoveredBy(discoveredBy []string, method string) bool {
	for _, m := range discoveredBy {
		if m == method {
			return true
		}
	}
	return false
}
