# RepoScout

给编程 Agent 用的仓库侦察工具。

在大仓库里做任务，通常一开始只能抓到主文件，很容易漏掉测试、配置、注册项、资源文件，或者散落在其他目录的配套实现。RepoScout 解决的就是这一步：给定少量 seed files，把可能相关的候选文件找出来，整理成一份适合继续读的 `ContextPack`。

它不替你改代码，也不接管上游 Agent 的规划——只负责"找候选、理顺上下文"。

## 工作流程

```
ReconRequest
  → 静态候选扩展（同目录 / 同模块 / 前缀匹配 / 测试配对 / import 图）
  → FileCard 构建（语言、模块、符号、启发式评分）
  → 可选 LLM Rerank（OpenAI-compatible）
  → 多因子排序
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
    "model": "your-model"
  },
  "runtime": {
    "enable_model_rerank": true,
    "max_input_tokens": 8192
  }
}
```

模型层可选，走 OpenAI-compatible `v1/chat/completions`。不配置时只走静态分析。

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

## 评估

```bash
reposcout eval examples/goldens
```

对照 golden dataset 评估召回率和排序质量。

## 当前状态

主链路已跑通，支持接入本地 OpenAI-compatible 后端（如 RWKV）。接下来主要继续增强：静态扩展搜索质量，以及候选文件的上下文构建质量。

MCP server 支持在计划中，尚未实现。

## 文档

- 设计思路与实现细节：[docs/RepoScout_MVP.md](docs/RepoScout_MVP.md)
