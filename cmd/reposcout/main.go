// Package main is the CLI entrypoint for reposcout.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/no22/repo-scout/internal/config"
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
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Running evaluation on dataset: %s\n", args[0])
		// TODO: Implement actual evaluation logic
	},
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
	var output []byte
	switch format {
	case "json":
		output, err = formatJSON(contextPack)
	case "markdown":
		output, err = formatMarkdown(contextPack)
	default:
		return fmt.Errorf("unsupported format: %s (supported: json, markdown)", format)
	}

	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Write output
	if outputPath != "" {
		if err := os.WriteFile(outputPath, output, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Output written to: %s\n", outputPath)
	} else {
		fmt.Println(string(output))
	}

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
