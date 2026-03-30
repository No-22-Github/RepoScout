// Package llm provides LLM integration for RepoScout.
package llm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/no22/repo-scout/internal/schema"
)

// TaskType defines the type of LLM task.
type TaskType string

const (
	// TaskClassifyFileRole classifies a file's role in the codebase.
	// Returns: "main_chain", "companion", "uncertain", "irrelevant"
	TaskClassifyFileRole TaskType = "classify_file_role"

	// TaskJudgeRelevance judges if a file is relevant to the user's task.
	// Returns: "relevant", "possibly_relevant", "irrelevant"
	TaskJudgeRelevance TaskType = "judge_relevance"

	// TaskShouldExpand decides if a file's neighbors should be explored.
	// Returns: "expand", "skip", "uncertain"
	TaskShouldExpand TaskType = "should_expand"

	// TaskIsImplicitDependency checks if a file is an implicit dependency.
	// Returns: "yes", "no", "uncertain"
	TaskIsImplicitDependency TaskType = "is_implicit_dependency"
)

// TaskCard represents a short task card sent to the LLM backend.
// It is designed to be lightweight and serializable as a prompt.
type TaskCard struct {
	// ID is a unique identifier for tracking purposes.
	ID string `json:"id"`

	// Type specifies the task type.
	Type TaskType `json:"type"`

	// Task is the user's original task description.
	Task string `json:"task"`

	// FilePath is the path to the file being analyzed.
	FilePath string `json:"file_path"`

	// FileLang is the programming language of the file.
	FileLang string `json:"file_lang,omitempty"`

	// FileModule is the module the file belongs to.
	FileModule string `json:"file_module,omitempty"`

	// FileSymbols are extracted symbols from the file.
	FileSymbols []string `json:"file_symbols,omitempty"`

	// FileNeighbors are neighbor files discovered through static analysis.
	FileNeighbors []string `json:"file_neighbors,omitempty"`

	// FileHeuristicTags are tags applied by heuristic rules.
	FileHeuristicTags []string `json:"file_heuristic_tags,omitempty"`

	// ContextSnippet is compressed context information.
	// This could be a summary of related files or key code snippets.
	ContextSnippet string `json:"context_snippet,omitempty"`

	// SeedFiles are the original seed files for context.
	SeedFiles []string `json:"seed_files,omitempty"`

	// FocusSymbols are symbols the user wants to focus on.
	FocusSymbols []string `json:"focus_symbols,omitempty"`

	// CreatedAt is the timestamp when the task was created.
	CreatedAt time.Time `json:"created_at"`

	// Metadata contains additional optional metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewTaskCard creates a new TaskCard with the given parameters.
func NewTaskCard(taskType TaskType, task string, fileCard *schema.FileCard) *TaskCard {
	return &TaskCard{
		ID:                generateTaskID(taskType, fileCard.Path),
		Type:              taskType,
		Task:              task,
		FilePath:          fileCard.Path,
		FileLang:          fileCard.Lang,
		FileModule:        fileCard.Module,
		FileSymbols:       fileCard.Symbols,
		FileNeighbors:     fileCard.Neighbors,
		FileHeuristicTags: fileCard.HeuristicTags,
		CreatedAt:         time.Now(),
		Metadata:          make(map[string]string),
	}
}

// NewTaskCardFromRequest creates a TaskCard with additional context from a ReconRequest.
func NewTaskCardFromRequest(taskType TaskType, req *schema.ReconRequest, fileCard *schema.FileCard) *TaskCard {
	tc := NewTaskCard(taskType, req.Task, fileCard)
	tc.SeedFiles = req.SeedFiles
	tc.FocusSymbols = req.FocusSymbols
	return tc
}

// generateTaskID generates a unique ID for a task.
func generateTaskID(taskType TaskType, filePath string) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s-%s-%d", taskType, sanitizePath(filePath), timestamp)
}

// sanitizePath replaces path separators for use in IDs.
func sanitizePath(path string) string {
	return strings.ReplaceAll(strings.ReplaceAll(path, "/", "_"), "\\", "_")
}

// SetContextSnippet sets the context snippet for the task.
func (tc *TaskCard) SetContextSnippet(snippet string) {
	tc.ContextSnippet = snippet
}

