// Package eval provides evaluation utilities for measuring RepoScout performance.
package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExpectedFiles represents the ground truth files for a golden sample.
type ExpectedFiles struct {
	MainChain      []string `json:"main_chain"`
	CompanionFiles []string `json:"companion_files,omitempty"`
	OptionalFiles  []string `json:"optional_files,omitempty"`
	ExcludedFiles  []string `json:"excluded_files,omitempty"`
	Notes          string   `json:"notes,omitempty"`
}

// SampleMeta contains metadata for a golden sample.
type SampleMeta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RepoFamily  string `json:"repo_family"`
	Profile     string `json:"profile"`
	CreatedAt   string `json:"created_at,omitempty"`
	Difficulty  string `json:"difficulty,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// GoldenSample represents a single golden test sample.
type GoldenSample struct {
	ID            string
	Path          string
	Meta          *SampleMeta
	ReconRequest  map[string]interface{}
	ExpectedFiles *ExpectedFiles
}

// SampleResult contains evaluation results for a single sample.
type SampleResult struct {
	SampleID      string   `json:"sample_id"`
	SampleName    string   `json:"sample_name"`
	Hits          []string `json:"hits"`
	Misses        []string `json:"misses"`
	Extras        []string `json:"extras"`
	RecallAt10    float64  `json:"recall_at_10"`
	RecallAt20    float64  `json:"recall_at_20"`
	RecallAll     float64  `json:"recall_all"`
	PrecisionAt10 float64  `json:"precision_at_10"`
	PrecisionAt20 float64  `json:"precision_at_20"`
	Error         string   `json:"error,omitempty"`
}

// EvalResult contains the overall evaluation results.
type EvalResult struct {
	TotalSamples      int             `json:"total_samples"`
	SuccessCount      int             `json:"success_count"`
	ErrorCount        int             `json:"error_count"`
	MeanRecallAt10    float64         `json:"mean_recall_at_10"`
	MeanRecallAt20    float64         `json:"mean_recall_at_20"`
	MeanRecallAll     float64         `json:"mean_recall_all"`
	MeanPrecisionAt10 float64         `json:"mean_precision_at_10"`
	MeanPrecisionAt20 float64         `json:"mean_precision_at_20"`
	SampleResults     []*SampleResult `json:"sample_results"`
}

// RunnerFunc is a function that runs RepoScout on a golden sample.
// It returns the ranked file paths (in order) and an error.
type RunnerFunc func(sample *GoldenSample) ([]string, error)

// Evaluator evaluates RepoScout performance against golden samples.
type Evaluator struct {
	goldensDir     string
	runner         RunnerFunc
	singleSampleID string // If set, only evaluate this sample
}

// NewEvaluator creates a new Evaluator.
func NewEvaluator(goldensDir string, runner RunnerFunc) *Evaluator {
	return &Evaluator{
		goldensDir: goldensDir,
		runner:     runner,
	}
}

// NewSingleSampleEvaluator creates an Evaluator that only runs a specific sample.
func NewSingleSampleEvaluator(goldensDir, sampleID string, runner RunnerFunc) *Evaluator {
	return &Evaluator{
		goldensDir:     goldensDir,
		runner:         runner,
		singleSampleID: sampleID,
	}
}

// LoadGoldens loads all golden samples from the goldens directory.
func (e *Evaluator) LoadGoldens() ([]*GoldenSample, error) {
	entries, err := os.ReadDir(e.goldensDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read goldens directory: %w", err)
	}

	var samples []*GoldenSample
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip hidden directories
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		samplePath := filepath.Join(e.goldensDir, entry.Name())
		sample, err := e.loadSample(samplePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load sample %s: %w", entry.Name(), err)
		}

		// Filter by single sample ID if set
		if e.singleSampleID != "" && sample.ID != e.singleSampleID {
			continue
		}

		samples = append(samples, sample)
	}

	// Sort by ID
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].ID < samples[j].ID
	})

	return samples, nil
}

// loadSample loads a single golden sample from its directory.
func (e *Evaluator) loadSample(path string) (*GoldenSample, error) {
	// Load meta.json
	metaPath := filepath.Join(path, "meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read meta.json: %w", err)
	}

	var meta SampleMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse meta.json: %w", err)
	}

	// Load recon_request.json
	reqPath := filepath.Join(path, "recon_request.json")
	reqData, err := os.ReadFile(reqPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read recon_request.json: %w", err)
	}

	var reconRequest map[string]interface{}
	if err := json.Unmarshal(reqData, &reconRequest); err != nil {
		return nil, fmt.Errorf("failed to parse recon_request.json: %w", err)
	}

	// Load expected_files.json
	expectedPath := filepath.Join(path, "expected_files.json")
	expectedData, err := os.ReadFile(expectedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read expected_files.json: %w", err)
	}

	var expected ExpectedFiles
	if err := json.Unmarshal(expectedData, &expected); err != nil {
		return nil, fmt.Errorf("failed to parse expected_files.json: %w", err)
	}

	return &GoldenSample{
		ID:            meta.ID,
		Path:          path,
		Meta:          &meta,
		ReconRequest:  reconRequest,
		ExpectedFiles: &expected,
	}, nil
}

// RunEvaluation runs the evaluation on all golden samples.
func (e *Evaluator) RunEvaluation() (*EvalResult, error) {
	samples, err := e.LoadGoldens()
	if err != nil {
		return nil, fmt.Errorf("failed to load goldens: %w", err)
	}

	if len(samples) == 0 {
		return nil, fmt.Errorf("no golden samples found in %s", e.goldensDir)
	}

	result := &EvalResult{
		TotalSamples:  len(samples),
		SampleResults: make([]*SampleResult, 0, len(samples)),
	}

	var totalRecallAt10, totalRecallAt20, totalRecallAll float64
	var totalPrecisionAt10, totalPrecisionAt20 float64

	for _, sample := range samples {
		sampleResult := e.evaluateSample(sample)
		result.SampleResults = append(result.SampleResults, sampleResult)

		if sampleResult.Error != "" {
			result.ErrorCount++
		} else {
			result.SuccessCount++
			totalRecallAt10 += sampleResult.RecallAt10
			totalRecallAt20 += sampleResult.RecallAt20
			totalRecallAll += sampleResult.RecallAll
			totalPrecisionAt10 += sampleResult.PrecisionAt10
			totalPrecisionAt20 += sampleResult.PrecisionAt20
		}
	}

	// Calculate mean metrics (only from successful samples)
	if result.SuccessCount > 0 {
		result.MeanRecallAt10 = totalRecallAt10 / float64(result.SuccessCount)
		result.MeanRecallAt20 = totalRecallAt20 / float64(result.SuccessCount)
		result.MeanRecallAll = totalRecallAll / float64(result.SuccessCount)
		result.MeanPrecisionAt10 = totalPrecisionAt10 / float64(result.SuccessCount)
		result.MeanPrecisionAt20 = totalPrecisionAt20 / float64(result.SuccessCount)
	}

	return result, nil
}

// evaluateSample evaluates a single golden sample.
func (e *Evaluator) evaluateSample(sample *GoldenSample) *SampleResult {
	result := &SampleResult{
		SampleID:   sample.ID,
		SampleName: sample.Meta.Name,
		Hits:       []string{},
		Misses:     []string{},
		Extras:     []string{},
	}

	// Run the recon
	rankedFiles, err := e.runner(sample)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Get expected files (main_chain + companion_files)
	expectedSet := make(map[string]bool)
	for _, f := range sample.ExpectedFiles.MainChain {
		expectedSet[f] = true
	}
	for _, f := range sample.ExpectedFiles.CompanionFiles {
		expectedSet[f] = true
	}

	// Calculate hits and misses
	foundSet := make(map[string]bool)
	for _, f := range rankedFiles {
		foundSet[f] = true
		if expectedSet[f] {
			result.Hits = append(result.Hits, f)
		} else {
			result.Extras = append(result.Extras, f)
		}
	}

	// Find misses (expected but not found)
	for f := range expectedSet {
		if !foundSet[f] {
			result.Misses = append(result.Misses, f)
		}
	}

	// Calculate recall metrics
	totalExpected := len(expectedSet)
	if totalExpected > 0 {
		// Recall@10
		top10 := min(10, len(rankedFiles))
		hitsAt10 := 0
		for i := 0; i < top10; i++ {
			if expectedSet[rankedFiles[i]] {
				hitsAt10++
			}
		}
		result.RecallAt10 = float64(hitsAt10) / float64(totalExpected)

		// Recall@20
		top20 := min(20, len(rankedFiles))
		hitsAt20 := 0
		for i := 0; i < top20; i++ {
			if expectedSet[rankedFiles[i]] {
				hitsAt20++
			}
		}
		result.RecallAt20 = float64(hitsAt20) / float64(totalExpected)

		// Overall recall
		result.RecallAll = float64(len(result.Hits)) / float64(totalExpected)
	}

	// Calculate precision metrics
	top10 := min(10, len(rankedFiles))
	if top10 > 0 {
		hitsAt10 := 0
		for i := 0; i < top10; i++ {
			if expectedSet[rankedFiles[i]] {
				hitsAt10++
			}
		}
		result.PrecisionAt10 = float64(hitsAt10) / float64(top10)
	}

	top20 := min(20, len(rankedFiles))
	if top20 > 0 {
		hitsAt20 := 0
		for i := 0; i < top20; i++ {
			if expectedSet[rankedFiles[i]] {
				hitsAt20++
			}
		}
		result.PrecisionAt20 = float64(hitsAt20) / float64(top20)
	}

	return result
}

// FormatText formats the evaluation result as human-readable text.
func FormatText(result *EvalResult) string {
	var sb strings.Builder

	sb.WriteString("=== RepoScout Evaluation Results ===\n\n")

	sb.WriteString(fmt.Sprintf("Total Samples: %d\n", result.TotalSamples))
	sb.WriteString(fmt.Sprintf("Successful: %d\n", result.SuccessCount))
	sb.WriteString(fmt.Sprintf("Errors: %d\n\n", result.ErrorCount))

	sb.WriteString("--- Aggregate Metrics ---\n")
	sb.WriteString(fmt.Sprintf("Mean Recall@10:     %.2f%%\n", result.MeanRecallAt10*100))
	sb.WriteString(fmt.Sprintf("Mean Recall@20:     %.2f%%\n", result.MeanRecallAt20*100))
	sb.WriteString(fmt.Sprintf("Mean Recall (All):  %.2f%%\n", result.MeanRecallAll*100))
	sb.WriteString(fmt.Sprintf("Mean Precision@10:  %.2f%%\n", result.MeanPrecisionAt10*100))
	sb.WriteString(fmt.Sprintf("Mean Precision@20:  %.2f%%\n\n", result.MeanPrecisionAt20*100))

	sb.WriteString("--- Sample Details ---\n\n")

	for _, sr := range result.SampleResults {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", sr.SampleID, sr.SampleName))

		if sr.Error != "" {
			sb.WriteString(fmt.Sprintf("  ERROR: %s\n\n", sr.Error))
			continue
		}

		sb.WriteString(fmt.Sprintf("  Recall@10: %.2f%%  Recall@20: %.2f%%  Recall: %.2f%%\n",
			sr.RecallAt10*100, sr.RecallAt20*100, sr.RecallAll*100))
		sb.WriteString(fmt.Sprintf("  Hits: %d  Misses: %d  Extras: %d\n",
			len(sr.Hits), len(sr.Misses), len(sr.Extras)))

		if len(sr.Hits) > 0 {
			sb.WriteString(fmt.Sprintf("  Hits: %v\n", sr.Hits))
		}
		if len(sr.Misses) > 0 {
			sb.WriteString(fmt.Sprintf("  Misses: %v\n", sr.Misses))
		}
		if len(sr.Extras) > 0 {
			sb.WriteString(fmt.Sprintf("  Extras (first 5): %v\n", firstN(sr.Extras, 5)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatJSON formats the evaluation result as JSON.
func FormatJSON(result *EvalResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// firstN returns the first n elements of a slice.
func firstN(slice []string, n int) []string {
	if len(slice) <= n {
		return slice
	}
	return slice[:n]
}
