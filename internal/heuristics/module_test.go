package heuristics

import (
	"sort"
	"testing"
)

func TestDetectModule(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple two-level path",
			path:     "browser/settings/settings_page.cc",
			expected: "browser/settings",
		},
		{
			name:     "three-level path",
			path:     "chrome/browser/ui/views/frame.cc",
			expected: "chrome/browser",
		},
		{
			name:     "two-level path exactly",
			path:     "src/components/Button.tsx",
			expected: "src/components",
		},
		{
			name:     "root level file",
			path:     "main.go",
			expected: "",
		},
		{
			name:     "single directory",
			path:     "cmd/main.go",
			expected: "cmd",
		},
		{
			name:     "deeply nested path",
			path:     "a/b/c/d/e/file.go",
			expected: "a/b",
		},
		{
			name:     "path with dots",
			path:     "github.com/user/repo/file.go",
			expected: "github.com/user",
		},
		{
			name:     "windows-style path",
			path:     "browser\\settings\\settings_page.cc",
			expected: "browser/settings",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "current directory",
			path:     "./file.go",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectModule(tt.path)
			if got != tt.expected {
				t.Errorf("DetectModule(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestDetectModuleWithConfig(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		config   *ModuleConfig
		expected string
	}{
		{
			name: "max depth 1",
			path: "a/b/c/d/file.go",
			config: &ModuleConfig{
				MaxDepth: 1,
			},
			expected: "a",
		},
		{
			name: "max depth 3",
			path: "a/b/c/d/file.go",
			config: &ModuleConfig{
				MaxDepth: 3,
			},
			expected: "a/b/c",
		},
		{
			name: "ignore single prefix",
			path: "src/components/Button.tsx",
			config: &ModuleConfig{
				MaxDepth:       2,
				IgnorePrefixes: []string{"src"},
			},
			expected: "components",
		},
		{
			name: "ignore nested prefix",
			path: "src/web/components/Button.tsx",
			config: &ModuleConfig{
				MaxDepth:       2,
				IgnorePrefixes: []string{"src/web"},
			},
			expected: "components",
		},
		{
			name: "ignore prefix exact match",
			path: "src/file.go",
			config: &ModuleConfig{
				MaxDepth:       2,
				IgnorePrefixes: []string{"src"},
			},
			expected: "",
		},
		{
			name: "multiple ignore prefixes",
			path: "lib/internal/util/helper.go",
			config: &ModuleConfig{
				MaxDepth:       2,
				IgnorePrefixes: []string{"lib", "pkg"},
			},
			expected: "internal/util",
		},
		{
			name:     "nil config uses default",
			path:     "a/b/c/file.go",
			config:   nil,
			expected: "a/b",
		},
		{
			name: "zero max depth falls back to default",
			path: "a/b/c/file.go",
			config: &ModuleConfig{
				MaxDepth: 0,
			},
			expected: "a/b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectModuleWithConfig(tt.path, tt.config)
			if got != tt.expected {
				t.Errorf("DetectModuleWithConfig(%q, %+v) = %q, want %q", tt.path, tt.config, got, tt.expected)
			}
		})
	}
}

func TestSameModule(t *testing.T) {
	tests := []struct {
		name     string
		path1    string
		path2    string
		expected bool
	}{
		{
			name:     "same module",
			path1:    "browser/settings/page1.cc",
			path2:    "browser/settings/page2.cc",
			expected: true,
		},
		{
			name:     "different modules",
			path1:    "browser/settings/page.cc",
			path2:    "browser/tabs/page.cc",
			expected: false,
		},
		{
			name:     "both root level",
			path1:    "main.go",
			path2:    "app.go",
			expected: true,
		},
		{
			name:     "root and non-root",
			path1:    "main.go",
			path2:    "cmd/main.go",
			expected: false,
		},
		{
			name:     "different depths same prefix",
			path1:    "a/b/file.go",
			path2:    "a/b/c/file.go",
			expected: true, // Both have module "a/b"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SameModule(tt.path1, tt.path2)
			if got != tt.expected {
				t.Errorf("SameModule(%q, %q) = %v, want %v", tt.path1, tt.path2, got, tt.expected)
			}
		})
	}
}

func TestParentModule(t *testing.T) {
	tests := []struct {
		module   string
		expected string
	}{
		{"browser/settings", "browser"},
		{"browser", ""},
		{"", ""},
		{"a/b/c", "a/b"},
		{"single", ""},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			got := ParentModule(tt.module)
			if got != tt.expected {
				t.Errorf("ParentModule(%q) = %q, want %q", tt.module, got, tt.expected)
			}
		})
	}
}

func TestIsSubModuleOf(t *testing.T) {
	tests := []struct {
		name     string
		child    string
		parent   string
		expected bool
	}{
		{
			name:     "direct submodule",
			child:    "browser/settings/ui",
			parent:   "browser/settings",
			expected: true,
		},
		{
			name:     "same module",
			child:    "browser/settings",
			parent:   "browser/settings",
			expected: true,
		},
		{
			name:     "not a submodule",
			child:    "browser/tabs",
			parent:   "browser/settings",
			expected: false,
		},
		{
			name:     "empty parent matches everything",
			child:    "any/module",
			parent:   "",
			expected: true,
		},
		{
			name:     "empty child with non-empty parent",
			child:    "",
			parent:   "browser",
			expected: false,
		},
		{
			name:     "nested submodule",
			child:    "a/b/c/d",
			parent:   "a",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSubModuleOf(tt.child, tt.parent)
			if got != tt.expected {
				t.Errorf("IsSubModuleOf(%q, %q) = %v, want %v", tt.child, tt.parent, got, tt.expected)
			}
		})
	}
}

