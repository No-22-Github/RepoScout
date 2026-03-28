// Package heuristics provides heuristic rules for file analysis.
package heuristics

import (
	"path/filepath"
	"strings"
)

// LangDetect identifies the programming language or file type based on file path.
// It returns a conservative language tag, never an error.
// Unknown files return "text" as a safe default.
func LangDetect(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	name := strings.ToLower(filepath.Base(path))

	// Check by extension first
	if lang, ok := extToLang[ext]; ok {
		return lang
	}

	// Check by exact filename
	if lang, ok := nameToLang[name]; ok {
		return lang
	}

	// Check by filename prefix patterns
	for prefix, lang := range prefixToLang {
		if strings.HasPrefix(name, prefix) {
			return lang
		}
	}

	// Check for files without extension but with known patterns
	// Example: Makefile, Dockerfile, Vagrantfile
	if lang, ok := exactNameToLang[name]; ok {
		return lang
	}

	// Default to text for unknown types
	return "text"
}

// extToLang maps file extensions to language identifiers.
var extToLang = map[string]string{
	// Go
	".go": "go",

	// JavaScript/TypeScript
	".js":  "js",
	".jsx": "jsx",
	".ts":  "ts",
	".tsx": "tsx",
	".mjs": "js",
	".cjs": "js",
	".mts": "ts",
	".cts": "ts",

	// C/C++
	".c":   "c",
	".h":   "c",
	".cpp": "cpp",
	".cc":  "cpp",
	".cxx": "cpp",
	".hpp": "cpp",
	".hxx": "cpp",
	".inc": "cpp",

	// Python
	".py":  "py",
	".pyi": "py",
	".pyw": "py",

	// Java/Kotlin
	".java":  "java",
	".kt":    "kotlin",
	".kts":   "kotlin",
	".scala": "scala",

	// Rust
	".rs": "rust",

	// Ruby
	".rb":      "ruby",
	".rake":    "ruby",
	".gemspec": "ruby",

	// C#
	".cs": "csharp",

	// Swift/Objective-C
	".swift": "swift",
	".m":     "objc",
	".mm":    "objcpp",

	// PHP
	".php":   "php",
	".phtml": "php",
	".php3":  "php",
	".php4":  "php",
	".php5":  "php",

	// Shell
	".sh":   "shell",
	".bash": "shell",
	".zsh":  "shell",
	".fish": "shell",

	// Config/Data
	".json":       "json",
	".yaml":       "yaml",
	".yml":        "yaml",
	".toml":       "toml",
	".ini":        "ini",
	".cfg":        "ini",
	".conf":       "ini",
	".env":        "dotenv",
	".properties": "properties",

	// Build systems
	".gn":     "gn",
	".gni":    "gn",
	".gradle": "gradle",
	".xml":    "xml",

	// Markup
	".html":     "html",
	".htm":      "html",
	".xhtml":    "html",
	".md":       "markdown",
	".markdown": "markdown",
	".rst":      "rst",
	".svg":      "svg",

	// Styles
	".css":  "css",
	".scss": "scss",
	".sass": "sass",
	".less": "less",

	// SQL
	".sql": "sql",

	// Protocol definitions
	".proto":  "proto",
	".thrift": "thrift",

	// WebAssembly
	".wat":  "wasm",
	".wast": "wasm",

	// Documentation
	".txt": "text",

	// Binary/compiled (still useful to tag)
	".so":    "binary",
	".dll":   "binary",
	".dylib": "binary",
	".a":     "binary",
	".lib":   "binary",
	".o":     "binary",
	".obj":   "binary",

	// Images
	".png":  "image",
	".jpg":  "image",
	".jpeg": "image",
	".gif":  "image",
	".ico":  "image",
	".webp": "image",

	// Lock files
	".lock": "lockfile",

	// Patch/diff
	".patch": "patch",
	".diff":  "patch",
}

