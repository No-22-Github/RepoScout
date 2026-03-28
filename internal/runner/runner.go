// Package runner orchestrates the full reconnaissance pipeline.
package runner

import (
	"fmt"
	"os"

	"github.com/no22/repo-scout/internal/config"
	"github.com/no22/repo-scout/internal/heuristics"
	"github.com/no22/repo-scout/internal/pack"
	"github.com/no22/repo-scout/internal/ranking"
	"github.com/no22/repo-scout/internal/scanner"
	"github.com/no22/repo-scout/internal/schema"
)

// Runner orchestrates the full reconnaissance pipeline.
type Runner struct {
	config *config.Config
}

// NewRunner creates a new Runner with the given configuration.
func NewRunner(cfg *config.Config) *Runner {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &Runner{config: cfg}
}

// Run executes the full reconnaissance pipeline for a ReconRequest.
func (r *Runner) Run(req *schema.ReconRequest) (*schema.ContextPack, error) {
	// Validate the request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Check if repo root exists
	if _, err := os.Stat(req.RepoRoot); os.IsNotExist(err) {
		return nil, fmt.Errorf("repo root does not exist: %s", req.RepoRoot)
	}

	// Phase 1: Scan repository for all files
	allFiles, err := scanner.ScanRepo(req.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to scan repository: %w", err)
	}

	// Phase 2: Expand seed files to candidate set
	candidates, discoverySources := r.expandCandidates(req.SeedFiles, allFiles)

	// Apply budget limits if specified
	if req.Budget != nil && req.Budget.MaxFiles > 0 && len(candidates) > req.Budget.MaxFiles {
		candidates = candidates[:req.Budget.MaxFiles]
	}

	// Also apply runtime config limit
	if r.config.Runtime.MaxCandidates > 0 && len(candidates) > r.config.Runtime.MaxCandidates {
		candidates = candidates[:r.config.Runtime.MaxCandidates]
	}

	// Phase 3: Build FileCards for all candidates
	cards := r.buildFileCards(candidates, req, discoverySources)

	// Phase 4: Rank candidates
	rankResult := r.rankCards(cards)

	// Phase 5: Build ContextPack
	contextPack := r.buildContextPack(req, rankResult)

	return contextPack, nil
}

// expandCandidates expands seed files to a candidate set.
func (r *Runner) expandCandidates(seedFiles, allFiles []string) ([]string, map[string][]heuristics.ExpansionSource) {
	// If no seed files, return a limited set of files
	if len(seedFiles) == 0 {
		// Limit to MaxCandidates if configured
		limit := r.config.Runtime.MaxCandidates
		if limit <= 0 || limit > 100 {
			limit = 100
		}
		if len(allFiles) > limit {
			return allFiles[:limit], nil
		}
		return allFiles, nil
	}

	// Use neighbor expander
	expander := heuristics.NewNeighborExpander(nil)
	result := expander.ExpandWithSources(seedFiles, allFiles)

	return result.Candidates, result.Sources
}

// buildFileCards creates FileCards for all candidate files.
func (r *Runner) buildFileCards(candidates []string, req *schema.ReconRequest, discoverySources map[string][]heuristics.ExpansionSource) []*schema.FileCard {
	builder := heuristics.NewFileCardBuilder(nil)

	opts := &heuristics.BuildOptions{
		RepoRoot:         req.RepoRoot,
		Profile:          req.Profile,
		FocusChecks:      req.FocusChecks,
		SeedFiles:        req.SeedFiles,
		DiscoverySources: discoverySources,
	}

	return builder.BuildAll(candidates, opts)
}

// rankCards ranks FileCards and returns the result.
func (r *Runner) rankCards(cards []*schema.FileCard) *ranking.RankResult {
	ranker := ranking.NewRanker(nil)
	return ranker.Rank(&ranking.RankInput{Cards: cards})
}

// buildContextPack creates a ContextPack from ranked results.
func (r *Runner) buildContextPack(req *schema.ReconRequest, rankResult *ranking.RankResult) *schema.ContextPack {
	builder := pack.NewBuilder(nil)

	input := &pack.BuildInput{
		Task:       req.Task,
		RankResult: rankResult,
		Request:    req,
	}

	return builder.Build(input)
}

// RunFromPath loads a ReconRequest from a JSON file and runs the pipeline.
func (r *Runner) RunFromPath(path string) (*schema.ContextPack, error) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read request file: %w", err)
	}

	// Parse the request
	req, err := schema.ParseReconRequest(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	return r.Run(req)
}
