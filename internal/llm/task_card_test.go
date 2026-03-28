package llm

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/no22/repo-scout/internal/schema"
)

func TestTaskTypeConstants(t *testing.T) {
	// Verify all required task types are defined
	requiredTypes := []TaskType{
		TaskClassifyFileRole,
		TaskJudgeRelevance,
		TaskShouldExpand,
		TaskIsImplicitDependency,
	}

	for _, tt := range requiredTypes {
		if tt == "" {
			t.Errorf("TaskType constant should not be empty")
		}
	}

	// Verify string values match spec
	if TaskClassifyFileRole != "classify_file_role" {
		t.Errorf("TaskClassifyFileRole = %q, want %q", TaskClassifyFileRole, "classify_file_role")
	}
	if TaskJudgeRelevance != "judge_relevance" {
		t.Errorf("TaskJudgeRelevance = %q, want %q", TaskJudgeRelevance, "judge_relevance")
	}
	if TaskShouldExpand != "should_expand" {
		t.Errorf("TaskShouldExpand = %q, want %q", TaskShouldExpand, "should_expand")
	}
	if TaskIsImplicitDependency != "is_implicit_dependency" {
		t.Errorf("TaskIsImplicitDependency = %q, want %q", TaskIsImplicitDependency, "is_implicit_dependency")
	}
}