// nameToLang maps exact filenames to language identifiers.
var nameToLang = map[string]string{
	// Build files without extensions
	"makefile":          "makefile",
	"gomod":             "go_mod",
	"go.sum":            "go_sum",
	"go.work":           "go_work",
	"cargo.toml":        "cargo",
	"cargo.lock":        "cargo_lock",
	"package.json":      "npm",
	"package-lock.json": "npm_lock",
	"yarn.lock":         "yarn_lock",
	"pnpm-lock.yaml":    "pnpm_lock",
	"composer.json":     "composer",
	"composer.lock":     "composer_lock",
	"pipfile":           "pipfile",
	"pipfile.lock":      "pipfile_lock",
	"poetry.lock":       "poetry_lock",
	"gemfile":           "gemfile",
	"gemfile.lock":      "gemfile_lock",

	// CI/CD
	".travis.yml":         "travis",
	".gitlab-ci.yml":      "gitlab_ci",
	"azure-pipelines.yml": "azure_pipelines",

	// Kubernetes
	"kustomization.yaml": "kustomize",
	"kustomization.yml":  "kustomize",
	"chart.yaml":         "helm",
}

// prefixToLang maps filename prefixes to language identifiers.
var prefixToLang = map[string]string{
	"dockerfile":       "dockerfile",
	"dockerfile.":      "dockerfile",
	"dockerfile_debug": "dockerfile",
	"dockerfile.dev":   "dockerfile",
	"dockerfile.prod":  "dockerfile",
	"dockerfile.test":  "dockerfile",
	"vagrantfile":      "ruby",
	"jenkinsfile":      "groovy",
	"makefile":         "makefile",
	"cmakelists":       "cmake",
	"cmakecache":       "cmake",
	"brewfile":         "ruby",
	"podfile":          "ruby",
	"fastfile":         "ruby",
	"apkbuild":         "shell",
	"pbxproj":          "pbxproj",
}

// exactNameToLang maps exact lowercase filenames to languages.
var exactNameToLang = map[string]string{
	"dockerfile":          "dockerfile",
	"vagrantfile":         "ruby",
	"jenkinsfile":         "groovy",
	"makefile":            "makefile",
	"brewfile":            "ruby",
	"podfile":             "ruby",
	"fastfile":            "ruby",
	"license":             "text",
	"license.md":          "markdown",
	"license.txt":         "text",
	"copying":             "text",
	"copying.md":          "markdown",
	"copying.txt":         "text",
	"readme":              "text",
	"readme.md":           "markdown",
	"readme.txt":          "text",
	"changelog":           "text",
	"changelog.md":        "markdown",
	"changelog.txt":       "text",
	"authors":             "text",
	"authors.md":          "markdown",
	"authors.txt":         "text",
	"contributors":        "text",
	"contributors.md":     "markdown",
	"contributors.txt":    "text",
	"news":                "text",
	"news.md":             "markdown",
	"news.txt":            "text",
	"todo":                "text",
	"todo.md":             "markdown",
	"todo.txt":            "text",
	"cmakelists.txt":      "cmake",
	"serviceaccountkey":   "pem",
	".gitignore":          "gitignore",
	".gitattributes":      "gitattributes",
	".dockerignore":       "dockerignore",
	".editorconfig":       "editorconfig",
	".mailmap":            "mailmap",
	".npmignore":          "npmignore",
	".nvmrc":              "nvmrc",
	".python-version":     "pyenv",
	".ruby-version":       "rbenv",
	".golangci.yml":       "golangci",
	".golangci.yaml":      "golangci",
	".golangci.toml":      "golangci",
	".golangci.json":      "golangci",
	".prettierrc":         "json",
	".prettierrc.json":    "json",
	".prettierrc.yaml":    "yaml",
	".prettierrc.yml":     "yaml",
	".eslintrc":           "json",
	".eslintrc.json":      "json",
	".eslintrc.yaml":      "yaml",
	".eslintrc.yml":       "yaml",
	".eslintrc.js":        "js",
	".eslintrc.cjs":       "js",
	".stylelintrc":        "json",
	".stylelintrc.json":   "json",
	".babelrc":            "json",
	".babelrc.json":       "json",
	".babelrc.js":         "js",
	".babelrc.cjs":        "js",
	"tsconfig.json":       "json",
	"jsconfig.json":       "json",
	".nycrc":              "json",
	".nycrc.json":         "json",
	".mocharc.json":       "json",
	".mocharc.js":         "js",
	".mocharc.yaml":       "yaml",
	".mocharc.yml":        "yaml",
	"jest.config.js":      "js",
	"jest.config.ts":      "ts",
	"jest.config.json":    "json",
	"vite.config.js":      "js",
	"vite.config.ts":      "ts",
	"vite.config.mjs":     "js",
	"webpack.config.js":   "js",
	"webpack.config.ts":   "ts",
	"webpack.config.mjs":  "js",
	"rollup.config.js":    "js",
	"rollup.config.ts":    "ts",
	"rollup.config.mjs":   "js",
	"esbuild.config.js":   "js",
	"esbuild.config.ts":   "ts",
	"esbuild.config.mjs":  "js",
	"tailwind.config.js":  "js",
	"tailwind.config.ts":  "ts",
	"tailwind.config.mjs": "js",
	"postcss.config.js":   "js",
	"postcss.config.ts":   "ts",
	"postcss.config.mjs":  "js",
	"postcss.config.json": "json",
	"next.config.js":      "js",
	"next.config.ts":      "ts",
	"next.config.mjs":     "js",
	"nuxt.config.js":      "js",
	"nuxt.config.ts":      "ts",
	"svelte.config.js":    "js",
	"svelte.config.ts":    "ts",
	"vite-env.d.ts":       "ts",
	"env.d.ts":            "ts",
}

