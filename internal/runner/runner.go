// Package runner orchestrates the full reconnaissance pipeline.
package runner

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/no22/repo-scout/internal/config"
	"github.com/no22/repo-scout/internal/heuristics"
	"github.com/no22/repo-scout/internal/llm"
	"github.com/no22/repo-scout/internal/pack"
	"github.com/no22/repo-scout/internal/ranking"
	"github.com/no22/repo-scout/internal/scanner"
	"github.com/no22/repo-scout/internal/schema"
)

// Runner orchestrates the full reconnaissance pipeline.
type Runner struct {
	config         *config.Config
	adapter        llm.ProviderAdapter
	adapterFactory func(*config.Config) llm.ProviderAdapter
}

// NewRunner creates a new Runner with the given configuration.
func NewRunner(cfg *config.Config) *Runner {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &Runner{
		config: cfg,
		adapterFactory: func(cfg *config.Config) llm.ProviderAdapter {
			if cfg == nil || !cfg.Runtime.EnableModelRerank || cfg.Provider.BaseURL == "" {
				return nil
			}
			return llm.NewOpenAICompatibleAdapter(llm.AdapterConfigFromConfig(cfg))
		},
	}
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
	candidates, discoverySources := r.expandCandidates(req, allFiles)

	// Also apply runtime config limit
	if r.config.Runtime.MaxCandidates > 0 && len(candidates) > r.config.Runtime.MaxCandidates {
		candidates = candidates[:r.config.Runtime.MaxCandidates]
	}

	// Phase 3: Build FileCards for all candidates
	cards := r.buildFileCards(candidates, req, discoverySources)

	// Phase 4: Optionally enrich cards with model judgments and rank candidates.
	modelEnhanced := r.applyLLMRerank(req, cards)
	rankResult := r.rankCards(cards)

	// Phase 5: Build ContextPack
	contextPack := r.buildContextPack(req, rankResult, modelEnhanced)

	return contextPack, nil
}

// expandCandidates expands seed files to a candidate set.
func (r *Runner) expandCandidates(req *schema.ReconRequest, allFiles []string) ([]string, map[string][]heuristics.ExpansionSource) {
	seedFiles := req.SeedFiles
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

	// Use iterative neighbor expansion for the configured depth.
	expander := heuristics.NewNeighborExpander(nil)
	depth := req.EffectiveExpandDepth()
	maxNeighbors := req.EffectiveMaxSeedNeighbors()

	candidateSet := make(map[string]bool)
	ordered := make([]string, 0, len(seedFiles))
	sources := make(map[string][]heuristics.ExpansionSource)
	frontier := make([]string, 0, len(seedFiles))

	for _, seed := range seedFiles {
		if !candidateSet[seed] {
			candidateSet[seed] = true
			ordered = append(ordered, seed)
		}
		sources[seed] = appendUniqueExpansionSources(sources[seed], heuristics.SourceSeed)
		frontier = append(frontier, seed)
	}

	for i := 0; i < depth && len(frontier) > 0; i++ {
		result := expander.ExpandWithSources(frontier, allFiles)
		nextFrontier := make([]string, 0, len(result.Candidates))
		for _, path := range result.Candidates {
			if !candidateSet[path] {
				if maxNeighbors > 0 && len(ordered)-len(seedFiles) >= maxNeighbors {
					continue
				}
				candidateSet[path] = true
				ordered = append(ordered, path)
				nextFrontier = append(nextFrontier, path)
			}
			sources[path] = appendUniqueExpansionSources(sources[path], result.Sources[path]...)
		}
		frontier = nextFrontier
	}

	sort.Strings(ordered[len(seedFiles):])
	return ordered, sources
}

