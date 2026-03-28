// Package main is the CLI entrypoint for reposcout.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/no22/repo-scout/internal/config"
	"github.com/no22/repo-scout/internal/eval"
	"github.com/no22/repo-scout/internal/output"
	"github.com/no22/repo-scout/internal/runner"
	"github.com/no22/repo-scout/internal/schema"
	"github.com/spf13/cobra"
)

var (
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "reposcout",
	Short: "RepoScout is a repository reconnaissance tool for coding agents",
	Long: `RepoScout helps coding agents understand large codebases by:
  - Identifying main chain files
  - Finding high-probability companion files
  - Suggesting reading order
  - Providing risk hints

It outputs a structured ContextPack for agents like Codex, Claude Code, and Cursor.`,
}

var runCmd = &cobra.Command{
	Use:   "run <recon_request.json>",
	Short: "Run repository reconnaissance",
	Long: `Run repository reconnaissance based on a recon request JSON file.

Outputs a ContextPack with main chain files, companion files, reading order, and risk hints.`,
	Args: cobra.ExactArgs(1),
	RunE: runRecon,
}

var evalCmd = &cobra.Command{
	Use:   "eval <dataset_dir>",
	Short: "Evaluate reposcout on a golden dataset",
	Long: `Evaluate reposcout performance against a golden dataset.

Runs reposcout on multiple test cases and reports metrics like recall and precision.`,
	Args: cobra.ExactArgs(1),
	RunE: runEval,
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server",
	Long:  `Start the Model Context Protocol server for reposcout.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting MCP server...")
		// TODO: Implement actual MCP server logic
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to runtime config file")
	runCmd.Flags().StringP("format", "f", "json", "Output format (json or markdown)")
	runCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	evalCmd.Flags().StringP("format", "f", "text", "Output format (text or json)")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(evalCmd)
	rootCmd.AddCommand(mcpCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runRecon executes the reconnaissance pipeline.
func runRecon(cmd *cobra.Command, args []string) error {
	// Get flags
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return fmt.Errorf("failed to get format flag: %w", err)
	}

	outputPath, err := cmd.Flags().GetString("output")
	if err != nil {
		return fmt.Errorf("failed to get output flag: %w", err)
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create runner and execute
	r := runner.NewRunner(cfg)
	contextPack, err := r.RunFromPath(args[0])
	if err != nil {
		return fmt.Errorf("reconnaissance failed: %w", err)
	}

	// Format output
	var out []byte
	switch format {
	case "json":
		out, err = formatJSON(contextPack)
	case "markdown":
		out, err = formatMarkdown(contextPack)
	default:
		return fmt.Errorf("unsupported format: %s (supported: json, markdown)", format)
	}

	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Write output
	if outputPath != "" {
		if err := os.WriteFile(outputPath, out, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Output written to: %s\n", outputPath)
	} else {
		fmt.Println(string(out))
	}

	return nil
}

// runEval executes the evaluation pipeline.
func runEval(cmd *cobra.Command, args []string) error {
	// Get flags
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return fmt.Errorf("failed to get format flag: %w", err)
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

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

		// Run the recon
		r := runner.NewRunner(cfg)
		contextPack, err := r.Run(req)
		if err != nil {
			return nil, fmt.Errorf("recon failed: %w", err)
		}

		// Return all files in reading order (which combines main_chain and companion_files)
		return contextPack.ReadingOrder, nil
	}

	// Create evaluator and run
	evaluator := eval.NewEvaluator(args[0], runnerFunc)
	result, err := evaluator.RunEvaluation()
	if err != nil {
		return fmt.Errorf("evaluation failed: %w", err)
	}

	// Format output
	var out string
	switch format {
	case "json":
		out, err = eval.FormatJSON(result)
	case "text":
		out = eval.FormatText(result)
	default:
		return fmt.Errorf("unsupported format: %s (supported: text, json)", format)
	}

	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	fmt.Println(out)
	return nil
}

// formatJSON formats the ContextPack as JSON.
func formatJSON(pack *schema.ContextPack) ([]byte, error) {
	return json.MarshalIndent(pack, "", "  ")
}

// formatMarkdown formats the ContextPack as Markdown using the output renderer.
func formatMarkdown(pack *schema.ContextPack) ([]byte, error) {
	renderer := output.NewMarkdownRenderer()
	return []byte(renderer.Render(pack)), nil
}
