package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// createTestGolden creates a temporary golden sample for testing.
func createTestGolden(t *testing.T, tmpDir, id, name string, mainChain, companionFiles []string) string {
	t.Helper()

	sampleDir := filepath.Join(tmpDir, id+"-"+name)
	if err := os.MkdirAll(sampleDir, 0755); err != nil {
		t.Fatalf("Failed to create sample directory: %v", err)
	}

	// Create meta.json
	meta := SampleMeta{
		ID:          id,
		Name:        name,
		Description: "Test sample " + name,
		RepoFamily:  "test",
		Profile:     "test",
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(sampleDir, "meta.json"), metaData, 0644); err != nil {
		t.Fatalf("Failed to write meta.json: %v", err)
	}

	// Create recon_request.json
	req := map[string]interface{}{
		"task":       "Test task for " + name,
		"repo_root":  "/tmp/test",
		"seed_files": []string{"a.go"},
	}
	reqData, _ := json.MarshalIndent(req, "", "  ")
	if err := os.WriteFile(filepath.Join(sampleDir, "recon_request.json"), reqData, 0644); err != nil {
		t.Fatalf("Failed to write recon_request.json: %v", err)
	}

	// Create expected_files.json
	expected := ExpectedFiles{
		MainChain:      mainChain,
		CompanionFiles: companionFiles,
	}
	expectedData, _ := json.MarshalIndent(expected, "", "  ")
	if err := os.WriteFile(filepath.Join(sampleDir, "expected_files.json"), expectedData, 0644); err != nil {
		t.Fatalf("Failed to write expected_files.json: %v", err)
	}

	return sampleDir
}

func TestLoadGoldens(t *testing.T) {
	// Create temporary goldens directory
	tmpDir := t.TempDir()

	// Create test samples
	createTestGolden(t, tmpDir, "001", "test-one", []string{"a.go", "b.go"}, []string{"c.go"})
	createTestGolden(t, tmpDir, "002", "test-two", []string{"d.go"}, []string{"e.go", "f.go"})

	// Create evaluator with a mock runner
	runnerFunc := func(sample *GoldenSample) ([]string, error) {
		return []string{"a.go", "b.go", "c.go"}, nil
	}
	evaluator := NewEvaluator(tmpDir, runnerFunc)

	// Load goldens
	samples, err := evaluator.LoadGoldens()
	if err != nil {
		t.Fatalf("LoadGoldens failed: %v", err)
	}

	if len(samples) != 2 {
		t.Errorf("Expected 2 samples, got %d", len(samples))
	}

	// Check first sample
	if samples[0].ID != "001" {
		t.Errorf("Expected first sample ID '001', got '%s'", samples[0].ID)
	}
	if samples[0].Meta.Name != "test-one" {
		t.Errorf("Expected first sample name 'test-one', got '%s'", samples[0].Meta.Name)
	}
	if len(samples[0].ExpectedFiles.MainChain) != 2 {
		t.Errorf("Expected 2 main_chain files, got %d", len(samples[0].ExpectedFiles.MainChain))
	}

	// Check second sample
	if samples[1].ID != "002" {
		t.Errorf("Expected second sample ID '002', got '%s'", samples[1].ID)
	}
}

func TestRunEvaluation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test sample
	createTestGolden(t, tmpDir, "001", "test-sample",
		[]string{"a.go", "b.go"},
		[]string{"c.go"})

	// Create evaluator with a mock runner that returns known results
	runnerFunc := func(sample *GoldenSample) ([]string, error) {
		// Return files in order: a.go (hit), x.go (extra), b.go (hit), y.go (extra)
		return []string{"a.go", "x.go", "b.go", "y.go", "z.go"}, nil
	}
	evaluator := NewEvaluator(tmpDir, runnerFunc)

	result, err := evaluator.RunEvaluation()
	if err != nil {
		t.Fatalf("RunEvaluation failed: %v", err)
	}

	if result.TotalSamples != 1 {
		t.Errorf("Expected TotalSamples=1, got %d", result.TotalSamples)
	}

	if result.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount=1, got %d", result.SuccessCount)
	}

	if result.ErrorCount != 0 {
		t.Errorf("Expected ErrorCount=0, got %d", result.ErrorCount)
	}

	// Check recall metrics
	// Total expected = 3 (a.go, b.go, c.go)
	// Top 10 hits = 2 (a.go, b.go) -> Recall@10 = 2/3 = 0.667
	// Top 20 hits = 2 (a.go, b.go) -> Recall@20 = 2/3 = 0.667
	// All hits = 2 (a.go, b.go) -> RecallAll = 2/3 = 0.667
	if result.SampleResults[0].RecallAt10 < 0.66 || result.SampleResults[0].RecallAt10 > 0.67 {
		t.Errorf("Expected Recall@10 ~0.667, got %f", result.SampleResults[0].RecallAt10)
	}

	if result.SampleResults[0].RecallAt20 < 0.66 || result.SampleResults[0].RecallAt20 > 0.67 {
		t.Errorf("Expected Recall@20 ~0.667, got %f", result.SampleResults[0].RecallAt20)
	}

	// Check hits and misses
	if len(result.SampleResults[0].Hits) != 2 {
		t.Errorf("Expected 2 hits, got %d", len(result.SampleResults[0].Hits))
	}

	if len(result.SampleResults[0].Misses) != 1 {
		t.Errorf("Expected 1 miss (c.go), got %d", len(result.SampleResults[0].Misses))
	}

	if len(result.SampleResults[0].Extras) != 3 {
		t.Errorf("Expected 3 extras (x.go, y.go, z.go), got %d", len(result.SampleResults[0].Extras))
	}
}

