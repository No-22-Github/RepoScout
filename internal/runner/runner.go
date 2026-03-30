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

// ProgressReporter reports progress of pipeline phases.
type ProgressReporter interface {
	Start(phase string)
	Startf(format string, args ...any)
	Done()
	DoneWithCount(count int, label string)
	Infof(format string, args ...any)
}

// noopProgress is a no-op progress reporter.
type noopProgress struct{}

func (n *noopProgress) Start(string)              {}
func (n *noopProgress) Startf(string, ...any)     {}
func (n *noopProgress) Done()                     {}
func (n *noopProgress) DoneWithCount(int, string) {}
func (n *noopProgress) Infof(string, ...any)      {}

// Runner orchestrates the full reconnaissance pipeline.
type Runner struct {
	config         *config.Config
	adapter        llm.ProviderAdapter
	adapterFactory func(*config.Config) llm.ProviderAdapter
	progress       ProgressReporter
}

// NewRunner creates a new Runner with the given configuration.
func NewRunner(cfg *config.Config) *Runner {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &Runner{
		config:   cfg,
		progress: &noopProgress{},
		adapterFactory: func(cfg *config.Config) llm.ProviderAdapter {
			if cfg == nil || !cfg.Runtime.EnableModelRerank || cfg.Provider.BaseURL == "" {
				return nil
			}
			return llm.NewOpenAICompatibleAdapter(llm.AdapterConfigFromConfig(cfg))
		},
	}
}

// NewRunnerWithProgress creates a new Runner with progress reporting.
func NewRunnerWithProgress(cfg *config.Config, progress ProgressReporter) *Runner {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	if progress == nil {
		progress = &noopProgress{}
	}
	return &Runner{
		config:   cfg,
		progress: progress,
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
	r.progress.Start("scanning repository")
	allFiles, err := scanner.ScanRepo(req.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to scan repository: %w", err)
	}
	r.progress.DoneWithCount(len(allFiles), "files")

	// Phase 1b: Build import graph for static expansion
	r.progress.Start("building import graph")
	importGraph := heuristics.NewImportGraphBuilder(req.RepoRoot).Build(allFiles)
	r.progress.Done()

	// Phase 2: Expand seed files to candidate set
	r.progress.Startf("expanding candidates (depth=%d)", req.EffectiveExpandDepth())
	candidates, discoverySources := r.expandCandidates(req, allFiles, importGraph)

	// Also apply runtime config limit
	if r.config.Runtime.MaxCandidates > 0 && len(candidates) > r.config.Runtime.MaxCandidates {
		candidates = candidates[:r.config.Runtime.MaxCandidates]
	}
	r.progress.DoneWithCount(len(candidates), "candidates")

	// Phase 3: Build FileCards for all candidates
	r.progress.Start("building file cards")
	cards := r.buildFileCards(candidates, req, discoverySources, importGraph)
	r.progress.DoneWithCount(len(cards), "cards")

	// Phase 4: Optionally enrich cards with model judgments and rank candidates.
	modelEnhanced := r.applyLLMRerank(req, cards)
	r.progress.Start("ranking candidates")
	rankResult := r.rankCards(cards)
	r.progress.Done()

	// Phase 5: Build ContextPack
	r.progress.Start("building context pack")
	contextPack := r.buildContextPack(req, rankResult, modelEnhanced)
	r.progress.Done()

	return contextPack, nil
}

// expandCandidates expands seed files to a candidate set.
func (r *Runner) expandCandidates(req *schema.ReconRequest, allFiles []string, importGraph *heuristics.ImportGraph) ([]string, map[string][]heuristics.ExpansionSource) {
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
	expander := heuristics.NewNeighborExpander(nil).WithImportGraph(importGraph)
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
func (r *Runner) buildFileCards(candidates []string, req *schema.ReconRequest, discoverySources map[string][]heuristics.ExpansionSource, importGraph *heuristics.ImportGraph) []*schema.FileCard {
	builder := heuristics.NewFileCardBuilder(nil)
	neighborMap := make(map[string][]string, len(candidates))
	if importGraph != nil {
		for _, candidate := range candidates {
			neighbors := importGraph.Neighbors(candidate)
			if len(neighbors) == 0 {
				continue
			}
			sort.Strings(neighbors)
			neighborMap[candidate] = neighbors
		}
	}

	opts := &heuristics.BuildOptions{
		RepoRoot:         req.RepoRoot,
		Profile:          req.Profile,
		FocusChecks:      req.FocusChecks,
		SeedFiles:        req.SeedFiles,
		FocusSymbols:     req.FocusSymbols,
		DiscoverySources: discoverySources,
		NeighborMap:      neighborMap,
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

	const maxNeighborsInPrompt = 8

	taskCards := make([]*llm.TaskCard, 0, len(targetCards))
	for _, card := range targetCards {
		taskCard := llm.NewTaskCardFromRequest(llm.TaskClassifyFileRole, req, card)
		taskCard.FileNeighbors = selectTopNeighbors(card.Neighbors, req.SeedFiles, maxNeighborsInPrompt)
		maxContextTokens := availableContextTokens(r.config, taskCard)
		taskCard.SetContextSnippet(buildTaskContext(req.RepoRoot, card, req.FocusSymbols, maxContextTokens))
		taskCards = append(taskCards, taskCard)
	}

	pool := llm.NewWorkerPool(&llm.WorkerPoolConfig{
		Adapter:          adapter,
		MaxConcurrency:   r.config.Runtime.MaxConcurrency,
		StopOnFirstError: false,
	})
	result := pool.Execute(context.Background(), taskCards)
	r.progress.Infof(
		"llm rerank: requested=%d succeeded=%d failed=%d",
		result.TotalTasks,
		result.SuccessfulTasks,
		result.FailedTasks,
	)
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

func availableContextTokens(cfg *config.Config, taskCard *llm.TaskCard) int {
	if cfg == nil || taskCard == nil {
		return 0
	}

	reserved := estimateTokenCount(taskCard.ToPrompt())
	reserved += estimateTokenCount(llm.LoadSystemPrompt(cfg.Provider.SystemPromptPath))
	reserved += estimateTokenCount("## Context\n\n")

	remaining := cfg.Runtime.MaxInputTokens - reserved
	if remaining < 0 {
		return 0
	}
	return remaining
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

// selectTopNeighbors returns up to max neighbors, prioritizing seed files first.
func selectTopNeighbors(neighbors, seedFiles []string, max int) []string {
	if len(neighbors) == 0 || max <= 0 {
		return nil
	}
	seedSet := make(map[string]bool, len(seedFiles))
	for _, s := range seedFiles {
		seedSet[s] = true
	}
	result := make([]string, 0, max)
	// Priority 1: neighbors that are seed files
	for _, n := range neighbors {
		if len(result) >= max {
			break
		}
		if seedSet[n] {
			result = append(result, n)
		}
	}
	// Priority 2: remaining neighbors
	for _, n := range neighbors {
		if len(result) >= max {
			break
		}
		if !seedSet[n] {
			result = append(result, n)
		}
	}
	return result
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
