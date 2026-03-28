// Package heuristics provides heuristic rules for file analysis.
package heuristics

import (
	"path/filepath"
	"strings"
)

// ModuleConfig holds configuration for module detection.
type ModuleConfig struct {
	// MaxDepth limits how many directory levels to include in the module name.
	// Default is 2, meaning "a/b/c/file.go" -> "a/b".
	// Set to 1 for single-level modules: "a/b/c/file.go" -> "a".
	MaxDepth int

	// IgnorePrefixes are path prefixes to strip before computing module.
	// Useful for monorepos with common prefixes like "src/", "lib/", "pkg/".
	IgnorePrefixes []string
}

// DefaultModuleConfig returns the default configuration for module detection.
func DefaultModuleConfig() *ModuleConfig {
	return &ModuleConfig{
		MaxDepth:       2,
		IgnorePrefixes: []string{},
	}
}

// ModuleDetector extracts coarse-grained module names from file paths.
type ModuleDetector struct {
	config *ModuleConfig
}

// NewModuleDetector creates a new ModuleDetector with the given configuration.
func NewModuleDetector(config *ModuleConfig) *ModuleDetector {
	if config == nil {
		config = DefaultModuleConfig()
	}
	return &ModuleDetector{config: config}
}

// Detect extracts the module name from a file path.
// The module is derived from the directory structure, providing a stable
// grouping for files in the same directory subtree.
//
// Examples with default config (MaxDepth=2):
//   - "browser/settings/settings_page.cc" -> "browser/settings"
//   - "chrome/browser/ui/views/frame.cc" -> "chrome/browser"
//   - "src/components/Button.tsx" -> "src/components"
//   - "main.go" -> "" (root files have no module)
func (md *ModuleDetector) Detect(path string) string {
	// Normalize path separators first (handles Windows-style backslashes)
	// filepath.ToSlash only works on Windows, so we need to also manually replace
	path = filepath.ToSlash(path)
	path = strings.ReplaceAll(path, "\\", "/")

	// Get directory part (now path uses forward slashes)
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return ""
	}

	// Strip ignored prefixes
	dir = md.stripIgnorePrefixes(dir)

	// Split into parts
	parts := strings.Split(dir, "/")

	// Filter out empty parts
	var cleanParts []string
	for _, p := range parts {
		if p != "" {
			cleanParts = append(cleanParts, p)
		}
	}

	// Apply max depth limit
	maxDepth := md.config.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 2 // fallback to default
	}
	if len(cleanParts) > maxDepth {
		cleanParts = cleanParts[:maxDepth]
	}

	return strings.Join(cleanParts, "/")
}

// stripIgnorePrefixes removes configured prefix paths from the directory.
func (md *ModuleDetector) stripIgnorePrefixes(dir string) string {
	for _, prefix := range md.config.IgnorePrefixes {
		prefix = strings.TrimSuffix(prefix, "/")
		if strings.HasPrefix(dir, prefix+"/") {
			return strings.TrimPrefix(dir, prefix+"/")
		}
		if dir == prefix {
			return ""
		}
	}
	return dir
}

// DetectModule is a convenience function that uses default configuration.
// It extracts the module name from a file path.
func DetectModule(path string) string {
	return NewModuleDetector(nil).Detect(path)
}

// DetectModuleWithConfig extracts the module name using custom configuration.
func DetectModuleWithConfig(path string, config *ModuleConfig) string {
	return NewModuleDetector(config).Detect(path)
}

// ModuleDepth returns the depth of the module (number of path components).
func ModuleDepth(module string) int {
	if module == "" {
		return 0
	}
	return strings.Count(module, "/") + 1
}

// SameModule returns true if two paths belong to the same module.
func SameModule(path1, path2 string) bool {
	return DetectModule(path1) == DetectModule(path2)
}

// SameModuleWithConfig returns true if two paths belong to the same module
// using the provided configuration.
func SameModuleWithConfig(path1, path2 string, config *ModuleConfig) bool {
	return DetectModuleWithConfig(path1, config) == DetectModuleWithConfig(path2, config)
}

// ParentModule returns the parent module one level up.
// Example: "browser/settings" -> "browser"
// Returns empty string if the module has no parent.
func ParentModule(module string) string {
	if module == "" {
		return ""
	}
	idx := strings.LastIndex(module, "/")
	if idx < 0 {
		return ""
	}
	return module[:idx]
}

// IsSubModuleOf returns true if child is a sub-module of parent.
// Example: "browser/settings/ui" is a sub-module of "browser/settings"
func IsSubModuleOf(child, parent string) bool {
	if parent == "" {
		return true // Everything is a sub-module of root
	}
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+"/")
}

// CommonModule returns the deepest common module shared by multiple paths.
// Returns empty string if no common module exists.
func CommonModule(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	if len(paths) == 1 {
		return DetectModule(paths[0])
	}

	// Start with the first path's module
	common := DetectModule(paths[0])

	// Find intersection with all other paths
	for _, path := range paths[1:] {
		mod := DetectModule(path)
		for !IsSubModuleOf(mod, common) && common != "" {
			common = ParentModule(common)
		}
	}

	return common
}

// GroupByModule groups file paths by their module.
// Returns a map where keys are module names and values are lists of file paths.
func GroupByModule(paths []string) map[string][]string {
	return GroupByModuleWithConfig(paths, nil)
}

// GroupByModuleWithConfig groups file paths by their module using custom configuration.
func GroupByModuleWithConfig(paths []string, config *ModuleConfig) map[string][]string {
	result := make(map[string][]string)
	detector := NewModuleDetector(config)

	for _, path := range paths {
		mod := detector.Detect(path)
		result[mod] = append(result[mod], path)
	}

	return result
}
