<p align="center">
  <img src="./docs/title.webp" alt="RepoScout title banner" width="960" />
</p>

<h1 align="center">RepoScout</h1>

<p align="center">Repository reconnaissance for coding agents.</p>

<p align="center">
  给编程 Agent 用的仓库侦察工具。给定少量 seed files，找出可能相关的候选文件，
  整理成一份适合继续阅读和执行的 <code>ContextPack</code>。
</p>

<p align="center">
  它不替你改代码，也不接管上游 Agent 的规划，只负责找候选、理顺上下文。
</p>

## 工作流程

```
ReconRequest
  → 静态候选扩展（同目录 / 同模块 / 前缀匹配 / 测试配对 / import 图）
  → FileCard 构建（语言、模块、符号、发现方式评分、启发式评分）
  → 可选 LLM Rerank（OpenAI-compatible）
  → 两阶段排序（结构分 + LLM 混合）
  → ContextPack 输出（main_chain / companion / reading_order / risk_hints）
```

静态层负责召回，LLM 层负责判断和重排，上游 Agent 负责继续读、继续改。

## 快速上手

```bash
# 从 JSON 请求文件运行
reposcout run request.json

# 直接传参
reposcout run --task "Add auth endpoint" --repo ./myproject --seed auth/handler.go

# 输出 Markdown
reposcout run request.json --format markdown

# 开启 LLM rerank
reposcout run request.json --rerank -c config.json
```

请求文件格式（`request.json`）：

```json
{
  "task": "Add settings sync support",
  "repo_root": "/path/to/repo",
  "seed_files": ["browser/settings/settings_page.cc"],
  "profile": "browser_settings",
  "budget": {
    "expand_depth": 1,
    "max_output_files": 20
  }
}
```

## 候选扩展策略

从 seed files 出发，通过以下五种方式扩展候选集：

| 策略 | 说明 |
|------|------|
| 同目录 | seed 文件所在目录的全部文件 |
| 同模块 | 路径结构相同的模块内文件 |
| 前缀匹配 | 文件名前缀相似的文件（如 `settings_page` → `settings_handler`）|
| 测试配对 | 实现与测试文件互相关联（`_test`、`_spec`、`__tests__/` 等）|
| import 图 | 通过 import/include 关系连接的文件（支持 Go、Python、JS/TS、C/C++、Ruby）|

## 配置

配置按以下优先级叠加（低 → 高）：

1. 内置默认值
2. 用户配置：`~/.config/reposcout.json`
3. 仓库配置：目标仓库根目录下的 `.reposcout.json`
4. 显式指定：`-c config.json`
5. 环境变量：`REPOSCOUT_PROVIDER_*` / `REPOSCOUT_RUNTIME_*`

通常把 API key 和模型放用户配置，把仓库相关的参数（如 `max_output_files`）放仓库配置。

配置文件示例：

```json
{
  "provider": {
    "base_url": "http://localhost:8080/v1",
    "api_key": "sk-xxx",
    "model": "your-model",
    "system_prompt_path": "/path/to/my_system_prompt.txt"
  },
  "runtime": {
    "enable_model_rerank": true,
    "max_input_tokens": 8192
  }
}
```

模型层可选，走 OpenAI-compatible `v1/chat/completions`。不配置时只走静态分析。

### system_prompt_path

LLM rerank 时发送的 system prompt 默认内置在代码里（精简版，适合大多数模型）。如果需要针对特定模型调整，有两种方式：

- **编辑源文件**：修改 `internal/llm/prompts/classify_system.txt`，重新编译即可。
- **运行时指定**：在配置文件里设置 `system_prompt_path` 指向外部文本文件，无需重新编译。路径支持相对路径（相对于配置文件所在目录）和绝对路径。也可通过环境变量 `REPOSCOUT_PROVIDER_SYSTEM_PROMPT_PATH` 设置。

### max_input_tokens

`max_input_tokens` 控制单次 LLM rerank 的总输入预算，不只是代码片段预算。它同时覆盖 system prompt、文件元信息和 RepoScout 自动拼接的 `## Context` 内容。

RepoScout 会先预留前面的固定部分，再把剩余预算留给上下文摘要和代码片段，所以这个值本质上决定了给模型喂多少上下文，RepoScout 会根据设置的数量，结合文件内容智能填充，尽可能填充到设置上限。

调参时建议先落在模型上下文窗口的“甜点区”，不要理解为模型的实际上下文上限。

## 输出结构

```json
{
  "main_chain": ["..."],        // 核心相关文件，建议优先读
  "companion_files": ["..."],   // 配套文件（测试、配置、资源等）
  "uncertain": ["..."],         // 相关性不确定
  "reading_order": ["..."],     // 建议阅读顺序
  "risk_hints": ["..."]         // 潜在遗漏或风险提示
}
```

## 排序算法

文件最终得分分两阶段计算：

**无 LLM 时（纯结构分）：**
```
score = DiscoveryScore×0.35 + ModuleWeight×0.15 + HeuristicScore×0.20 + ProfileScore×0.10
```

**有 LLM 时（混合）：**
```
score = structural×0.35 + llm_score×0.65
```

`DiscoveryScore` 按发现方式区分强弱：import 图（0.7）> 测试配对（0.5）> symbol 命中（0.3）> 同目录/前缀（0.2）> 同模块（0.1）> seed 文件（1.0）。

分类阈值：`main_chain ≥ 0.5`，`companion 0.3–0.5`，`uncertain 0.1–0.3`。



```bash
reposcout eval examples/goldens
```

对照 golden dataset 评估召回率和排序质量。

## 当前状态

主链路已跑通，支持接入本地 OpenAI-compatible 后端（如 RWKV）。近期主要改进：

- 排序算法重设计：引入 `DiscoveryScore` 区分 import 图与随机同目录文件的权重差异，LLM 有结果时以 0.65 权重主导最终排序
- LLM 上下文质量提升：prompt 中加入结构分摘要、人类可读的发现方式描述、symbol 正则缓存、更健壮的 JSON 解析
- system prompt 支持外部文件，便于针对不同小模型调整

MCP server 支持在计划中，尚未实现。

## 文档

- 设计思路与实现细节：[docs/RepoScout_MVP.md](docs/RepoScout_MVP.md)