func TestModuleDepth(t *testing.T) {
	tests := []struct {
		module   string
		expected int
	}{
		{"", 0},
		{"browser", 1},
		{"browser/settings", 2},
		{"a/b/c/d", 4},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			got := ModuleDepth(tt.module)
			if got != tt.expected {
				t.Errorf("ModuleDepth(%q) = %d, want %d", tt.module, got, tt.expected)
			}
		})
	}
}

func TestCommonModule(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		expected string
	}{
		{
			name:     "empty slice",
			paths:    []string{},
			expected: "",
		},
		{
			name:     "single path",
			paths:    []string{"browser/settings/page.cc"},
			expected: "browser/settings",
		},
		{
			name:     "same module",
			paths:    []string{"browser/settings/a.cc", "browser/settings/b.cc"},
			expected: "browser/settings",
		},
		{
			name:     "different modules same parent",
			paths:    []string{"browser/settings/a.cc", "browser/tabs/b.cc"},
			expected: "browser",
		},
		{
			name:     "no common module",
			paths:    []string{"a/b/c/file.go", "x/y/z/file.go"},
			expected: "",
		},
		{
			name:     "all root files",
			paths:    []string{"main.go", "app.go"},
			expected: "",
		},
		{
			name:     "mixed depths",
			paths:    []string{"a/b/c/d/file.go", "a/b/x/file.go", "a/y/z/file.go"},
			expected: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommonModule(tt.paths)
			if got != tt.expected {
				t.Errorf("CommonModule(%v) = %q, want %q", tt.paths, got, tt.expected)
			}
		})
	}
}

func TestGroupByModule(t *testing.T) {
	paths := []string{
		"browser/settings/page1.cc",
		"browser/settings/page2.cc",
		"browser/tabs/tab1.cc",
		"chrome/browser/ui/view.cc",
		"main.go",
		"cmd/main.go",
	}

	groups := GroupByModule(paths)

	// Verify grouping
	if len(groups["browser/settings"]) != 2 {
		t.Errorf("expected 2 files in browser/settings, got %d", len(groups["browser/settings"]))
	}
	if len(groups["browser/tabs"]) != 1 {
		t.Errorf("expected 1 file in browser/tabs, got %d", len(groups["browser/tabs"]))
	}
	if len(groups["chrome/browser"]) != 1 {
		t.Errorf("expected 1 file in chrome/browser, got %d", len(groups["chrome/browser"]))
	}
	if len(groups["cmd"]) != 1 {
		t.Errorf("expected 1 file in cmd, got %d", len(groups["cmd"]))
	}
	if len(groups[""]) != 1 {
		t.Errorf("expected 1 root file, got %d", len(groups[""]))
	}
}

func TestGroupByModuleWithConfig(t *testing.T) {
	paths := []string{
		"src/components/Button.tsx",
		"src/components/Input.tsx",
		"src/utils/helper.ts",
		"src/pages/index.tsx",
	}

	config := &ModuleConfig{
		MaxDepth:       2,
		IgnorePrefixes: []string{"src"},
	}

	groups := GroupByModuleWithConfig(paths, config)

	// After ignoring "src" prefix
	if len(groups["components"]) != 2 {
		t.Errorf("expected 2 files in components, got %d", len(groups["components"]))
	}
	if len(groups["utils"]) != 1 {
		t.Errorf("expected 1 file in utils, got %d", len(groups["utils"]))
	}
	if len(groups["pages"]) != 1 {
		t.Errorf("expected 1 file in pages, got %d", len(groups["pages"]))
	}
}

func TestModuleDetectorDetect(t *testing.T) {
	detector := NewModuleDetector(&ModuleConfig{
		MaxDepth:       3,
		IgnorePrefixes: []string{"internal"},
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"internal/heuristics/module.go", "heuristics"},
		{"internal/heuristics/sub/module.go", "heuristics/sub"},
		{"internal/config/config.go", "config"},
		{"a/b/c/d/e.go", "a/b/c"},
		{"single/file.go", "single"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detector.Detect(tt.path)
			if got != tt.expected {
				t.Errorf("Detect(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestDefaultModuleConfig(t *testing.T) {
	config := DefaultModuleConfig()
	if config.MaxDepth != 2 {
		t.Errorf("expected default MaxDepth 2, got %d", config.MaxDepth)
	}
	if len(config.IgnorePrefixes) != 0 {
		t.Errorf("expected empty IgnorePrefixes, got %v", config.IgnorePrefixes)
	}
}

func TestModuleStability(t *testing.T) {
	// Test that files in the same directory always get the same module
	paths := []string{
		"browser/settings/page1.cc",
		"browser/settings/page2.cc",
		"browser/settings/page3.h",
		"browser/settings/utils/helper.cc",
	}

	modules := make(map[string]string)
	for _, p := range paths {
		modules[p] = DetectModule(p)
	}

	// All files in browser/settings should have module "browser/settings"
	// except utils/helper.cc which is one level deeper
	for p, m := range modules {
		if p == "browser/settings/utils/helper.cc" {
			if m != "browser/settings" {
				t.Errorf("path %q got module %q, want browser/settings", p, m)
			}
		} else {
			if m != "browser/settings" {
				t.Errorf("path %q got module %q, want browser/settings", p, m)
			}
		}
	}
}

func TestSortedGroupKeys(t *testing.T) {
	// Verify that GroupByModule produces consistent results
	paths := []string{
		"c/file.go",
		"a/file.go",
		"b/file.go",
	}

	groups := GroupByModule(paths)

	// Get sorted keys
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	expected := []string{"a", "b", "c"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("sorted key %d = %q, want %q", i, k, expected[i])
		}
	}
}
