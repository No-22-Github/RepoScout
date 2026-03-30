// Package main is the CLI entrypoint for reposcout.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/no22/repo-scout/internal/cli"
	"github.com/no22/repo-scout/internal/config"
	"github.com/no22/repo-scout/internal/eval"
	"github.com/no22/repo-scout/internal/output"
	"github.com/no22/repo-scout/internal/runner"
	"github.com/no22/repo-scout/internal/schema"
	"github.com/spf13/cobra"
)

// Version is set by -ldflags at build time.
var version = "dev"

// Command-line flags.
var (
	configPath  string
	quiet       bool
	noColor     bool
	colorOutput bool
)

// Run command flags.
var (
	runTask     string
	runRepo     string
	runSeed     []string
	runProfile  string
	runFocus    []string
	runDepth    int
	runMaxFiles int
	runFormat   string
	runOutput   string
	runRerank   bool
	runNoRerank bool
)

// Eval command flags.
var (
	evalFormat string
	evalSample string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		cli.PrintError("%v", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "reposcout",
	Short: "Repository reconnaissance tool for coding agents",
	Long: `RepoScout helps coding agents understand large codebases by identifying
main chain files, companion files, and suggesting reading order.

It outputs a structured ContextPack that can be used by agents like
Claude Code, Cursor, or human developers to quickly understand a codebase.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Handle color settings
		if noColor {
			cli.ColorEnabled = false
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("reposcout version %s\n", version)
	},
}

var runCmd = &cobra.Command{
	Use:   "run [request.json]",
	Short: "Run repository reconnaissance",
	Long: `Run repository reconnaissance based on a recon request.

Supports two modes:
  1. File mode: Provide a JSON file with the full request
  2. Parameter mode: Provide --task and --repo flags directly

The output is a ContextPack with main chain files, companion files,
reading order, and risk hints.`,
	Example: `  # From JSON file
  reposcout run request.json

  # From JSON file with markdown output
  reposcout run request.json --format markdown

  # Quick scan from command line
  reposcout run --task "Add auth endpoint" --repo ./myproject --seed auth/handler.go

  # Full parameters with profile
  reposcout run --task "Fix login bug" --repo ./project \
    --seed auth/login.go,auth/handler.go \
    --profile bug-fix \
    --focus tests,feature_flag \
    --format markdown

  # Enable LLM reranking
  reposcout run request.json --rerank -c config.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRecon,
}

var evalCmd = &cobra.Command{
	Use:   "eval <dataset_dir>",
	Short: "Evaluate reposcout on a golden dataset",
	Long: `Evaluate reposcout performance against a golden dataset.

Runs reposcout on multiple test cases and reports metrics like
recall@10, recall@20, and precision.`,
	Example: `  # Run all golden samples
  reposcout eval examples/goldens

  # Run specific sample
  reposcout eval examples/goldens --sample 001-browser-settings-toggle

  # JSON output
  reposcout eval examples/goldens --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runEval,
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server",
	Long:  `Start the Model Context Protocol (MCP) server for integration with coding agents.`,
	Example: `  # Start MCP server
  reposcout mcp

  # Start with config
  reposcout mcp -c config.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("MCP server is not yet implemented, see RS-027")
	},
}

func init() {
	// Root flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to runtime config file")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")

	// Run command flags
	runCmd.Flags().StringVarP(&runFormat, "format", "f", "json", "Output format (json or markdown)")
	runCmd.Flags().StringVarP(&runOutput, "output", "o", "", "Output file path (default: stdout)")
	runCmd.Flags().BoolVar(&colorOutput, "color", false, "Colorize JSON output")

	// Parameter mode flags
	runCmd.Flags().StringVarP(&runTask, "task", "t", "", "Task description (required in parameter mode)")
	runCmd.Flags().StringVarP(&runRepo, "repo", "r", "", "Repository root path (required in parameter mode)")
	runCmd.Flags().StringSliceVarP(&runSeed, "seed", "s", nil, "Seed files (comma-separated or multiple flags)")
	runCmd.Flags().StringVarP(&runProfile, "profile", "p", "", "Analysis profile (e.g., browser_settings)")
	runCmd.Flags().StringSliceVar(&runFocus, "focus", nil, "Focus checks (comma-separated, e.g., tests,feature_flag)")
	runCmd.Flags().IntVar(&runDepth, "depth", 0, "Candidate expansion depth (default: 1)")
	runCmd.Flags().IntVar(&runMaxFiles, "max-files", 0, "Maximum output files (default: 20)")
	runCmd.Flags().BoolVar(&runRerank, "rerank", false, "Enable LLM reranking")
	runCmd.Flags().BoolVar(&runNoRerank, "no-rerank", false, "Disable LLM reranking (override config)")

	// Eval command flags
	evalCmd.Flags().StringVarP(&evalFormat, "format", "f", "text", "Output format (text or json)")
	evalCmd.Flags().StringVar(&evalSample, "sample", "", "Run only this sample (by ID)")

	// Add commands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(evalCmd)
	rootCmd.AddCommand(mcpCmd)
}