// buildFileCards creates FileCards for all candidate files.
func (r *Runner) buildFileCards(candidates []string, req *schema.ReconRequest, discoverySources map[string][]heuristics.ExpansionSource) []*schema.FileCard {
	builder := heuristics.NewFileCardBuilder(nil)

	opts := &heuristics.BuildOptions{
		RepoRoot:         req.RepoRoot,
		Profile:          req.Profile,
		FocusChecks:      req.FocusChecks,
		SeedFiles:        req.SeedFiles,
		FocusSymbols:     req.FocusSymbols,
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
func (r *Runner) buildContextPack(req *schema.ReconRequest, rankResult *ranking.RankResult, modelEnhanced bool) *schema.ContextPack {
	builder := pack.NewBuilder(r.builderConfig(req))

	input := &pack.BuildInput{
		Task:          req.Task,
		RankResult:    rankResult,
		Request:       req,
		ModelEnhanced: modelEnhanced,
	}

	return builder.Build(input)
}

func (r *Runner) builderConfig(req *schema.ReconRequest) *pack.BuilderConfig {
	cfg := pack.DefaultBuilderConfig()

	maxOutputFiles := r.config.Runtime.MaxOutputFiles
	if reqMax := req.EffectiveMaxOutputFiles(); reqMax > 0 {
		maxOutputFiles = reqMax
	}
	cfg.MaxTotalFiles = maxOutputFiles

	return cfg
}

func (r *Runner) applyLLMRerank(req *schema.ReconRequest, cards []*schema.FileCard) bool {
	if !r.config.Runtime.EnableModelRerank || len(cards) == 0 {
		return false
	}

	adapter := r.adapter
	if adapter == nil && r.adapterFactory != nil {
		adapter = r.adapterFactory(r.config)
	}
	if adapter == nil || !adapter.IsAvailable() {
		return false
	}

	preRank := r.rankCards(cards)
	targetCards := preRank.Cards
	if maxJobs := req.EffectiveMaxLLMJobs(); maxJobs > 0 && len(targetCards) > maxJobs {
		targetCards = targetCards[:maxJobs]
	}

	taskCards := make([]*llm.TaskCard, 0, len(targetCards))
	for _, card := range targetCards {
		taskCard := llm.NewTaskCardFromRequest(llm.TaskClassifyFileRole, req, card)
		taskCard.SetContextSnippet(buildTaskContext(card))
		taskCards = append(taskCards, taskCard)
	}

	pool := llm.NewWorkerPool(&llm.WorkerPoolConfig{
		Adapter:          adapter,
		MaxConcurrency:   r.config.Runtime.MaxConcurrency,
		StopOnFirstError: false,
	})
	result := pool.Execute(context.Background(), taskCards)
	successful := 0
	for i, taskResult := range result.Results {
		if taskResult == nil {
			continue
		}
		applyTaskResult(targetCards[i], taskResult)
		successful++
	}
	return successful > 0
}

func buildTaskContext(card *schema.FileCard) string {
	if card == nil {
		return ""
	}

	context := ""
	if len(card.DiscoveredBy) > 0 {
		context += "Discovered by: " + joinLimited(card.DiscoveredBy, 4) + "\n"
	}
	if len(card.HeuristicTags) > 0 {
		context += "Heuristic tags: " + joinLimited(card.HeuristicTags, 4) + "\n"
	}
	return context
}

func joinLimited(values []string, limit int) string {
	if len(values) == 0 {
		return ""
	}
	if limit <= 0 || len(values) <= limit {
		return fmt.Sprintf("%v", values)
	}
	return fmt.Sprintf("%v", values[:limit])
}

func applyTaskResult(card *schema.FileCard, result *llm.TaskResult) {
	if card == nil || card.Scores == nil || result == nil {
		return
	}
	card.Scores.LLMLabel = result.GetLabel()
	card.Scores.LLMConfidence = result.Confidence
}

func appendUniqueExpansionSources(existing []heuristics.ExpansionSource, additions ...heuristics.ExpansionSource) []heuristics.ExpansionSource {
	seen := make(map[heuristics.ExpansionSource]bool, len(existing))
	for _, source := range existing {
		seen[source] = true
	}
	for _, source := range additions {
		if !seen[source] {
			existing = append(existing, source)
			seen[source] = true
		}
	}
	return existing
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
