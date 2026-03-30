# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 常用命令

```bash
# 构建
go build ./cmd/reposcout/...

# 运行测试
go test ./...

# 运行单个包的测试
go test ./internal/ranking/...

# 运行（从请求文件）
go run ./cmd/reposcout run examples/recon_request.sample.json

# 运行（直接传参）
go run ./cmd/reposcout run --task "Add auth endpoint" --repo . --seed internal/runner/runner.go

# 评估 golden dataset
go run ./cmd/reposcout eval examples/goldens
```

## 架构概览

RepoScout 是一个给编程 Agent 用的仓库侦察工具。输入少量 seed files，输出一份 `ContextPack`（候选文件 + 阅读顺序 + 风险提示）。**只读不改**，不接管上游 Agent 的规划。

### 主流水线（5 个阶段）

```
ReconRequest
  → Phase 1:  scanner — 扫描仓库所有文件
  → Phase 1b: import_graph — 构建静态依赖图
  → Phase 2:  neighbor_expander — 从 seed 扩展候选（5 种策略）
  → Phase 3:  file_card_builder — 构建 FileCard（元数据 + 启发式评分）
  → Phase 4:  llm/worker_pool — 可选 LLM rerank（OpenAI-compatible）
  → Phase 5:  ranker + pack/builder — 排序 + 组装 ContextPack
```

流水线入口：`internal/runner/runner.go`

### 核心数据结构（`internal/schema/`）

- **ReconRequest**：输入，包含 `task`、`seed_files`、`profile`、`budget` 等
- **FileCard**：中间态，每个候选文件的元数据、符号、邻居、各维度得分
- **ContextPack**：输出，`main_chain`（≥0.5）/ `companion`（0.3–0.5）/ `uncertain`（0.1–0.3）/ `reading_order` / `risk_hints`

### 候选扩展策略（`internal/heuristics/neighbor_expander.go`）

5 种策略：同目录、同模块、前缀匹配、测试配对、import 图。import 图支持 Go/Python/JS/TS/C/C++/Ruby。

### 排序算法（`internal/ranking/ranker.go`）

无 LLM：`score = DiscoveryScore×0.35 + ModuleWeight×0.15 + HeuristicScore×0.20 + ProfileScore×0.10`

有 LLM：`score = structural×0.35 + llm_score×0.65`

DiscoveryScore 权重：seed(1.0) > import图(0.7) > 测试配对(0.5) > symbol命中(0.3) > 同目录/前缀(0.2) > 同模块(0.1)

### 配置系统（`internal/config/`）

优先级从低到高：内置默认值 → `~/.config/reposcout.json` → 目标仓库 `.reposcout.json` → `-c config.json` → 环境变量（`REPOSCOUT_PROVIDER_*` / `REPOSCOUT_RUNTIME_*`）

### LLM 集成（`internal/llm/`）

走 OpenAI-compatible `v1/chat/completions`。System prompt 内置于 `internal/llm/prompts/classify_system.txt`，可通过配置的 `system_prompt_path` 在运行时覆盖，无需重新编译。`max_input_tokens` 控制单次 rerank 的总输入预算（含 system prompt + 元信息 + 代码片段）。

### 评估（`internal/eval/`）

Golden dataset 在 `examples/goldens/`，每个样本包含请求和期望输出。用 `reposcout eval` 对照评估召回率和排序质量。