// runRecon executes the reconnaissance pipeline.
func runRecon(cmd *cobra.Command, args []string) error {
	// Determine input mode
	var req *schema.ReconRequest
	var err error

	if len(args) > 0 {
		// File mode
		req, err = loadRequestFromFile(args[0])
		if err != nil {
			return err
		}
	} else {
		// Parameter mode
		req, err = buildRequestFromFlags()
		if err != nil {
			return err
		}
	}

	// Load configuration
	loadResult, err := config.LoadForRepoWithMeta(configPath, req.RepoRoot)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := loadResult.Config

	// Handle rerank flags
	if runRerank {
		cfg.Runtime.EnableModelRerank = true
	} else if runNoRerank {
		cfg.Runtime.EnableModelRerank = false
	}

	// Create progress reporter
	progress := cli.NewProgressReporter(quiet)
	reportLoadedConfigPaths(loadResult.LoadedPaths)

	// Create runner with progress
	r := runner.NewRunnerWithProgress(cfg, progress)

	// Execute
	contextPack, err := r.Run(req)
	if err != nil {
		return fmt.Errorf("reconnaissance failed: %w", err)
	}

	// Format output
	var out []byte
	switch runFormat {
	case "json":
		out, err = formatJSON(contextPack, colorOutput)
	case "markdown":
		out, err = formatMarkdown(contextPack)
	default:
		return fmt.Errorf("unsupported format: %s (supported: json, markdown)", runFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Write output
	if runOutput != "" {
		if err := os.WriteFile(runOutput, out, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "Output written to: %s\n", runOutput)
		}
	} else {
		fmt.Println(string(out))
	}

	return nil
}

// loadRequestFromFile loads a ReconRequest from a JSON file.
func loadRequestFromFile(path string) (*schema.ReconRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read request file: %w", err)
	}
	return schema.ParseReconRequest(data)
}

