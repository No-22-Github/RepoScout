# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./cmd/reposcout

# Test
go test ./...

# Run (file-based)
go run ./cmd/reposcout run examples/recon_request.sample.json

# Run (inline)
go run ./cmd/reposcout run --task "..." --repo ./path --seed file.go

# Run with LLM rerank
go run ./cmd/reposcout run examples/recon_request.sample.json --config examples/runtime_config.sample.json

# Evaluate against golden dataset
go run ./cmd/reposcout eval examples/goldens
```

## Architecture

RepoScout is a repository reconnaissance tool for coding agents. Given seed files and a task description, it identifies relevant candidate files and outputs a structured `ContextPack`.

**Pipeline** (in `runner.Run()`):

```
ReconRequest → Scanner.ScanRepo() → ImportGraphBuilder.Build()
    → NeighborExpander.Expand() (5 strategies)
    → FileCardBuilder.Build() (symbols, heuristic scores)
    → [Optional] LLM Rerank
    → Ranker.Rank() (multi-factor scoring)
    → ContextPackBuilder.Build() → ContextPack
```

**Packages:**

| Package | Role |
|---------|------|
| `schema` | Core types: `ReconRequest`, `ContextPack`, `FileCard`, `RiskHint` |
| `scanner` | Repo file scanning, language detection, symbol extraction |
| `heuristics` | 5 static expansion strategies + file scoring rules |
| `ranking` | Multi-factor scoring (seed 0.3, module 0.2, heuristic 0.4, profile 0.3, LLM 0.3) |
| `pack` | Assembles `ContextPack`: classifies main_chain/companion/uncertain, reading order, risk hints |
| `runner` | Orchestrates the full pipeline |
| `llm` | OpenAI-compatible provider adapter for optional reranking |
| `config` | Runtime config loading |
| `cli` | Cobra CLI (`run`, `eval`, `version`, `mcp`) |
| `output` | JSON and Markdown formatters |
| `eval` | Golden dataset evaluation |
| `mcp` | MCP server stub (not yet implemented) |

**Output classification thresholds:** main_chain ≥ 0.5, companion_files 0.3–0.5, uncertain_nodes 0.1–0.3.

**The 5 expansion strategies** (in `heuristics`): same directory, same module (path structure), prefix match, test/impl pairing, import graph connections.
