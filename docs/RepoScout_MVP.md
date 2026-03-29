# RepoScout MVP

更新时间：2026-03-29

## 1. 当前定位

`RepoScout` 不是一个二次规划型 Agent，也不是一个 LLM 驱动的搜索控制器。

当前版本的定位已经收口为：

- 输入：任务描述、仓库路径、少量 seed files
- 过程：
  - 先用静态分析做候选文件扩展和收缩
  - 再可选地用 LLM 对候选文件做轻量分析和重排
- 输出：给上游编程 Agent 直接消费的 `ContextPack`

一句话：

**RepoScout 是一个 analysis-first 的 repo recon 工具，不是一个 LLM-driven 的搜索编排层。**

---

## 2. 为什么要这样收口

如果上游本身就是 Codex、Claude Code、Cursor 这类编程大模型，那么它通常已经具备：

- 给出 seed files 的能力
- 基于上下文自己决定下一步看的文件
- 在必要时继续发起额外搜索或补充候选文件

因此 RepoScout 不应该再引入一层模型驱动搜索，否则会出现：

- 与上游 Agent 的规划能力重叠
- 责任边界不清
- 评测复杂度上升
- 用户不知道“是谁决定扩展错了”

所以当前方向明确调整为：

- 保留并增强静态文件扩展搜索
- 保留并增强 LLM 对候选文件的分析能力
- 不再把 LLM 驱动搜索当作核心卖点

---

## 3. 核心能力边界

### 3.1 保留并增强的能力

- 基于 seed 的静态扩展搜索
- 候选文件的静态收缩与排序
- 基于候选文件的 LLM 轻量分析
- 结构化 `ContextPack` 输出

### 3.2 明确不作为核心方向的能力

- `LLM` 决定是否继续扩展文件搜索
- `LLM` 接管上游 Agent 的规划职责
- RepoScout 自己做多轮驱动式搜索

### 3.3 仍然需要的静态扩展搜索

静态扩展不是要削弱，而是要继续增强。

优先增强的方向：

- seed 文件邻域扩展
- 同目录 / 同模块扩展
- 文件名模式匹配
- companion file 匹配
  - 实现 / 测试
  - 源文件 / 头文件
  - mock / fixture / spec
- import / include / registration 关系

原则：

**静态层负责“找候选”，LLM 层负责“分析候选”。**

---

## 4. 当前已实现能力

当前仓库已经具备：

- 静态主链路：
  - `ReconRequest -> Candidate Expansion -> FileCard -> Ranker -> ContextPack`
- CLI：
  - `reposcout run`
  - `reposcout eval`
- 静态候选扩展：
  - 同目录
  - 同模块
  - 文件名前缀匹配
  - companion sibling 匹配
- 基础启发式规则与 profile 规则
- 轻量符号抽取
- `ContextPack` 的 JSON / Markdown 输出
- OpenAI-compatible provider 接口
- LLM rerank：
  - 当前只接 `classify_file_role`
- LLM 上下文预算控制：
  - `runtime.max_input_tokens`
  - 尽量用文件概要、imports 和相关代码片段贴近预算上限

---

## 5. 当前明确未做的能力

当前未实现：

- MCP 服务入口
- `judge_relevance` 主流程接入
- `is_implicit_dependency` 主流程接入
- import / include / registration 驱动的更强静态扩展
- 面向真实 provider 的系统级联调文档

当前不再作为主方向推进：

- `should_expand`
- 模型参与候选扩展
- LLM 驱动搜索

---

## 6. MVP 成功标准

当前版本不追求“完全自动找全文件”，只追求下面几件事成立：

1. 在真实仓库中，静态扩展搜索能稳定给出一批高质量候选。
2. 在这批候选上，LLM rerank 能比纯静态排序更稳定地提升 `main_chain` 和 `companion_files` 质量。
3. 输出结构足够稳定，可被上游 Agent 直接消费。

建议重点看这几类指标：

- Top-10 / Top-20 关键文件召回
- `main_chain` 质量
- `companion_files` 噪音率
- 纯静态 vs LLM rerank 的对照收益
- 单任务耗时是否可接受

---

## 7. 输入输出约定

### 7.1 ReconRequest

保留最小必要字段：

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

### 7.2 FileCard

`FileCard` 是候选文件的中间表示，只保留真正影响排序和上下文构建的信息：

- `path`
- `lang`
- `module`
- `symbols`
- `neighbors`
- `discovered_by`
- `heuristic_tags`
- `scores`

### 7.3 ContextPack

RepoScout 对上游交付的最终结构：

- `main_chain`
- `companion_files`
- `uncertain_nodes`
- `reading_order`
- `risk_hints`
- `summary_markdown`

---

## 8. LLM 在 MVP 中负责什么

当前 LLM 的职责是：

- 对候选文件做轻量分析
- 参与重排
- 帮助把更有价值的文件提到 `main_chain` / `companion_files`

当前推荐的 LLM 分析任务：

1. `classify_file_role`
2. `judge_relevance`
3. `is_implicit_dependency`

当前不再作为主方向的任务：

1. `should_expand`

输入原则：

- 不直接把整仓塞给模型
- 以 `TaskCard` 为基本输入单元
- 尽量使用压缩后的文件概要和相关代码片段

---

## 9. 运行配置

统一运行配置保留为两组：

### 9.1 provider

- `provider.base_url`
- `provider.api_key`
- `provider.model`
- `provider.api_style`

### 9.2 runtime

- `runtime.max_concurrency`
- `runtime.request_timeout_sec`
- `runtime.max_input_tokens`
- `runtime.max_candidates`
- `runtime.max_output_files`
- `runtime.enable_model_rerank`

说明：

- `runtime.max_input_tokens` 用于限制单次 LLM 输入预算
- 构建 prompt 时应优先保留任务和文件元数据
- 剩余预算尽量填给 imports、声明摘要和相关代码片段

示例：

```json
{
  "provider": {
    "api_style": "openai",
    "base_url": "http://127.0.0.1:8080/openai/v1",
    "api_key": "",
    "model": "rwkv7-g1e-7.2b-20260301-ctx8192"
  },
  "runtime": {
    "max_concurrency": 16,
    "request_timeout_sec": 60,
    "max_input_tokens": 4096,
    "max_candidates": 200,
    "max_output_files": 20,
    "enable_model_rerank": true
  }
}
```

---

## 10. 当前推荐推进顺序

后续开发优先级应收口为：

1. 继续增强静态扩展搜索
2. 继续增强 LLM 上下文构建
3. 验证 `classify_file_role` rerank 在真实仓库上的收益
4. 视收益决定是否接 `judge_relevance`
5. 再考虑 `is_implicit_dependency`
6. 最后补 MCP

不建议的顺序：

- 重新把产品方向拉回 LLM 驱动搜索
- 在没有评测收益前继续堆更多模型任务
- 在没有把静态扩展做强前，把问题都推给 LLM

---

## 11. 一句话结论

RepoScout 当前的正确方向是：

**增强静态文件扩展搜索，增强候选文件的 LLM 分析质量，把结果稳定整理成给上游编程 Agent 使用的 `ContextPack`。**
