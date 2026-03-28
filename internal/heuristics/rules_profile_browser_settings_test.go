package heuristics

import (
	"testing"
)

func TestDefaultBrowserSettingsProfileConfig(t *testing.T) {
	config := DefaultBrowserSettingsProfileConfig()

	if len(config.SettingsPagePatterns) == 0 {
		t.Error("SettingsPagePatterns should not be empty")
	}
	if len(config.HandlerPatterns) == 0 {
		t.Error("HandlerPatterns should not be empty")
	}
	if len(config.PrefsPatterns) == 0 {
		t.Error("PrefsPatterns should not be empty")
	}
	if len(config.TestPatterns) == 0 {
		t.Error("TestPatterns should not be empty")
	}

	if config.SettingsPageScore <= 0 || config.SettingsPageScore > 1 {
		t.Errorf("SettingsPageScore should be between 0 and 1, got %f", config.SettingsPageScore)
	}
	if config.HandlerScore <= 0 || config.HandlerScore > 1 {
		t.Errorf("HandlerScore should be between 0 and 1, got %f", config.HandlerScore)
	}
	if config.PrefsScore <= 0 || config.PrefsScore > 1 {
		t.Errorf("PrefsScore should be between 0 and 1, got %f", config.PrefsScore)
	}
	if config.TestScore <= 0 || config.TestScore > 1 {
		t.Errorf("TestScore should be between 0 and 1, got %f", config.TestScore)
	}
}

func TestBrowserSettingsProfileRuleEngine_SettingsPage(t *testing.T) {
	engine := NewBrowserSettingsProfileRuleEngine(nil)

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		// Settings page in settings directory
		{"settings page tsx", "src/browser/settings/foo_page.tsx", true},
		{"settings page jsx", "src/browser/settings/page.jsx", true},
		{"settings page cc", "src/browser/settings/settings_page.cc", true},
		{"settings ui component", "src/browser/settings/settings_ui.tsx", true},
		{"settings dialog", "src/options/settings_dialog.tsx", true},
		{"settings view", "src/settings/settings_view.tsx", true},
		// Options page variants
		{"options page", "src/options/options_page.tsx", true},
		{"preferences page", "src/preferences/prefs_page.tsx", true},
		// Not a settings page
		{"regular component", "src/components/button.tsx", false},
		{"page outside settings", "src/views/page.tsx", false},
		{"non-page file in settings", "src/browser/settings/utils.ts", false},
		{"handler file", "src/browser/settings/foo_handler.cc", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.ApplyRules(tc.path)
			hasSettingsPageTag := containsTag(result.Tags, TagSettingsPage)
			if hasSettingsPageTag != tc.expected {
				t.Errorf("expected settings_page tag %v for %s, got %v (tags: %v)", tc.expected, tc.path, hasSettingsPageTag, result.Tags)
			}
		})
	}
}

func TestBrowserSettingsProfileRuleEngine_Handler(t *testing.T) {
	engine := NewBrowserSettingsProfileRuleEngine(nil)

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		// Handler patterns
		{"settings handler", "src/browser/settings/foo_handler.cc", true},
		{"settings controller", "src/browser/settings/settings_controller.ts", true},
		{"prefs handler", "src/prefs/prefs_handler.cc", true},
		{"pref service", "src/browser/prefs/pref_service.cc", true},
		{"settings delegate", "src/settings/settings_delegate.mm", true},
		{"settings manager", "src/options/settings_manager.ts", true},
		{"settings bridge", "src/settings/settings_bridge.cc", true},
		{"config service", "src/config/config_service.go", true},
		// Not a handler
		{"regular file", "src/main.cc", false},
		{"handler outside settings", "src/api/handler.cc", false},
		{"settings page", "src/browser/settings/foo_page.tsx", false},
		{"test file", "src/browser/settings/foo_handler_test.cc", true}, // has handler in name
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.ApplyRules(tc.path)
			hasHandlerTag := containsTag(result.Tags, TagHandler)
			if hasHandlerTag != tc.expected {
				t.Errorf("expected handler tag %v for %s, got %v (tags: %v)", tc.expected, tc.path, hasHandlerTag, result.Tags)
			}
		})
	}
}