// SetMetadata sets a metadata key-value pair.
func (tc *TaskCard) SetMetadata(key, value string) {
	if tc.Metadata == nil {
		tc.Metadata = make(map[string]string)
	}
	tc.Metadata[key] = value
}

// ToJSON returns the TaskCard as JSON bytes.
func (tc *TaskCard) ToJSON() ([]byte, error) {
	return json.Marshal(tc)
}

// ToJSONIndent returns the TaskCard as indented JSON bytes.
func (tc *TaskCard) ToJSONIndent() ([]byte, error) {
	return json.MarshalIndent(tc, "", "  ")
}

// ToPrompt generates a prompt string for the LLM based on the task type.
// This is the primary method for converting a TaskCard to LLM input.
func (tc *TaskCard) ToPrompt() string {
	var sb strings.Builder

	// Write task context
	sb.WriteString("## Task\n")
	sb.WriteString(tc.Task)
	sb.WriteString("\n\n")

	// Write file information
	sb.WriteString("## File\n")
	sb.WriteString(fmt.Sprintf("Path: %s\n", tc.FilePath))
	if tc.FileLang != "" {
		sb.WriteString(fmt.Sprintf("Language: %s\n", tc.FileLang))
	}
	if tc.FileModule != "" {
		sb.WriteString(fmt.Sprintf("Module: %s\n", tc.FileModule))
	}
	if len(tc.FileSymbols) > 0 {
		sb.WriteString(fmt.Sprintf("Symbols: %s\n", strings.Join(tc.FileSymbols, ", ")))
	}
	if len(tc.FileNeighbors) > 0 {
		sb.WriteString(fmt.Sprintf("Neighbors: %s\n", strings.Join(tc.FileNeighbors, ", ")))
	}
	if len(tc.FileHeuristicTags) > 0 {
		sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(tc.FileHeuristicTags, ", ")))
	}
	sb.WriteString("\n")

	// Write additional context if available
	if tc.ContextSnippet != "" {
		sb.WriteString("## Context\n")
		sb.WriteString(tc.ContextSnippet)
		sb.WriteString("\n\n")
	}

	// Write seed files if available
	if len(tc.SeedFiles) > 0 {
		sb.WriteString("## Seed Files\n")
		for _, sf := range tc.SeedFiles {
			sb.WriteString(fmt.Sprintf("- %s\n", sf))
		}
		sb.WriteString("\n")
	}

	// Write focus symbols if available
	if len(tc.FocusSymbols) > 0 {
		sb.WriteString("## Focus Symbols\n")
		sb.WriteString(strings.Join(tc.FocusSymbols, ", "))
		sb.WriteString("\n\n")
	}

	// Write the question based on task type
	sb.WriteString("## Question\n")
	sb.WriteString(tc.getQuestion())

	return sb.String()
}

// getQuestion returns the question to ask based on task type.
func (tc *TaskCard) getQuestion() string {
	switch tc.Type {
	case TaskClassifyFileRole:
		return `Classify this file's role in relation to the task.
Options: "main_chain", "companion", "uncertain", "irrelevant"

Respond in JSON format:
{
  "classification": "<one of the options>",
  "confidence": <0.0 to 1.0>,
  "reason": "<optional, brief>"
}`

	case TaskJudgeRelevance:
		return `Is this file relevant to the task?
Options: "relevant", "possibly_relevant", "irrelevant"

Respond in JSON format:
{
  "relevance": "<one of the options>",
  "confidence": <0.0 to 1.0>,
  "reason": "<optional, brief>"
}`

	case TaskShouldExpand:
		return `Should we explore the neighbors of this file to find more relevant files?
Options: "expand", "skip", "uncertain"

Consider:
- Does this file import or use other modules that might be relevant?
- Are there related files that could provide important context?

Respond in JSON format:
{
  "decision": "<one of the options>",
  "confidence": <0.0 to 1.0>,
  "reason": "<optional, brief>"
}`

	case TaskIsImplicitDependency:
		return `Is this file an implicit dependency for the task?
An implicit dependency is a file that the task implicitly requires but is not directly mentioned.
Options: "yes", "no", "uncertain"

Respond in JSON format:
{
  "is_implicit": "<one of the options>",
  "confidence": <0.0 to 1.0>,
  "reason": "<optional, brief>"
}`

	default:
		return "Please analyze this file and provide your assessment."
	}
}