func TestRunEvaluationWithError(t *testing.T) {
	tmpDir := t.TempDir()

	createTestGolden(t, tmpDir, "001", "error-sample", []string{"a.go"}, []string{})

	// Create evaluator with a runner that always errors
	runnerFunc := func(sample *GoldenSample) ([]string, error) {
		return nil, &testError{msg: "simulated error"}
	}
	evaluator := NewEvaluator(tmpDir, runnerFunc)

	result, err := evaluator.RunEvaluation()
	if err != nil {
		t.Fatalf("RunEvaluation should not fail even with runner errors: %v", err)
	}

	if result.ErrorCount != 1 {
		t.Errorf("Expected ErrorCount=1, got %d", result.ErrorCount)
	}

	if result.SampleResults[0].Error == "" {
		t.Error("Expected error message in sample result")
	}
}

func TestPrecisionMetrics(t *testing.T) {
	tmpDir := t.TempDir()

	createTestGolden(t, tmpDir, "001", "precision-test",
		[]string{"a.go", "b.go"},
		[]string{"c.go"})

	// Create evaluator with a mock runner
	runnerFunc := func(sample *GoldenSample) ([]string, error) {
		// Return exactly 5 files: 3 hits + 2 extras
		return []string{"a.go", "x.go", "b.go", "y.go", "c.go"}, nil
	}
	evaluator := NewEvaluator(tmpDir, runnerFunc)

	result, err := evaluator.RunEvaluation()
	if err != nil {
		t.Fatalf("RunEvaluation failed: %v", err)
	}

	sr := result.SampleResults[0]

	// Precision@10: 3 hits in top 5 / 5 = 0.6 (we only have 5 files, so top 10 is limited to 5)
	// Expected files: a.go, b.go, c.go (main_chain + companion_files)
	// Top 10 files: a.go(hit), x.go(extra), b.go(hit), y.go(extra), c.go(hit)
	// Precision = 3 hits / 5 files = 0.6
	if sr.PrecisionAt10 < 0.59 || sr.PrecisionAt10 > 0.61 {
		t.Errorf("Expected Precision@10 ~0.6, got %f", sr.PrecisionAt10)
	}

	// Precision@20: same as Precision@10 since we only have 5 files
	if sr.PrecisionAt20 < 0.59 || sr.PrecisionAt20 > 0.61 {
		t.Errorf("Expected Precision@20 ~0.6, got %f", sr.PrecisionAt20)
	}
}

func TestFormatText(t *testing.T) {
	result := &EvalResult{
		TotalSamples:      2,
		SuccessCount:      2,
		ErrorCount:        0,
		MeanRecallAt10:    0.75,
		MeanRecallAt20:    0.85,
		MeanRecallAll:     0.90,
		MeanPrecisionAt10: 0.60,
		MeanPrecisionAt20: 0.70,
		SampleResults: []*SampleResult{
			{
				SampleID:   "001",
				SampleName: "test-one",
				Hits:       []string{"a.go", "b.go"},
				Misses:     []string{"c.go"},
				Extras:     []string{"x.go"},
				RecallAt10: 0.67,
				RecallAt20: 0.67,
				RecallAll:  0.67,
			},
		},
	}

	text := FormatText(result)

	// Check that key metrics are present
	if !contains(text, "Mean Recall@10") {
		t.Error("Expected 'Mean Recall@10' in output")
	}
	if !contains(text, "75.00%") {
		t.Error("Expected '75.00%' in output")
	}
	if !contains(text, "test-one") {
		t.Error("Expected sample name 'test-one' in output")
	}
}

func TestFormatJSON(t *testing.T) {
	result := &EvalResult{
		TotalSamples:   1,
		SuccessCount:   1,
		ErrorCount:     0,
		MeanRecallAt10: 0.75,
		SampleResults:  []*SampleResult{},
	}

	jsonStr, err := FormatJSON(result)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}

	// Verify it's valid JSON
	var parsed EvalResult
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	if parsed.TotalSamples != 1 {
		t.Errorf("Expected TotalSamples=1 after parsing, got %d", parsed.TotalSamples)
	}
}

func TestEmptyGoldensDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runnerFunc := func(sample *GoldenSample) ([]string, error) {
		return []string{}, nil
	}
	evaluator := NewEvaluator(tmpDir, runnerFunc)

	_, err := evaluator.RunEvaluation()
	if err == nil {
		t.Error("Expected error for empty goldens directory")
	}
}

// Helper types and functions

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