func TestBrowserSettingsProfileRuleEngine_PrefsRegistration(t *testing.T) {
	engine := NewBrowserSettingsProfileRuleEngine(nil)

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		// Prefs registration patterns
		{"prefs registration", "src/browser/prefs/pref_registration.cc", true},
		{"pref service", "src/prefs/pref_service.cc", true},
		{"pref delegate", "src/browser/prefs/pref_delegate.h", true},
		{"default prefs", "src/browser/prefs/default_prefs.cc", true},
		{"pref store", "src/prefs/pref_store.cc", true},
		{"pref registry", "src/browser/prefs/pref_registry.cc", true},
		{"prefs in path", "src/prefs/foo.cc", true},
		{"preferences in path", "src/preferences/bar.cc", true},
		// Not a prefs file
		{"regular file", "src/main.cc", false},
		{"config json", "src/config.json", false}, // not source file
		{"settings page", "src/browser/settings/foo_page.tsx", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.ApplyRules(tc.path)
			hasPrefsTag := containsTag(result.Tags, TagPrefsRegistration)
			if hasPrefsTag != tc.expected {
				t.Errorf("expected prefs_registration tag %v for %s, got %v (tags: %v)", tc.expected, tc.path, hasPrefsTag, result.Tags)
			}
		})
	}
}

func TestBrowserSettingsProfileRuleEngine_Test(t *testing.T) {
	engine := NewBrowserSettingsProfileRuleEngine(nil)

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		// Test files related to settings
		{"settings test", "src/browser/settings/foo_test.cc", true},
		{"settings unittest", "src/browser/settings/settings_unittest.cc", true},
		{"settings browsertest", "src/browser/settings/settings_browsertest.cc", true},
		{"prefs test", "src/prefs/prefs_test.cc", true},
		{"pref test", "src/browser/prefs/pref_test.cc", true},
		{"handler test", "src/browser/settings/handler_test.cc", true},
		{"config test", "src/config/config_test.go", true},
		// Not a settings test
		{"regular test", "src/foo_test.cc", false},
		{"unrelated test", "src/api/handler_test.cc", false},
		{"settings non-test", "src/browser/settings/foo.cc", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.ApplyRules(tc.path)
			hasTestTag := containsTag(result.Tags, TagBrowserSettingsTest)
			if hasTestTag != tc.expected {
				t.Errorf("expected browser_settings_test tag %v for %s, got %v (tags: %v)", tc.expected, tc.path, hasTestTag, result.Tags)
			}
		})
	}
}

func TestBrowserSettingsProfileRuleEngine_MultipleMatches(t *testing.T) {
	engine := NewBrowserSettingsProfileRuleEngine(nil)

	// File that matches multiple patterns
	result := engine.ApplyRules("src/browser/settings/settings_handler_test.cc")

	// Should have handler and test tags
	if !containsTag(result.Tags, TagHandler) {
		t.Error("expected handler tag")
	}
	if !containsTag(result.Tags, TagBrowserSettingsTest) {
		t.Error("expected browser_settings_test tag")
	}

	// Score should be accumulated
	if result.Score <= 0 {
		t.Errorf("expected positive score, got %f", result.Score)
	}

	// Should have multiple discovered_by entries
	if len(result.DiscoveredBy) < 2 {
		t.Errorf("expected at least 2 discovered_by entries, got %d: %v", len(result.DiscoveredBy), result.DiscoveredBy)
	}
}

func TestBrowserSettingsProfileRuleEngine_ScoreCapping(t *testing.T) {
	// Create config with high scores to test capping
	config := &BrowserSettingsProfileConfig{
		SettingsPagePatterns: DefaultBrowserSettingsProfileConfig().SettingsPagePatterns,
		HandlerPatterns:      DefaultBrowserSettingsProfileConfig().HandlerPatterns,
		PrefsPatterns:        DefaultBrowserSettingsProfileConfig().PrefsPatterns,
		TestPatterns:         DefaultBrowserSettingsProfileConfig().TestPatterns,
		SettingsPageScore:    0.5,
		HandlerScore:         0.5,
		PrefsScore:           0.5,
		TestScore:            0.5,
	}
	engine := NewBrowserSettingsProfileRuleEngine(config)

	// File that matches multiple patterns
	result := engine.ApplyRules("src/browser/settings/settings_handler_test.cc")

	// Score should be capped at 1.0
	if result.Score > 1.0 {
		t.Errorf("score should be capped at 1.0, got %f", result.Score)
	}
}

