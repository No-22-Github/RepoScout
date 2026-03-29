# RepoScout 项目说明与实现思路

更新时间：2026-03-29

## 这个项目想解决什么

RepoScout 面向的是已经会写代码、也会自己做规划的通用编程 Agent。

它不去替代上游 Agent，而是专门处理一个更窄、但很常见的问题：当任务落到一个不熟悉的大仓库里时，怎么先把“应该读哪些文件”这件事做得更稳一点。

很多任务一开始都能找到主文件，但很容易漏掉这些东西：

- 配套测试
- 默认配置
- 注册点
- 资源文件
- 同模块下的隐式依赖

RepoScout 的目标，就是把这批候选先收出来，再整理成一份更适合继续阅读的 `ContextPack`。

## 当前产品边界

RepoScout 当前是一个 analysis-first 的 repo recon 工具。

它负责：

- 根据 `seed_files` 做静态扩展搜索
- 构建候选文件的 `FileCard`
- 可选用 LLM 分析候选文件并 rerank
- 输出结构化的 `ContextPack`

它不负责：

- 接管上游 Agent 的规划
- 自己做多轮 LLM 驱动搜索
- 决定用户整体任务该怎么拆

一句话：

**静态层负责找候选，LLM 层负责分析候选，上游 Agent 负责继续规划和执行。**

## 为什么不做 LLM 驱动搜索

这个方向现在已经明确降级。

原因很简单：如果上游本身就是 Codex、Claude Code、Cursor 这类编程模型，那它通常已经能：

- 提供 seed files
- 根据当前结果继续决定往哪里看
- 在必要时补充新的候选文件

这时 RepoScout 再加一层 `should_expand` 式搜索控制，收益不一定大，反而容易带来：

- 责任边界重叠
- 评测困难
- 结果归因模糊

所以当前路线很明确：

- 保留并增强静态扩展搜索
- 保留并增强 LLM 候选分析
- 不再把 LLM 驱动搜索当成核心卖点

## 当前主链路

```text
ReconRequest
  ->
Static Candidate Expansion
  ->
FileCard
  ->
Optional LLM Rerank
  ->
ContextPack
```

## 当前已经实现的部分

- CLI 主链路：
  - `reposcout run`
  - `reposcout eval`
- 静态候选扩展：
  - 同目录
  - 同模块
  - 文件名前缀匹配
  - companion sibling 匹配
- 基础启发式规则与 profile 支持
- 轻量符号抽取
- `ContextPack` 的 JSON / Markdown 输出
- OpenAI-compatible provider 接口
- LLM rerank：
  - 当前接入 `classify_file_role`
- 基于 token 预算的上下文构建：
  - 文件概要
  - imports / include 提示
  - 相关声明
  - 相关代码片段

## 当前还没做完，但值得继续做的部分

- import / include / registration 驱动的更强静态扩展
- `judge_relevance` 主流程接入
- `is_implicit_dependency` 主流程接入
- 在真实大仓库上验证纯静态和 LLM rerank 的收益差
- MCP 服务入口

## 输入输出怎么理解

### ReconRequest

这是上游传给 RepoScout 的任务输入，重点字段通常是：

- `task`
- `repo_root`
- `seed_files`
- `focus_symbols`
- `focus_checks`
- `budget`

示例：

```json
{
  "task": "Add a new config option and wire it into the browser settings UI",
  "repo_root": "/path/to/repo",
  "profile": "browser_settings",
  "seed_files": [
    "src/browser/settings/foo_page.tsx",
    "src/browser/settings/foo_handler.cc"
  ],
  "focus_symbols": [
    "foo",
    "enable_foo"
  ],
  "focus_checks": [
    "tests",
    "default_config",
    "resources_or_strings",
    "feature_flag"
  ],
  "budget": {
    "max_seed_neighbors": 40,
    "expand_depth": 2,
    "max_output_files": 20,
    "max_llm_jobs": 64
  }
}
```

### FileCard

`FileCard` 是候选文件的中间表示，主要用于排序和构建模型输入。重点信息包括：

- `path`
- `lang`
- `module`
- `symbols`
- `neighbors`
- `discovered_by`
- `heuristic_tags`
- `scores`

### ContextPack

这是 RepoScout 交给上游 Agent 的最终产物，主要字段包括：

- `main_chain`
- `companion_files`
- `uncertain_nodes`
- `reading_order`
- `risk_hints`
- `summary_markdown`

## LLM 在这里到底起什么作用

当前 LLM 的职责是分析候选文件，而不是控制搜索流程。

它更像一个“候选裁判”：

- 判断文件更像主链路还是配套文件
- 参与 rerank
- 帮助把更值得看的文件往前提

当前推荐继续强化的分析任务：

1. `classify_file_role`
2. `judge_relevance`
3. `is_implicit_dependency`

已经不再作为核心方向推进的任务：

1. `should_expand`

## 上下文构建原则

RepoScout 不追求把整份源码粗暴塞进模型。

当前策略是：

1. 先保留任务和文件元数据
2. 再补文件概要，比如 `package`、`imports`、声明摘要
3. 最后把剩余 token 预算尽量填给相关代码片段

对应配置主要在 `runtime`：

- `max_concurrency`
- `request_timeout_sec`
- `max_input_tokens`
- `max_candidates`
- `max_output_files`
- `enable_model_rerank`

`provider` 部分则负责对接具体模型后端：

- `base_url`
- `api_key`
- `model`
- `api_style`

## 现在最值得做的事

如果继续往下做，优先级建议还是这几个：

1. 继续增强静态扩展搜索
2. 继续增强 LLM 上下文构建质量
3. 在更大的真实仓库上做纯静态 vs LLM rerank 对照
4. 再决定是否继续扩展更多分析型任务
5. 最后补 MCP

## 一句话总结

RepoScout 不是一个替上游 Agent 思考的系统，而是一个先帮它把仓库读图整理好的工具。
