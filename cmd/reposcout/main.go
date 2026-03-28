// Package main is the CLI entrypoint for reposcout.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
	Run: func(cmd *cobra.Command, args []string) {
		format, _ := cmd.Flags().GetString("format")
		fmt.Printf("Running recon with request: %s (format: %s)\n", args[0], format)
		// TODO: Implement actual reconnaissance logic
	},
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
	runCmd.Flags().StringP("format", "f", "json", "Output format (json or markdown)")
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