func TestNewTaskCard(t *testing.T) {
	fc := schema.NewFileCard("internal/scanner/scanner.go")
	fc.Lang = "go"
	fc.Module = "scanner"
	fc.Symbols = []string{"ScanRepo", "ScanFile"}
	fc.Neighbors = []string{"internal/config/config.go"}
	fc.HeuristicTags = []string{"core"}

	tc := NewTaskCard(TaskClassifyFileRole, "Add error handling to scanner", fc)

	if tc.Type != TaskClassifyFileRole {
		t.Errorf("Type = %q, want %q", tc.Type, TaskClassifyFileRole)
	}
	if tc.Task != "Add error handling to scanner" {
		t.Errorf("Task = %q, want %q", tc.Task, "Add error handling to scanner")
	}
	if tc.FilePath != "internal/scanner/scanner.go" {
		t.Errorf("FilePath = %q, want %q", tc.FilePath, "internal/scanner/scanner.go")
	}
	if tc.FileLang != "go" {
		t.Errorf("FileLang = %q, want %q", tc.FileLang, "go")
	}
	if tc.FileModule != "scanner" {
		t.Errorf("FileModule = %q, want %q", tc.FileModule, "scanner")
	}
	if len(tc.FileSymbols) != 2 {
		t.Errorf("len(FileSymbols) = %d, want 2", len(tc.FileSymbols))
	}
	if len(tc.FileNeighbors) != 1 {
		t.Errorf("len(FileNeighbors) = %d, want 1", len(tc.FileNeighbors))
	}
	if len(tc.FileHeuristicTags) != 1 {
		t.Errorf("len(FileHeuristicTags) = %d, want 1", len(tc.FileHeuristicTags))
	}
	if tc.ID == "" {
		t.Error("ID should not be empty")
	}
	if tc.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestNewTaskCardFromRequest(t *testing.T) {
	req := &schema.ReconRequest{
		Task:         "Implement caching layer",
		RepoRoot:     "/home/user/project",
		SeedFiles:    []string{"cmd/main.go", "internal/service.go"},
		FocusSymbols: []string{"Cache", "Get", "Set"},
	}

	fc := schema.NewFileCard("internal/cache/redis.go")
	fc.Lang = "go"

	tc := NewTaskCardFromRequest(TaskJudgeRelevance, req, fc)

	if tc.Task != "Implement caching layer" {
		t.Errorf("Task = %q, want %q", tc.Task, "Implement caching layer")
	}
	if len(tc.SeedFiles) != 2 {
		t.Errorf("len(SeedFiles) = %d, want 2", len(tc.SeedFiles))
	}
	if len(tc.FocusSymbols) != 3 {
		t.Errorf("len(FocusSymbols) = %d, want 3", len(tc.FocusSymbols))
	}
}

func TestTaskCardSetContextSnippet(t *testing.T) {
	fc := schema.NewFileCard("test.go")
	tc := NewTaskCard(TaskClassifyFileRole, "test task", fc)

	tc.SetContextSnippet("This file implements the main handler")

	if tc.ContextSnippet != "This file implements the main handler" {
		t.Errorf("ContextSnippet = %q, want %q", tc.ContextSnippet, "This file implements the main handler")
	}
}

func TestTaskCardSetMetadata(t *testing.T) {
	fc := schema.NewFileCard("test.go")
	tc := NewTaskCard(TaskClassifyFileRole, "test task", fc)

	tc.SetMetadata("priority", "high")
	tc.SetMetadata("source", "user")

	if tc.Metadata["priority"] != "high" {
		t.Errorf("Metadata[priority] = %q, want %q", tc.Metadata["priority"], "high")
	}
	if tc.Metadata["source"] != "user" {
		t.Errorf("Metadata[source] = %q, want %q", tc.Metadata["source"], "user")
	}
}

func TestTaskCardToJSON(t *testing.T) {
	fc := schema.NewFileCard("internal/handler.go")
	fc.Lang = "go"
	fc.Module = "handler"

	tc := NewTaskCard(TaskClassifyFileRole, "Add authentication", fc)
	tc.SetContextSnippet("Handler for API endpoints")

	data, err := tc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Verify it's valid JSON
	var parsed TaskCard
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if parsed.Type != TaskClassifyFileRole {
		t.Errorf("Unmarshaled Type = %q, want %q", parsed.Type, TaskClassifyFileRole)
	}
	if parsed.FilePath != "internal/handler.go" {
		t.Errorf("Unmarshaled FilePath = %q, want %q", parsed.FilePath, "internal/handler.go")
	}
}

func TestTaskCardToJSONIndent(t *testing.T) {
	fc := schema.NewFileCard("test.go")
	tc := NewTaskCard(TaskClassifyFileRole, "test", fc)

	data, err := tc.ToJSONIndent()
	if err != nil {
		t.Fatalf("ToJSONIndent() error = %v", err)
	}

	// Should contain newlines for indentation
	if !strings.Contains(string(data), "\n") {
		t.Error("Indented JSON should contain newlines")
	}
}

func TestTaskCardToPrompt(t *testing.T) {
	fc := schema.NewFileCard("internal/service/auth.go")
	fc.Lang = "go"
	fc.Module = "service/auth"
	fc.Symbols = []string{"Login", "Logout", "ValidateToken"}
	fc.Neighbors = []string{"internal/config/config.go", "internal/db/user.go"}
	fc.HeuristicTags = []string{"core", "security"}

	tc := NewTaskCard(TaskClassifyFileRole, "Add OAuth2 support", fc)
	tc.SetContextSnippet("This service handles user authentication")
	tc.SeedFiles = []string{"cmd/server/main.go"}
	tc.FocusSymbols = []string{"OAuth", "Token"}

	prompt := tc.ToPrompt()

	// Verify essential parts are in the prompt
	tests := []struct {
		name     string
		contains string
	}{
		{"task", "Add OAuth2 support"},
		{"file path", "internal/service/auth.go"},
		{"language", "go"},
		{"module", "service/auth"},
		{"symbols", "Login"},
		{"neighbors", "internal/config/config.go"},
		{"tags", "core"},
		{"context snippet", "This service handles user authentication"},
		{"seed files", "cmd/server/main.go"},
		{"focus symbols", "OAuth"},
		{"task type question", "Classify this file"},
		{"options", "main_chain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(prompt, tt.contains) {
				t.Errorf("Prompt missing %q", tt.contains)
			}
		})
	}
}

func TestTaskCardToPromptAllTaskTypes(t *testing.T) {
	taskTypes := []struct {
		name     string
		taskType TaskType
		question string
	}{
		{
			name:     "classify_file_role",
			taskType: TaskClassifyFileRole,
			question: "Classify this file's role",
		},
		{
			name:     "judge_relevance",
			taskType: TaskJudgeRelevance,
			question: "Is this file relevant",
		},
		{
			name:     "should_expand",
			taskType: TaskShouldExpand,
			question: "Should we explore the neighbors",
		},
		{
			name:     "is_implicit_dependency",
			taskType: TaskIsImplicitDependency,
			question: "Is this file an implicit dependency",
		},
	}

	fc := schema.NewFileCard("test.go")

	for _, tt := range taskTypes {
		t.Run(tt.name, func(t *testing.T) {
			tc := NewTaskCard(tt.taskType, "test task", fc)
			prompt := tc.ToPrompt()

			if !strings.Contains(prompt, tt.question) {
				t.Errorf("Prompt for %s missing expected question: %q", tt.name, tt.question)
			}
		})
	}
}