// buildRequestFromFlags builds a ReconRequest from command-line flags.
func buildRequestFromFlags() (*schema.ReconRequest, error) {
	if runTask == "" {
		return nil, fmt.Errorf("--task is required when not using a request file")
	}
	if runRepo == "" {
		return nil, fmt.Errorf("--repo is required when not using a request file")
	}

	// Resolve repo path to absolute
	absRepo, err := filepath.Abs(runRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve repo path: %w", err)
	}

	req := &schema.ReconRequest{
		Task:        runTask,
		RepoRoot:    absRepo,
		SeedFiles:   runSeed,
		Profile:     runProfile,
		FocusChecks: runFocus,
	}

	// Set budget if any limits specified
	if runDepth > 0 || runMaxFiles > 0 {
		req.Budget = &schema.Budget{
			ExpandDepth:    runDepth,
			MaxOutputFiles: runMaxFiles,
		}
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	return req, nil
}

// runEval executes the evaluation pipeline.
func runEval(cmd *cobra.Command, args []string) error {
	// Create a runner function for the evaluator
	runnerFunc := func(sample *eval.GoldenSample) ([]string, error) {
		// Parse the recon request from the sample
		reqData, err := json.Marshal(sample.ReconRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal recon request: %w", err)
		}

		req, err := schema.ParseReconRequest(reqData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse recon request: %w", err)
		}

		loadResult, err := config.LoadForRepoWithMeta(configPath, req.RepoRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
		cfg := loadResult.Config
		reportLoadedConfigPaths(loadResult.LoadedPaths)

		// Run the recon
		r := runner.NewRunner(cfg)
		contextPack, err := r.Run(req)
		if err != nil {
			return nil, fmt.Errorf("recon failed: %w", err)
		}

		return contextPack.ReadingOrder, nil
	}

	// Create evaluator
	goldensDir := args[0]
	evaluator := eval.NewEvaluator(goldensDir, runnerFunc)

	// Filter by sample if specified
	if evalSample != "" {
		evaluator = eval.NewSingleSampleEvaluator(goldensDir, evalSample, runnerFunc)
	}

	// Run evaluation
	progress := cli.NewProgressReporter(quiet)
	progress.Start("running evaluation")

	result, err := evaluator.RunEvaluation()
	if err != nil {
		progress.Error(err)
		return fmt.Errorf("evaluation failed: %w", err)
	}

	progress.Done()

	// Format output
	var out string
	switch evalFormat {
	case "json":
		out, err = eval.FormatJSON(result)
	case "text":
		out = formatEvalText(result)
	default:
		return fmt.Errorf("unsupported format: %s (supported: text, json)", evalFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	fmt.Println(out)
	return nil
}

func reportLoadedConfigPaths(paths []string) {
	if quiet {
		return
	}
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "reposcout: config files: none")
		return
	}
	fmt.Fprintf(os.Stderr, "reposcout: config files: %s\n", strings.Join(paths, ", "))
}

// formatJSON formats the ContextPack as JSON.
func formatJSON(pack *schema.ContextPack, colorize bool) ([]byte, error) {
	data, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return nil, err
	}

	if colorize {
		return []byte(cli.HighlightJSON(string(data))), nil
	}
	return data, nil
}

// formatMarkdown formats the ContextPack as Markdown.
func formatMarkdown(pack *schema.ContextPack) ([]byte, error) {
	renderer := output.NewMarkdownRenderer()
	return []byte(renderer.Render(pack)), nil
}

// formatEvalText formats evaluation results with colors.
func formatEvalText(result *eval.EvalResult) string {
	var sb strings.Builder

	sb.WriteString(cli.Bold("=== RepoScout Evaluation Results ===\n\n"))

	// Summary
	sb.WriteString(fmt.Sprintf("Samples: %d total, %s successful, %s errors\n\n",
		result.TotalSamples,
		cli.Green(fmt.Sprintf("%d", result.SuccessCount)),
		func() string {
			if result.ErrorCount > 0 {
				return cli.Red(fmt.Sprintf("%d", result.ErrorCount))
			}
			return "0"
		}(),
	))

	// Metrics
	sb.WriteString(cli.Bold("--- Aggregate Metrics ---\n"))
	sb.WriteString(fmt.Sprintf("  Recall@10:    %s\n", formatPercent(result.MeanRecallAt10)))
	sb.WriteString(fmt.Sprintf("  Recall@20:    %s\n", formatPercent(result.MeanRecallAt20)))
	sb.WriteString(fmt.Sprintf("  Recall (All): %s\n", formatPercent(result.MeanRecallAll)))
	sb.WriteString(fmt.Sprintf("  Precision@10: %s\n", formatPercent(result.MeanPrecisionAt10)))
	sb.WriteString(fmt.Sprintf("  Precision@20: %s\n\n", formatPercent(result.MeanPrecisionAt20)))

	// Sample details
	sb.WriteString(cli.Bold("--- Sample Details ---\n\n"))

	for _, sr := range result.SampleResults {
		sb.WriteString(fmt.Sprintf("[%s] %s\n",
			cli.Cyan(sr.SampleID),
			sr.SampleName,
		))

		if sr.Error != "" {
			sb.WriteString(fmt.Sprintf("  %s %s\n\n", cli.Red("ERROR:"), sr.Error))
			continue
		}

		sb.WriteString(fmt.Sprintf("  Recall@10: %s  Recall@20: %s  Recall: %s\n",
			formatPercent(sr.RecallAt10),
			formatPercent(sr.RecallAt20),
			formatPercent(sr.RecallAll),
		))
		sb.WriteString(fmt.Sprintf("  Hits: %s  Misses: %s  Extras: %s\n",
			cli.Green(fmt.Sprintf("%d", len(sr.Hits))),
			cli.Red(fmt.Sprintf("%d", len(sr.Misses))),
			cli.Yellow(fmt.Sprintf("%d", len(sr.Extras))),
		))

		if len(sr.Hits) > 0 {
			sb.WriteString(fmt.Sprintf("  Hits: %s\n", truncateList(sr.Hits, 5)))
		}
		if len(sr.Misses) > 0 {
			sb.WriteString(fmt.Sprintf("  Misses: %s\n", cli.Red(truncateList(sr.Misses, 5))))
		}
		if len(sr.Extras) > 0 {
			sb.WriteString(fmt.Sprintf("  Extras: %s\n", cli.Gray(truncateList(sr.Extras, 5))))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatPercent formats a float as a percentage string with color.
func formatPercent(v float64) string {
	pct := v * 100
	s := fmt.Sprintf("%.1f%%", pct)
	if pct >= 70 {
		return cli.Green(s)
	} else if pct >= 40 {
		return cli.Yellow(s)
	}
	return cli.Red(s)
}

// truncateList truncates a list for display.
func truncateList(items []string, max int) string {
	if len(items) <= max {
		return fmt.Sprintf("%v", items)
	}
	return fmt.Sprintf("%v ... and %d more", items[:max], len(items)-max)
}