func TestApplyBrowserSettingsProfileRules(t *testing.T) {
	// Test the convenience function
	result := ApplyBrowserSettingsProfileRules("src/browser/settings/foo_handler.cc")
	if !containsTag(result.Tags, TagHandler) {
		t.Error("expected handler tag from convenience function")
	}
}

func TestApplyBrowserSettingsProfileRulesWithConfig(t *testing.T) {
	// Test custom configuration
	config := &BrowserSettingsProfileConfig{
		HandlerPatterns:   []string{"custom_handler"},
		HandlerScore:      0.9,
		SettingsPageScore: 0.0,
		PrefsScore:        0.0,
		TestScore:         0.0,
	}
	// Path must be in settings context to match handler patterns
	result := ApplyBrowserSettingsProfileRulesWithConfig("src/settings/custom_handler.cc", config)
	if !containsTag(result.Tags, TagHandler) {
		t.Error("expected handler tag with custom config")
	}
	if result.Score < 0.9 {
		t.Errorf("expected score >= 0.9 with custom config, got %f", result.Score)
	}
}

func TestGetBrowserSettingsProfileTags(t *testing.T) {
	tags := GetBrowserSettingsProfileTags()
	if len(tags) != 4 {
		t.Errorf("expected 4 browser settings profile tags, got %d", len(tags))
	}

	expectedTags := []string{
		TagSettingsPage,
		TagHandler,
		TagPrefsRegistration,
		TagBrowserSettingsTest,
	}

	for _, expected := range expectedTags {
		found := false
		for _, tag := range tags {
			if tag == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tag %s in GetBrowserSettingsProfileTags result", expected)
		}
	}
}

func TestMatchesProfile(t *testing.T) {
	testCases := []struct {
		profile  string
		expected bool
	}{
		{"browser_settings", true},
		{"BROWSER_SETTINGS", true},
		{"Browser_Settings", true},
		{"browser-settings", false},
		{"other_profile", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.profile, func(t *testing.T) {
			result := MatchesProfile(tc.profile)
			if result != tc.expected {
				t.Errorf("MatchesProfile(%s) = %v, expected %v", tc.profile, result, tc.expected)
			}
		})
	}
}

func TestBrowserSettingsProfileRuleEngine_DiscoveredBy(t *testing.T) {
	engine := NewBrowserSettingsProfileRuleEngine(nil)

	// Test settings page
	result := engine.ApplyRules("src/browser/settings/foo_page.tsx")
	if !containsDiscoveredBy(result.DiscoveredBy, "browser_settings:settings_page") {
		t.Error("expected 'browser_settings:settings_page' in discovered_by")
	}

	// Test handler
	result = engine.ApplyRules("src/browser/settings/foo_handler.cc")
	if !containsDiscoveredBy(result.DiscoveredBy, "browser_settings:handler") {
		t.Error("expected 'browser_settings:handler' in discovered_by")
	}

	// Test prefs
	result = engine.ApplyRules("src/prefs/pref_service.cc")
	if !containsDiscoveredBy(result.DiscoveredBy, "browser_settings:prefs_registration") {
		t.Error("expected 'browser_settings:prefs_registration' in discovered_by")
	}

	// Test test file
	result = engine.ApplyRules("src/browser/settings/foo_test.cc")
	if !containsDiscoveredBy(result.DiscoveredBy, "browser_settings:test") {
		t.Error("expected 'browser_settings:test' in discovered_by")
	}
}

func TestBrowserSettingsProfileRuleEngine_NoMatch(t *testing.T) {
	engine := NewBrowserSettingsProfileRuleEngine(nil)

	// File that doesn't match any browser settings patterns
	result := engine.ApplyRules("src/api/user_service.go")

	if len(result.Tags) != 0 {
		t.Errorf("expected no tags for non-matching file, got %v", result.Tags)
	}
	if len(result.DiscoveredBy) != 0 {
		t.Errorf("expected no discovered_by for non-matching file, got %v", result.DiscoveredBy)
	}
	if result.Score != 0 {
		t.Errorf("expected score 0 for non-matching file, got %f", result.Score)
	}
}