func TestGenerateTaskID(t *testing.T) {
	id1 := generateTaskID(TaskClassifyFileRole, "internal/handler.go")
	id2 := generateTaskID(TaskClassifyFileRole, "internal/handler.go")

	// IDs should be different due to timestamp
	if id1 == id2 {
		t.Error("Task IDs should be unique")
	}

	// IDs should contain task type and sanitized path
	if !strings.Contains(id1, "classify_file_role") {
		t.Error("Task ID should contain task type")
	}
	if !strings.Contains(id1, "internal_handler.go") {
		t.Error("Task ID should contain sanitized path")
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"internal/handler.go", "internal_handler.go"},
		{"path\\to\\file.go", "path_to_file.go"},
		{"simple.go", "simple.go"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizePath(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pure JSON",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON in text",
			input:    `Here is the response: {"classification": "main_chain", "confidence": 0.9}`,
			expected: `{"classification": "main_chain", "confidence": 0.9}`,
		},
		{
			name:     "JSON with prefix and suffix",
			input:    `Some text before {"a": 1} some text after`,
			expected: `{"a": 1}`,
		},
		{
			name:     "no JSON",
			input:    "No JSON here",
			expected: "{}",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if result != tt.expected {
				t.Errorf("extractJSON(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseTaskResult(t *testing.T) {
	tests := []struct {
		name       string
		taskType   TaskType
		response   string
		wantLabel  string
		wantConf   float64
		wantReason bool
	}{
		{
			name:       "classify_file_role response",
			taskType:   TaskClassifyFileRole,
			response:   `{"classification": "main_chain", "confidence": 0.85, "reason": "Core handler for the task"}`,
			wantLabel:  "main_chain",
			wantConf:   0.85,
			wantReason: true,
		},
		{
			name:       "judge_relevance response",
			taskType:   TaskJudgeRelevance,
			response:   `{"relevance": "relevant", "confidence": 0.9, "reason": "Directly implements the feature"}`,
			wantLabel:  "relevant",
			wantConf:   0.9,
			wantReason: true,
		},
		{
			name:       "should_expand response",
			taskType:   TaskShouldExpand,
			response:   `{"decision": "expand", "confidence": 0.7, "reason": "Has important imports"}`,
			wantLabel:  "expand",
			wantConf:   0.7,
			wantReason: true,
		},
		{
			name:       "is_implicit_dependency response",
			taskType:   TaskIsImplicitDependency,
			response:   `{"is_implicit": "yes", "confidence": 0.6, "reason": "Configuration needed"}`,
			wantLabel:  "yes",
			wantConf:   0.6,
			wantReason: true,
		},
		{
			name:       "response with extra text",
			taskType:   TaskClassifyFileRole,
			response:   `Here's my analysis: {"classification": "companion", "confidence": 0.75, "reason": "Supporting file"} End of response.`,
			wantLabel:  "companion",
			wantConf:   0.75,
			wantReason: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTaskResult(tt.taskType, tt.response)
			if err != nil {
				t.Fatalf("ParseTaskResult() error = %v", err)
			}

			if result.Type != tt.taskType {
				t.Errorf("Type = %q, want %q", result.Type, tt.taskType)
			}
			if result.GetLabel() != tt.wantLabel {
				t.Errorf("GetLabel() = %q, want %q", result.GetLabel(), tt.wantLabel)
			}
			if result.Confidence != tt.wantConf {
				t.Errorf("Confidence = %f, want %f", result.Confidence, tt.wantConf)
			}
			if tt.wantReason && result.Reason == "" {
				t.Error("Reason should not be empty")
			}
		})
	}
}

func TestParseTaskResultEmpty(t *testing.T) {
	// When response has no JSON, extractJSON returns "{}" which is valid
	// ParseTaskResult should succeed but with empty fields
	result, err := ParseTaskResult(TaskClassifyFileRole, "not valid JSON")
	if err != nil {
		t.Errorf("ParseTaskResult should not error on empty JSON, got: %v", err)
	}
	if result.Classification != "" {
		t.Errorf("Classification should be empty for empty JSON, got: %q", result.Classification)
	}
}

func TestTaskResultToJSON(t *testing.T) {
	result := &TaskResult{
		Type:           TaskClassifyFileRole,
		Classification: "main_chain",
		Confidence:     0.9,
		Reason:         "Core file for the task",
	}

	data, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	var parsed TaskResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed.Classification != "main_chain" {
		t.Errorf("Classification = %q, want %q", parsed.Classification, "main_chain")
	}
}

func TestTaskResultIsPositive(t *testing.T) {
	tests := []struct {
		name     string
		result   *TaskResult
		expected bool
	}{
		{
			name:     "classify main_chain",
			result:   &TaskResult{Type: TaskClassifyFileRole, Classification: "main_chain"},
			expected: true,
		},
		{
			name:     "classify companion",
			result:   &TaskResult{Type: TaskClassifyFileRole, Classification: "companion"},
			expected: true,
		},
		{
			name:     "classify uncertain",
			result:   &TaskResult{Type: TaskClassifyFileRole, Classification: "uncertain"},
			expected: false,
		},
		{
			name:     "classify irrelevant",
			result:   &TaskResult{Type: TaskClassifyFileRole, Classification: "irrelevant"},
			expected: false,
		},
		{
			name:     "relevance relevant",
			result:   &TaskResult{Type: TaskJudgeRelevance, Relevance: "relevant"},
			expected: true,
		},
		{
			name:     "relevance possibly_relevant",
			result:   &TaskResult{Type: TaskJudgeRelevance, Relevance: "possibly_relevant"},
			expected: true,
		},
		{
			name:     "relevance irrelevant",
			result:   &TaskResult{Type: TaskJudgeRelevance, Relevance: "irrelevant"},
			expected: false,
		},
		{
			name:     "expand yes",
			result:   &TaskResult{Type: TaskShouldExpand, Decision: "expand"},
			expected: true,
		},
		{
			name:     "expand skip",
			result:   &TaskResult{Type: TaskShouldExpand, Decision: "skip"},
			expected: false,
		},
		{
			name:     "implicit yes",
			result:   &TaskResult{Type: TaskIsImplicitDependency, IsImplicit: "yes"},
			expected: true,
		},
		{
			name:     "implicit no",
			result:   &TaskResult{Type: TaskIsImplicitDependency, IsImplicit: "no"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsPositive(); got != tt.expected {
				t.Errorf("IsPositive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTaskCardSerializationRoundTrip(t *testing.T) {
	fc := schema.NewFileCard("internal/service/cache.go")
	fc.Lang = "go"
	fc.Module = "service/cache"
	fc.Symbols = []string{"Get", "Set", "Delete"}
	fc.Neighbors = []string{"internal/config/config.go"}
	fc.HeuristicTags = []string{"core"}

	tc := NewTaskCard(TaskClassifyFileRole, "Implement distributed caching", fc)
	tc.SetContextSnippet("Redis-based cache implementation")
	tc.SetMetadata("priority", "high")
	tc.SeedFiles = []string{"cmd/main.go"}
	tc.FocusSymbols = []string{"Cache", "Redis"}

	// Serialize
	data, err := tc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Deserialize
	var parsed TaskCard
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// Verify key fields
	if parsed.Type != tc.Type {
		t.Errorf("Type mismatch: got %q, want %q", parsed.Type, tc.Type)
	}
	if parsed.Task != tc.Task {
		t.Errorf("Task mismatch: got %q, want %q", parsed.Task, tc.Task)
	}
	if parsed.FilePath != tc.FilePath {
		t.Errorf("FilePath mismatch: got %q, want %q", parsed.FilePath, tc.FilePath)
	}
	if len(parsed.FileSymbols) != len(tc.FileSymbols) {
		t.Errorf("FileSymbols length mismatch: got %d, want %d", len(parsed.FileSymbols), len(tc.FileSymbols))
	}
	if parsed.ContextSnippet != tc.ContextSnippet {
		t.Errorf("ContextSnippet mismatch: got %q, want %q", parsed.ContextSnippet, tc.ContextSnippet)
	}
}

func TestTaskCardCreatedAtPreservation(t *testing.T) {
	fc := schema.NewFileCard("test.go")
	tc := NewTaskCard(TaskClassifyFileRole, "test", fc)
	originalTime := tc.CreatedAt

	// Serialize and deserialize
	data, _ := json.Marshal(tc)
	var parsed TaskCard
	json.Unmarshal(data, &parsed)

	// Time should be preserved (within reasonable tolerance)
	diff := parsed.CreatedAt.Sub(originalTime)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("CreatedAt not preserved: original=%v, parsed=%v", originalTime, parsed.CreatedAt)
	}
}