// DetectLangFromPath detects language from a file path.
// This is an alias for LangDetect for API consistency.
func DetectLangFromPath(path string) string {
	return LangDetect(path)
}

// IsSourceFile returns true if the file is a recognized source code file.
func IsSourceFile(path string) bool {
	lang := LangDetect(path)
	sourceLangs := map[string]bool{
		"go":     true,
		"js":     true,
		"jsx":    true,
		"ts":     true,
		"tsx":    true,
		"c":      true,
		"cpp":    true,
		"py":     true,
		"java":   true,
		"kotlin": true,
		"scala":  true,
		"rust":   true,
		"ruby":   true,
		"csharp": true,
		"swift":  true,
		"objc":   true,
		"objcpp": true,
		"php":    true,
		"shell":  true,
		"sql":    true,
		"proto":  true,
		"thrift": true,
		"gn":     true,
	}
	return sourceLangs[lang]
}

// IsConfigFile returns true if the file is a configuration file.
func IsConfigFile(path string) bool {
	lang := LangDetect(path)
	configLangs := map[string]bool{
		"json":       true,
		"yaml":       true,
		"toml":       true,
		"ini":        true,
		"dotenv":     true,
		"properties": true,
		"xml":        true,
		"makefile":   true,
		"cmake":      true,
		"gradle":     true,
		"npm":        true,
		"cargo":      true,
		"go_mod":     true,
		"dockerfile": true,
	}
	return configLangs[lang]
}

// IsGeneratedFile returns true if the file is likely auto-generated.
func IsGeneratedFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	dir := strings.ToLower(filepath.Dir(path))

	generatedPatterns := []string{
		"generated",
		"gen_",
		"_gen.",
		".gen.",
		".pb.go",
		".pb.rs",
		"_pb2.",
		"_pb.",
		".mock.",
		".min.js",
		".min.css",
	}

	for _, pattern := range generatedPatterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}

	generatedDirs := []string{
		"generated",
		"gen",
		"third_party",
		"thirdparty",
		"vendor",
	}

	for _, d := range generatedDirs {
		if strings.Contains(dir, d) {
			return true
		}
	}

	return false
}