// ParseTaskResult parses the LLM response into a TaskResult.
func ParseTaskResult(taskType TaskType, response string) (*TaskResult, error) {
	// Try to extract JSON from the response
	jsonStr := extractJSON(response)

	var result TaskResult
	if err := json.Unmarshal([]byte(jsonStr), &result.RawResponse); err != nil {
		return nil, fmt.Errorf("failed to parse task result: %w", err)
	}

	// Parse type-specific fields
	result.Type = taskType
	result.RawText = response

	switch taskType {
	case TaskClassifyFileRole:
		if classification, ok := result.RawResponse["classification"].(string); ok {
			result.Classification = classification
		}
	case TaskJudgeRelevance:
		if relevance, ok := result.RawResponse["relevance"].(string); ok {
			result.Relevance = relevance
		}
	case TaskShouldExpand:
		if decision, ok := result.RawResponse["decision"].(string); ok {
			result.Decision = decision
		}
	case TaskIsImplicitDependency:
		if isImplicit, ok := result.RawResponse["is_implicit"].(string); ok {
			result.IsImplicit = isImplicit
		}
	}

	if confidence, ok := result.RawResponse["confidence"].(float64); ok {
		result.Confidence = confidence
	}
	if reason, ok := result.RawResponse["reason"].(string); ok {
		result.Reason = reason
	}

	return &result, nil
}

// extractJSON attempts to extract the first complete JSON object from a response string.
// Uses brace-counting to correctly handle nested objects and trailing content.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return "{}"
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return "{}"
}

// TaskResult represents the parsed result from an LLM task.
type TaskResult struct {
	// Type is the task type this result corresponds to.
	Type TaskType `json:"type"`

	// Classification is the result for TaskClassifyFileRole.
	// Values: "main_chain", "companion", "uncertain", "irrelevant"
	Classification string `json:"classification,omitempty"`

	// Relevance is the result for TaskJudgeRelevance.
	// Values: "relevant", "possibly_relevant", "irrelevant"
	Relevance string `json:"relevance,omitempty"`

	// Decision is the result for TaskShouldExpand.
	// Values: "expand", "skip", "uncertain"
	Decision string `json:"decision,omitempty"`

	// IsImplicit is the result for TaskIsImplicitDependency.
	// Values: "yes", "no", "uncertain"
	IsImplicit string `json:"is_implicit,omitempty"`

	// Confidence is the confidence score from 0.0 to 1.0.
	Confidence float64 `json:"confidence,omitempty"`

	// Reason is the explanation for the result.
	Reason string `json:"reason,omitempty"`

	// RawResponse contains the raw parsed JSON response.
	RawResponse map[string]interface{} `json:"raw_response,omitempty"`

	// RawText is the original response text from the LLM.
	RawText string `json:"-"`
}

// ToJSON returns the TaskResult as JSON bytes.
func (tr *TaskResult) ToJSON() ([]byte, error) {
	return json.Marshal(tr)
}

// GetLabel returns the primary label for the result based on task type.
func (tr *TaskResult) GetLabel() string {
	switch tr.Type {
	case TaskClassifyFileRole:
		return tr.Classification
	case TaskJudgeRelevance:
		return tr.Relevance
	case TaskShouldExpand:
		return tr.Decision
	case TaskIsImplicitDependency:
		return tr.IsImplicit
	default:
		return ""
	}
}

// IsPositive returns true if the result indicates a positive outcome.
func (tr *TaskResult) IsPositive() bool {
	switch tr.Type {
	case TaskClassifyFileRole:
		return tr.Classification == "main_chain" || tr.Classification == "companion"
	case TaskJudgeRelevance:
		return tr.Relevance == "relevant" || tr.Relevance == "possibly_relevant"
	case TaskShouldExpand:
		return tr.Decision == "expand"
	case TaskIsImplicitDependency:
		return tr.IsImplicit == "yes"
	default:
		return false
	}
}
