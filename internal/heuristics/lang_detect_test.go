package heuristics

import (
	"testing"
)

func TestLangDetect(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		// Go
		{"main.go", "go"},
		{"internal/scanner/scanner.go", "go"},
		{"cmd/app/main.go", "go"},

		// JavaScript/TypeScript
		{"index.js", "js"},
		{"app.jsx", "jsx"},
		{"main.ts", "ts"},
		{"component.tsx", "tsx"},
		{"module.mjs", "js"},
		{"common.cjs", "js"},

		// C/C++
		{"main.c", "c"},
		{"header.h", "c"},
		{"impl.cpp", "cpp"},
		{"impl.cc", "cpp"},
		{"impl.cxx", "cpp"},
		{"header.hpp", "cpp"},
		{"header.hxx", "cpp"},

		// Python
		{"app.py", "py"},
		{"types.pyi", "py"},
		{"script.pyw", "py"},

		// Java/Kotlin
		{"Main.java", "java"},
		{"App.kt", "kotlin"},
		{"build.gradle.kts", "kotlin"},
		{"Main.scala", "scala"},

		// Rust
		{"main.rs", "rust"},
		{"lib.rs", "rust"},

		// Ruby
		{"app.rb", "ruby"},
		{"gem.gemspec", "ruby"},

		// C#
		{"Program.cs", "csharp"},

		// Swift/Objective-C
		{"App.swift", "swift"},
		{"delegate.m", "objc"},
		{"impl.mm", "objcpp"},

		// PHP
		{"index.php", "php"},
		{"view.phtml", "php"},

		// Shell
		{"script.sh", "shell"},
		{"script.bash", "shell"},
		{"config.zsh", "shell"},
		{"config.fish", "shell"},

		// Config/Data
		{"config.json", "json"},
		{"data.yaml", "yaml"},
		{"data.yml", "yaml"},
		{"config.toml", "toml"},
		{"settings.ini", "ini"},
		{"app.cfg", "ini"},
		{".env", "dotenv"},
		{"app.properties", "properties"},

		// Build systems
		{"BUILD.gn", "gn"},
		{"vars.gni", "gn"},
		{"build.gradle", "gradle"},
		{"config.xml", "xml"},

		// Markup
		{"index.html", "html"},
		{"README.md", "markdown"},
		{"docs.rst", "rst"},

		// Styles
		{"style.css", "css"},
		{"style.scss", "scss"},
		{"style.sass", "sass"},
		{"style.less", "less"},

		// SQL
		{"schema.sql", "sql"},

		// Protocol definitions
		{"api.proto", "proto"},
		{"service.thrift", "thrift"},

		// Special files
		{"Dockerfile", "dockerfile"},
		{"Dockerfile.prod", "dockerfile"},
		{"dockerfile.dev", "dockerfile"},
		{"Makefile", "makefile"},

		// Unknown/conservative fallback
		{"unknown.xyz", "text"},
		{"README", "text"},
		{"LICENSE", "text"},
		{"randomfile", "text"},

		// Case insensitivity
		{"MAIN.GO", "go"},
		{"DOCKERFILE", "dockerfile"},
		{"MAKEFILE", "makefile"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := LangDetect(tt.path)
			if result != tt.expected {
				t.Errorf("LangDetect(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestDetectLangFromPath(t *testing.T) {
	// Test that it's an alias for LangDetect
	tests := []string{
		"main.go",
		"app.js",
		"Dockerfile",
		"unknown.xyz",
	}

	for _, path := range tests {
		expected := LangDetect(path)
		result := DetectLangFromPath(path)
		if result != expected {
			t.Errorf("DetectLangFromPath(%q) = %q, want %q", path, result, expected)
		}
	}
}

func TestIsSourceFile(t *testing.T) {
	sourceFiles := []string{
		"main.go",
		"app.js",
		"component.tsx",
		"main.py",
		"Main.java",
		"lib.rs",
		"app.rb",
		"main.cpp",
		"App.swift",
		"index.php",
		"script.sh",
	}

	for _, path := range sourceFiles {
		if !IsSourceFile(path) {
			t.Errorf("IsSourceFile(%q) = false, want true", path)
		}
	}

	nonSourceFiles := []string{
		"README.md",
		"config.json",
		"Dockerfile",
		"style.css",
		"image.png",
		"data.yaml",
		"Makefile",
	}

	for _, path := range nonSourceFiles {
		if IsSourceFile(path) {
			t.Errorf("IsSourceFile(%q) = true, want false", path)
		}
	}
}

func TestIsConfigFile(t *testing.T) {
	configFiles := []string{
		"config.json",
		"settings.yaml",
		"app.toml",
		".env",
		"config.ini",
		"build.xml",
		"Makefile",
		"Dockerfile",
		"settings.gradle",
	}

	for _, path := range configFiles {
		if !IsConfigFile(path) {
			t.Errorf("IsConfigFile(%q) = false, want true", path)
		}
	}

	nonConfigFiles := []string{
		"main.go",
		"app.js",
		"README.md",
		"style.css",
		"image.png",
	}

	for _, path := range nonConfigFiles {
		if IsConfigFile(path) {
			t.Errorf("IsConfigFile(%q) = true, want false", path)
		}
	}
}

func TestIsGeneratedFile(t *testing.T) {
	generatedFiles := []string{
		"generated.go",
		"gen_types.go",
		"types_gen.go",
		"api.pb.go",
		"service.pb.rs",
		"types_pb2.py",
		"bundle.min.js",
		"style.min.css",
		"third_party/lib.go",
		"vendor/pkg/main.go",
	}

	for _, path := range generatedFiles {
		if !IsGeneratedFile(path) {
			t.Errorf("IsGeneratedFile(%q) = false, want true", path)
		}
	}

	nonGeneratedFiles := []string{
		"main.go",
		"app.js",
		"types.go",
		"mock_user.go",
	}

	for _, path := range nonGeneratedFiles {
		if IsGeneratedFile(path) {
			t.Errorf("IsGeneratedFile(%q) = true, want false", path)
		}
	}
}

func TestLangDetectConservative(t *testing.T) {
	// Test that unknown files return "text" instead of error
	unknownFiles := []string{
		"file.xyz",
		"file.unknown",
		"file.123",
		"file.",
		"file",
		"",
	}

	for _, path := range unknownFiles {
		result := LangDetect(path)
		if result == "" {
			t.Errorf("LangDetect(%q) returned empty string, should return conservative value", path)
		}
		if result == "error" {
			t.Errorf("LangDetect(%q) returned 'error', should never return error", path)
		}
	}
}

func TestCaseInsensitiveDetection(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		// Mixed case extensions
		{"MAIN.Go", "go"},
		{"App.JS", "js"},
		{"Component.TSX", "tsx"},
		{"DOCKERFILE", "dockerfile"},
		{"MAKEFILE", "makefile"},
		{"Package.JSON", "json"},
		{"Config.YAML", "yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := LangDetect(tt.path)
			if result != tt.expected {
				t.Errorf("LangDetect(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestPathWithDirectories(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"src/main.go", "go"},
		{"internal/scanner/scanner.go", "go"},
		{"cmd/reposcout/main.go", "go"},
		{"src/components/Button.tsx", "tsx"},
		{"build/Dockerfile", "dockerfile"},
		{"scripts/build.sh", "shell"},
		{"configs/app.yaml", "yaml"},
		{"third_party/lib/BUILD.gn", "gn"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := LangDetect(tt.path)
			if result != tt.expected {
				t.Errorf("LangDetect(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}
