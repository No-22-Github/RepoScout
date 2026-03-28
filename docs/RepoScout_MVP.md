# RepoScout MVP 文档

## 1. 文档目标

本文档用于定义 `RepoScout` 的首个可落地版本（MVP）。

这个版本的目标不是做一个全自动改代码 Agent，也不是做一个通用的代码搜索平台，而是做一个：

**给通用代码 Agent 提供“仓库侦察 + 上下文小抄”的前置工具。**

同时，这个 MVP 会明确沿着 `RWKV` 的方向优化，突出以下几点：

- 本地单机部署
- 短上下文、小任务、高并发
- 用一批 LLM Worker 处理大量轻量判断任务
- 输出可直接喂给上游 Agent 的 `ContextPack`

这里的 `LLM Worker` 是产品层抽象概念：

- 接口上可以连接任意 OpenAI 兼容后端
- 优化方向上优先为 RWKV 设计
- 在单机部署 + 高并发条件下，RWKV 是首选后端

---

## 2. MVP 的一句话定义

输入一个任务描述和少量 seed 文件，`RepoScout` 先用静态分析收缩候选范围，再用批量 LLM Worker 对候选文件做角色判断和扩展判断，最后输出一个带阅读顺序、主链路、配套文件和风险提示的 `ContextPack`。

---

## 3. 为什么 MVP 要优先按 RWKV 方向设计

如果只是做“仓库侦察工具”，完全可以用更大的在线模型一次性扫一遍候选文件。但这条路有两个问题：

1. 成本高，且并发性差。
2. 难以体现 RWKV 的独特优势。

RWKV 的适配点不在“大上下文通读”，而在：

- 对单文件或小批量文件做短上下文判断
- 高并发运行大量分类任务
- 在本地机器上稳定复用
- 作为上游大模型的廉价前置筛选器

所以 `RepoScout` 的核心不是“让 RWKV 看完整个仓库”，而是：

**把大问题拆成很多个短任务，让 RWKV 负责批量判断。**

这是最符合 RWKV 形态的产品路线。

---

## 4. MVP 成功标准

MVP 不追求“全仓通用、零漏报”，只追求下面三件事成立：

1. 对目标仓库族，能比“只靠通用 Agent 自己搜仓”更稳定地找出关键配套文件。
2. 能证明 RWKV 在大量轻判断任务上有明显吞吐优势。
3. 输出结果足够结构化，能被 Codex、Claude Code、Cursor 一类 Agent 直接消费。

建议用下面的成功指标：

- Top-10 文件召回率：`>= 70%`
- Top-20 文件召回率：`>= 85%`
- 隐式依赖命中率：对测试、配置注册、资源字符串等重点项有明显增益
- 单任务侦察耗时：本地单机可接受
- 多任务并发吞吐：明显优于串行大模型调用

---

## 5. MVP 不做什么

为了避免第一版过重，以下内容明确不进入 MVP：

- 不直接改代码
- 不负责自动生成 patch
- 不做跨所有语言和所有仓库的通用支持
- 不做复杂 UI，先以 CLI / JSON 输出为主
- 不做完整代码索引平台
- 不让 RWKV 负责长链推理和多轮规划

一句话：**MVP 只做“侦察和整理”，不做“执行和修改”。**

---

## 6. 目标用户与使用场景

### 6.1 目标用户

- 使用 Codex / Claude Code / Cursor 的开发者
- 需要在大仓里做中小改动的工程师
- 希望把 RWKV 用在工程生产链路里的参赛者或开发者

### 6.2 典型场景

用户要完成一个任务：

> “新增一个配置项，并接到设置页面里”

上游 Agent 已经大致知道入口文件，但不知道还有哪些文件大概率要一起看：

- 配置注册
- 默认值
- UI 文案/资源
- 测试
- feature flag
- 文档或 build 注册项

这时把任务和 seed 文件交给 `RepoScout`，它返回：

- 主链路文件
- 高概率配套文件
- 不确定但值得复查的文件
- 建议阅读顺序
- 风险提示

然后上游 Agent 再继续精读和改代码。

---

## 7. MVP 范围收缩

MVP 必须收缩，不然很容易做成泛化能力不稳定的大工程。

### 7.1 首个支持的 repo family

建议 MVP 只针对一类仓库先做深：

**优先建议：Chromium / Browser Settings / Prefs 这一类模式明确的 C++ + TS 混合仓库。**

原因：

- 你的例子已经天然贴近这个场景
- “设置页 -> handler -> prefs 注册 -> strings -> test” 的链路很典型
- 隐式依赖模式清晰，适合构造评测集

如果不做 Chromium，也至少要选一个“结构稳定、套路明显”的大仓，不建议一开始就做“任意 GitHub 仓库通用”。

### 7.2 首批 focus checks

MVP 只支持少数高价值检查项：

- `tests`
- `default_config`
- `build_registration`
- `resources_or_strings`
- `feature_flag`

`docs` 可以先作为弱支持项，不作为主卖点。

---

## 8. MVP 核心思路

系统采用“两段式”：

### 第一段：静态分析收缩范围

目标是把候选文件从“大仓全量”收缩到“几十到几百个高价值候选”。

手段包括：

- seed 文件邻域扩展
- 同目录 / 同模块扩展
- 符号命中
- 文件名模式匹配
- import / include / call / registration 关系
- 规则命中

这一段尽量不用模型，优先靠确定性逻辑完成。

### 第二段：LLM 并发轻判断

这一段默认优先为 RWKV 优化，但工程接口不绑定 RWKV 专有实现。

小模型后端不负责全局理解，而是负责对候选文件做短任务判断，例如：

- 这个文件是不是主链路角色之一
- 这个文件是不是隐式依赖
- 这个文件值不值得继续扩展
- 这个文件更像测试、配置桥接、资源还是控制层

最后把静态证据和小模型判断合并，拼装 `ContextPack`。

---

## 9. MVP 系统架构

```text
User Task / Agent
   ->
ReconRequest Builder
   ->
Static Scout
   ->
Candidate File Set
   ->
LLM Worker Pool
   ->
Ranker + ContextPack Builder
   ->
ContextPack(JSON + Markdown Summary)
```

### 9.1 模块划分

#### A. ReconRequest Builder

负责整理上游输入，生成统一请求。

#### B. Static Scout

负责静态收缩候选集和生成 `FileCard`。

#### C. LLM Worker Pool

负责并发执行小任务分类。

#### D. Ranker / Merger

把规则分、图邻接分、LLM 分数合并。

#### E. ContextPack Builder

输出结构化结果和给人/Agent 看的简明总结。

---

## 10. MVP 输入输出定义

### 10.1 输入：ReconRequest

MVP 版保留最少字段：

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

### 10.2 中间结构：FileCard

MVP 版的 `FileCard` 不要做太重，先保留对排序真正有用的信息：

```json
{
  "path": "src/browser/settings/foo_handler.cc",
  "lang": "cpp",
  "module": "browser/settings",
  "symbols": ["HandleFooToggle", "RegisterFooPrefs"],
  "neighbors": [
    {
      "path": "src/browser/settings/foo_page.tsx",
      "edge": "same_module"
    }
  ],
  "discovered_by": [
    "seed",
    "symbol_hit",
    "same_dir_rule"
  ],
  "heuristic_tags": [
    "possible_controller",
    "possible_pref_bridge"
  ]
}
```

### 10.3 输出：ContextPack

```json
{
  "task": "Add a new config option and wire it into the browser settings UI",
  "repo_family": "browser_settings",
  "main_chain": [],
  "companion_files": [],
  "uncertain_nodes": [],
  "reading_order": [],
  "risk_hints": [],
  "summary_markdown": "..."
}
```

MVP 阶段最重要的是保证这几个字段稳定：

- `main_chain`
- `companion_files`
- `reading_order`
- `risk_hints`

---

## 11. 小模型后端在 MVP 中负责什么

### 11.0 模型接口约束

虽然项目方向明确按 RWKV 优化，但 MVP 在工程接口层面不要和某个具体服务端强耦合。

当前约束应定义为：

- 小模型后端统一走 OpenAI 兼容接口
- 默认对接形态为 `v1/chat/completions`
- RepoScout 只依赖“请求/响应格式兼容”，不依赖某个特定服务端实现

这意味着在工程上：

- 只要兼容 OpenAI 风格的聊天补全接口，理论上都可以先接入
- 在 RWKV 服务端尚未完全定型前，可以先用其他兼容后端做联调和管线验证
- 等 RWKV 服务端完善后，再无缝替换到底层 provider

也就是说：

**MVP 的小模型后端是可替换的，但项目的优化目标仍然是 RWKV。**

### 11.0.1 术语约定

后续文档和实现统一采用下面的术语：

- `LLM Worker`：执行单个轻量判断任务的抽象执行单元
- `LLM Worker Pool`：并发调度一批轻量判断任务的执行层
- `Provider`：具体模型后端实现
- `RWKV-first provider`：默认优先优化的 provider 方向
- `generic provider mode`：不依赖特定模型品牌的通用接入模式

这意味着：

- 产品层描述优先说 `LLM Worker`
- 工程实现通过 `Provider` 抽象接具体后端
- 路线选择上仍然优先围绕 RWKV 做性能和部署优化

### 11.1 适合交给小模型后端的任务

MVP 中，小模型后端只做短文本分类和轻推断任务。

建议保留 4 类任务：

1. `classify_file_role`
2. `judge_relevance`
3. `should_expand`
4. `is_implicit_dependency`

### 11.2 每个任务的输入形式

小模型后端的输入不要塞整文件，建议统一成“任务卡片”：

- 任务描述
- 当前文件路径
- 文件摘要片段
- 命中的符号
- 少量邻接文件信息
- 关注检查项

也就是说，模型看到的是“压缩过的文件卡”，不是原始大文件全文。

### 11.3 为什么这样设计

这样做有三个好处：

- 适应 8k 上下文
- 方便批量并发
- 让任务分布稳定，便于调 prompt 和做评测

### 11.4 标准输出格式

统一输出为：

```json
{
  "task_type": "is_implicit_dependency",
  "label": "resources_or_strings",
  "confidence": 0.79,
  "reason": "Nearby settings UI file usually requires matching visible strings"
}
```

MVP 中 `reason` 只要求简短可读，不要求长解释。

---

## 12. 静态分析优先级

MVP 是否靠谱，首先取决于静态层做得够不够扎实。

建议按下面顺序实现：

### 12.1 必做

- 文件遍历与语言识别
- seed 文件同目录/同模块扩展
- import/include/调用的浅层邻接
- 文件名规则匹配
- `test` / `strings` / `prefs` / `flag` / `config` 等关键词规则

### 12.2 可选

- 简化符号抽取
- 轻量 cross-reference
- build 文件关联

### 12.3 暂缓

- 完整 LSP 级别索引
- 全语言 AST 深度分析
- 精确过程间数据流

原则很简单：

**MVP 静态层只做高性价比收缩，不做重型程序分析。**

---

## 13. 排序与合并策略

最终排序建议采用简单可解释的线性打分，而不是一开始就上复杂学习排序。

示意：

```text
final_score =
  heuristic_score * 0.45 +
  graph_score * 0.25 +
  llm_score * 0.30
```

其中：

- `heuristic_score`：规则命中分
- `graph_score`：与 seed 和主链路的邻近程度
- `llm_score`：小模型分类置信分

这样设计的原因：

- 第一版更稳定
- 出错时更容易解释
- 方便后续替换模型或增强规则

---

## 14. MVP 交付形态

MVP 建议提供两种接入形态：

### 14.1 CLI 工具

适合本地命令行调用、脚本集成和比赛演示。

### 14.2 MCP 服务

适合接入 Codex、Claude Code、Cursor 一类编程 Agent 客户端，让它作为外部仓库侦察工具被调用。

在这两种接入形态之上，再提供两类输出：

### 14.3 JSON 输出

给 Agent 或其他程序消费。

### 14.4 Markdown 摘要

给人直接读，建议包含：

- 任务一句话摘要
- 建议先读的 3 到 5 个文件
- 可能漏改的配套项
- 不确定点

CLI 示例：

```bash
reposcout run recon_request.json --format json
reposcout run recon_request.json --format markdown
reposcout mcp
```

MCP 形态下，建议暴露一个主工具，例如：

```text
reposcout_recon(recon_request, runtime_config?)
```

其中 `runtime_config` 用于覆盖默认 provider、模型名、并发数等运行参数。

---

## 15. 运行配置

由于项目会作为 CLI 工具或 MCP 服务接入编程 Agent 客户端，MVP 必须支持显式运行配置。

这些配置不应散落在代码里，而应统一收口到配置结构中。

### 15.1 必备配置项

- `provider.base_url`
- `provider.api_key`
- `provider.model`
- `provider.api_style`
- `runtime.max_concurrency`
- `runtime.request_timeout_sec`
- `runtime.max_candidates`
- `runtime.max_output_files`
- `runtime.enable_model_rerank`

### 15.2 推荐配置结构

```json
{
  "provider": {
    "api_style": "openai_compatible",
    "base_url": "http://127.0.0.1:8000/v1",
    "api_key": "dummy-or-real-key",
    "model": "reposcout-worker"
  },
  "runtime": {
    "max_concurrency": 64,
    "request_timeout_sec": 30,
    "max_candidates": 200,
    "max_output_files": 20,
    "enable_model_rerank": true
  }
}
```

### 15.3 配置原则

- CLI 模式下，配置可以来自配置文件、环境变量或命令行参数
- MCP 模式下，配置应以服务启动参数为主，必要时允许单次调用覆盖
- 上层业务逻辑不直接感知 `api_key`、`base_url` 等 provider 细节
- 即使换成其他兼容后端，配置结构也尽量不变

### 15.4 为什么要提前定义配置

如果不在 MVP 阶段先把运行配置结构固定，后面接 CLI、MCP、mock provider、RWKV provider 时很容易出现：

- 配置入口重复
- 并发控制分散
- provider 切换成本高
- Agent 客户端集成方式不一致

所以配置层在 MVP 中不是附属项，而是正式功能。

---

## 16. MVP 的实现阶段

建议分两阶段。

### Phase 1：无模型版本先跑通

目标：

- 跑通 `ReconRequest -> Static Scout -> ContextPack`
- 先用纯规则输出一个可用版本

验收标准：

- 能在目标仓库上跑出基本可读的 `main_chain` 和 `companion_files`
- 结果虽然粗糙，但比手工乱搜更集中

### Phase 2：接入 LLM Worker Pool

目标：

- 对候选文件批量跑小模型任务
- 用小模型结果重排和补充 `companion_files`

验收标准：

- 对重点检查项有可测的召回提升
- 并发吞吐能展示 RWKV 的优势

这两阶段不能倒过来。先把静态骨架做稳，再接 RWKV。

---

## 17. MVP 评测方案

这是整个项目最关键的部分。

### 17.1 评测样本来源

从目标仓库中收集真实变更任务，优先选：

- 设置项新增
- 配置项改动
- feature flag 接线
- UI 文案同步
- 测试补齐

每个任务需要人工整理：

- 用户任务描述
- seed 文件
- 最终真实涉及文件集合

这会形成 `gold set`。

### 17.2 评测指标

- `Recall@10`
- `Recall@20`
- 隐式依赖召回率
- 主链路命中率
- 平均候选文件数
- 平均耗时
- 并发吞吐

### 17.3 对照组

至少要做三个版本对比：

1. 纯静态规则
2. 静态规则 + RWKV
3. 通用 Agent 自己搜仓的 baseline

如果第二组不能稳定优于第一组，说明 RWKV 接入方式还没有找对。

---

## 18. MVP 的主要风险

### 18.1 风险一：RWKV 带来的收益不明显

可能原因：

- 候选集已经被静态层收得太准
- 输入卡片压缩得不好
- 分类任务定义太模糊

对策：

- 把 RWKV 聚焦到最难的几类判定
- 控制标签集，避免任务发散
- 先做少数高价值任务，不贪多

### 18.2 风险二：召回不稳定

可能原因：

- 仓库模式差异太大
- 规则不够贴近目标 repo family

对策：

- 先只支持一个 repo family
- 先做 profile 化规则，不追求通用

### 18.3 风险三：工程量失控

可能原因：

- 一开始就想做多语言、多仓库、多模型支持

对策：

- 严格限制 MVP 范围
- 所有新增功能都要先回答：是否直接提升 `ContextPack` 的可用性或 RWKV 的展示效果？

---

## 19. 建议的 MVP 技术选型

语言层面建议优先考虑：

- Go：适合作为完整后端实现，方便做 CLI、并发调度、静态扫描和部署

模型接入建议：

- Provider 接口统一按 OpenAI 兼容 `v1/chat/completions` 封装
- 先实现 provider 抽象层，再接具体后端
- 开发期可替换成任意兼容后端进行联调
- 若目标是单机部署 + 高并发，小模型后端优先选择 RWKV

这里需要明确一句：

**从 RepoScout 的产品形态看，小模型后端换哪个都可以；但如果约束条件是“单机部署 + 高并发 + 本地可控成本”，RWKV 仍然是当前最优解。**

核心模块建议：

- `request_schema`
- `repo_scanner`
- `candidate_builder`
- `filecard_builder`
- `llm_worker_pool`
- `ranker`
- `contextpack_builder`
- `evaluator`

目录建议：

```text
reposcout/
  schemas/
  scanner/
  heuristics/
  llm/
  ranking/
  output/
  eval/
```

---

## 20. 推荐的第一版演示路径

为了比赛展示效果，建议准备一个固定 Demo：

1. 给出一个真实任务描述
2. 指定 1 到 2 个 seed 文件
3. 展示静态层筛出的候选集
4. 展示 LLM Worker Pool 并发判断过程
5. 输出 `ContextPack`
6. 对照真实改动文件集合，展示命中情况

这个演示比“长篇解释架构”更有说服力。

---

## 21. 最终结论

`RepoScout` 的 MVP 应该定义为：

**一个面向大仓改动任务的前置侦察器。它先用静态规则缩小范围，再利用 RWKV 的本地高并发优势批量完成轻量判断，最终产出给通用 Agent 使用的 ContextPack。**

这个定义有三个好处：

- 它能体现 RWKV 的真实优势
- 它避免与成熟代码 Agent 正面竞争
- 它有清晰的评测目标和可交付结果

第一版最重要的不是“多智能”，而是：

**稳定、可评测、可演示。**

如果这三点成立，这个项目就有继续做下去的价值。

---

## 22. 文档使用方式

从这一节开始，文档不再只是产品描述，而是实现路线说明。

目标是做到：

- 新开一个 Codex 会话
- 让它先读本文件
- 它就能判断当前应该实现哪个功能点
- 实现完成后能更新状态并继续做下一个功能点

为此，后面的 todo list 会遵守四个规则：

1. 每个功能点都有唯一 ID。
2. 每个功能点都有前置依赖、输入输出、完成标准。
3. 每个功能点尽量能独立提交。
4. 默认按照 ID 顺序执行，除非某个功能点明确写了可以并行。

---

## 23. 实现约束

为了避免实现过程中不断跑偏，MVP 的工程约束先固定如下：

### 23.1 技术栈

- 主语言：Go 1.23+
- 包管理：Go Modules
- 接入形态：CLI + MCP 服务
- 数据交换：JSON 文件优先

### 23.2 第一版目录约束

建议按下面结构实现，不要求一次建全，但新增代码应尽量落在这个结构中：

```text
cmd/reposcout/
  main.go
internal/config/
  config.go
internal/schema/
  recon_request.go
  file_card.go
  context_pack.go
internal/scanner/
  repo_scanner.go
  language_detector.go
internal/heuristics/
  candidate_builder.go
  rules_basic.go
  rules_browser_settings.go
internal/ranking/
  ranker.go
internal/output/
  markdown_renderer.go
internal/llm/
  task_card.go
  worker_pool.go
  adapter.go
internal/mcp/
  server.go
internal/eval/
  evaluator.go
examples/
  recon_request.sample.json
  runtime_config.sample.json
  goldens/
```

### 23.3 第一版命令约束

MVP 至少支持以下命令：

```bash
reposcout run <recon_request.json> --format json
reposcout run <recon_request.json> --format markdown
reposcout eval <dataset_dir>
reposcout mcp
```

### 23.4 编码约束

- 优先写纯函数和小模块，避免一开始堆在单个脚本里
- 所有核心数据结构都必须可序列化成 JSON
- 每个功能点完成后，至少要有一个最小可运行示例
- 没有 RWKV 也必须能跑通静态版流程
- provider 配置和运行时配置必须通过统一配置层读取

---

## 24. 实现顺序总览

按依赖关系，推荐实现顺序如下：

1. 基础骨架
2. 配置层
3. Schema 定义
4. Repo 扫描与语言识别
5. 候选集静态生成
6. ContextPack 组装
7. CLI 跑通无模型链路
8. Markdown 输出
9. 评测框架
10. LLM 任务卡
11. Provider 抽象层
12. LLM Worker Pool
13. LLM 分数融合
14. MCP 服务入口
15. Demo 与样例数据

只要前 1 到 7 步完成，就已经有一个能跑的无模型 MVP。

---

## 25. 功能点 Todo List

下面的 todo list 按执行顺序组织。

状态约定：

- `[TODO]` 未开始
- `[DOING]` 进行中
- `[DONE]` 已完成
- `[BLOCKED]` 被依赖阻塞

### RS-001 [DONE] 初始化项目骨架

目标：

建立最小可运行的 Go 项目骨架和 CLI 入口。

前置依赖：

- 无

输入：

- 仓库根目录

输出：

- `go.mod`
- `cmd/reposcout/main.go`
- `internal/` 基础目录
- 可执行的空命令入口

完成标准：

- 能运行 `go run ./cmd/reposcout --help`
- CLI 至少暴露 `run` 和 `eval` 两个子命令

备注：

这是后续所有功能点的基础，不要在这个阶段引入业务逻辑。

### RS-002 [DONE] 实现统一运行配置结构

目标：

把 CLI 和 MCP 共用的 provider / runtime 配置固定下来。

前置依赖：

- `RS-001`

输入：

- 配置文件
- 环境变量
- 命令行默认值

输出：

- `internal/config/config.go`
- `examples/runtime_config.sample.json`

完成标准：

- 至少支持配置项：
  - `provider.base_url`
  - `provider.api_key`
  - `provider.model`
  - `provider.api_style`
  - `runtime.max_concurrency`
  - `runtime.request_timeout_sec`
  - `runtime.max_candidates`
  - `runtime.max_output_files`
  - `runtime.enable_model_rerank`
- 配置结构可被 CLI 和后续 MCP 共同复用

### RS-003 [DONE] 定义 ReconRequest Schema

目标：

把 `ReconRequest` 固化成代码结构和 JSON 校验入口。

前置依赖：

- `RS-001`

输入：

- 用户提供的 recon request JSON

输出：

- `internal/schema/recon_request.go`
- `examples/recon_request.sample.json`

完成标准：

- 能从 JSON 文件解析出 `ReconRequest`
- 缺少必填字段时报清晰错误
- 至少支持字段：
  - `task`
  - `repo_root`
  - `profile`
  - `seed_files`
  - `focus_symbols`
  - `focus_checks`
  - `budget`

### RS-004 [DONE] 定义 FileCard Schema

目标：

固定静态分析阶段的中间产物结构。

前置依赖：

- `RS-001`

输入：

- 候选文件基础信息

输出：

- `internal/schema/file_card.go`

完成标准：

- `FileCard` 可序列化为 JSON
- 至少包含字段：
  - `path`
  - `lang`
  - `module`
  - `symbols`
  - `neighbors`
  - `discovered_by`
  - `heuristic_tags`
  - `scores`

### RS-005 [DONE] 定义 ContextPack Schema

目标：

固定最终交付给上游 Agent 的结果结构。

前置依赖：

- `RS-001`

输入：

- 主链路文件、配套文件、风险提示、阅读顺序

输出：

- `internal/schema/context_pack.go`

完成标准：

- `ContextPack` 可序列化为 JSON
- 至少包含字段：
  - `task`
  - `repo_family`
  - `main_chain`
  - `companion_files`
  - `uncertain_nodes`
  - `reading_order`
  - `risk_hints`
  - `summary_markdown`

### RS-006 [DONE] 实现 Repo 文件扫描

目标：

从 `repo_root` 构建基础文件列表。

前置依赖：

- `RS-003`

输入：

- `repo_root`

输出：

- 全仓相对路径列表

完成标准：

- 能递归扫描仓库
- 支持忽略常见无关目录，例如 `.git`、`node_modules`、`out`、`dist`
- 返回稳定的相对路径列表

### RS-007 [TODO] 实现语言识别

目标：

按扩展名和少量规则识别文件语言或类型。

前置依赖：

- `RS-006`

输入：

- 文件路径

输出：

- `lang` 标签，例如 `cpp`、`ts`、`tsx`、`gn`、`json`、`text`

完成标准：

- 至少覆盖 MVP 目标仓库里高频文件类型
- 不能识别时返回保守类型，而不是报错

### RS-008 [TODO] 实现模块路径归一化

目标：

为每个文件生成粗粒度 `module`，供邻域和排序使用。

前置依赖：

- `RS-006`
- `RS-007`

输入：

- 文件相对路径

输出：

- 模块名，例如 `browser/settings`

完成标准：

- 同目录或同子树文件能稳定归入同一模块
- 规则先简单可解释，不需要复杂语义切分

### RS-009 [TODO] 实现 seed 基础邻域扩展

目标：

围绕 `seed_files` 构建第一层候选集。

前置依赖：

- `RS-003`
- `RS-006`
- `RS-008`

输入：

- `seed_files`
- 全仓文件列表

输出：

- 第一批候选文件路径集合

完成标准：

- 至少支持三类扩展：
  - 同目录
  - 同模块
  - 文件名前缀近似匹配
- 候选集中必须保留所有 seed 文件

### RS-010 [TODO] 实现基础关键词规则

目标：

基于文件路径和文件名识别高价值配套文件。

前置依赖：

- `RS-008`

输入：

- 候选文件路径
- `focus_checks`

输出：

- 路径级 heuristic tags 和 heuristic scores

完成标准：

- 至少覆盖：
  - `tests`
  - `default_config`
  - `resources_or_strings`
  - `build_registration`
  - `feature_flag`
- 每个命中项都能写入 `discovered_by` 或 `heuristic_tags`

### RS-011 [TODO] 实现 Browser Settings Profile 规则

目标：

为首个 repo family 提供 profile 化规则，而不是只靠通用规则。

前置依赖：

- `RS-009`

输入：

- `profile=browser_settings`
- 候选文件路径

输出：

- 额外的 profile 命中分和标签

完成标准：

- 至少支持识别：
  - settings page
  - handler/controller
  - prefs registration
  - strings/resources
  - test files
- 规则命中结果可解释

### RS-012 [TODO] 实现轻量符号抽取

目标：

为 `FileCard` 提供足够轻量的符号信息。

前置依赖：

- `RS-006`
- `RS-007`

输入：

- 文件内容

输出：

- 粗粒度符号列表

完成标准：

- 不追求完整 AST
- 至少能提取函数名、类名、常量名中的一部分
- 失败时允许返回空列表，但不能中断流程

### RS-013 [TODO] 生成 FileCard

目标：

把候选文件统一组装成 `FileCard`。

前置依赖：

- `RS-004`
- `RS-008`
- `RS-010`
- `RS-011`
- `RS-012`

输入：

- 候选文件集合
- 路径规则结果
- 符号抽取结果

输出：

- `FileCard[]`

完成标准：

- 每个候选文件都能生成一张 `FileCard`
- seed 文件必须带上 `seed` 来源标记
- 需要给出初始 `heuristic_score`

### RS-014 [TODO] 构建候选集排序器

目标：

对 `FileCard` 做第一版静态排序。

前置依赖：

- `RS-013`

输入：

- `FileCard[]`

输出：

- 按分数排序后的候选集

完成标准：

- 排序逻辑稳定可解释
- 至少合并：
  - seed 权重
  - 同模块权重
  - 关键词规则权重
  - profile 规则权重

### RS-015 [TODO] 组装无模型版 ContextPack

目标：

在没有模型重排的情况下输出第一版 `ContextPack`。

前置依赖：

- `RS-005`
- `RS-014`

输入：

- 排序后的候选集

输出：

- `ContextPack`

完成标准：

- 能产出：
  - `main_chain`
  - `companion_files`
  - `reading_order`
  - `risk_hints`
- 即使结果粗糙，也必须结构完整

### RS-016 [TODO] 实现 `reposcout run --format json`

目标：

把无模型链路接入 CLI。

前置依赖：

- `RS-002`
- `RS-003`
- `RS-015`

输入：

- recon request JSON 文件

输出：

- JSON 格式 `ContextPack`

完成标准：

- 命令可直接运行
- 错误输出可读
- 成功输出合法 JSON

### RS-017 [TODO] 实现 Markdown 渲染

目标：

把 `ContextPack` 渲染成便于人类和 Agent 阅读的摘要文本。

前置依赖：

- `RS-015`

输入：

- `ContextPack`

输出：

- Markdown 字符串

完成标准：

- 至少包含：
  - 任务摘要
  - 推荐先读文件
  - 高优先级配套文件
  - 风险提示
  - 不确定点

### RS-018 [TODO] 实现 `reposcout run --format markdown`

目标：

把 Markdown 输出接入 CLI。

前置依赖：

- `RS-016`
- `RS-017`

输入：

- recon request JSON 文件

输出：

- Markdown 文本

完成标准：

- 命令可以独立输出 Markdown
- 内容与 JSON 输出保持一致

### RS-019 [TODO] 建立 goldens 数据集格式

目标：

定义评测样本目录结构和样本格式。

前置依赖：

- `RS-003`
- `RS-005`

输入：

- 真实任务样本

输出：

- `examples/goldens/` 数据格式约定

完成标准：

- 每个样本至少包含：
  - `task`
  - `recon_request.json`
  - `expected_files.json`
- 文档中明确如何新增样本

### RS-020 [TODO] 实现基础评测器

目标：

对静态版结果计算召回指标。

前置依赖：

- `RS-016`
- `RS-019`

输入：

- goldens 数据集目录

输出：

- Recall 指标和样本级明细

完成标准：

- 至少输出：
  - `Recall@10`
  - `Recall@20`
  - 样本级命中/漏失文件

### RS-021 [TODO] 定义 LLM TaskCard

目标：

把小模型后端的输入固定成统一短任务卡片。

前置依赖：

- `RS-013`

输入：

- `task`
- 单个 `FileCard`
- 上下文压缩信息

输出：

- `TaskCard`

完成标准：

- 至少支持任务类型：
  - `classify_file_role`
  - `judge_relevance`
  - `should_expand`
  - `is_implicit_dependency`
- `TaskCard` 可以直接序列化成 prompt 输入

### RS-022 [TODO] 实现 Provider Adapter 抽象层

目标：

把 RepoScout 与具体模型后端实现解耦。

前置依赖：

- `RS-002`
- `RS-021`

输入：

- `TaskCard`

输出：

- 标准化 provider 推理结果

完成标准：

- 至少定义一个 adapter 接口
- 即使暂时没有真实模型，也能接入 mock adapter
- 适配层请求格式按 OpenAI 兼容 `v1/chat/completions` 设计
- 不把具体 provider 的私有字段泄漏到上层业务逻辑

### RS-023 [TODO] 实现 LLM Worker Pool

目标：

并发执行一批 `TaskCard`。

前置依赖：

- `RS-022`

输入：

- `TaskCard[]`
- `max_concurrency`

输出：

- 标准化任务结果列表

完成标准：

- 能并发调度
- 单任务失败不影响整体流程
- 输出顺序与输入可关联

### RS-024 [TODO] 实现 LLM 结果融合

目标：

把小模型判断结果并入 `FileCard` 和最终排序。

前置依赖：

- `RS-014`
- `RS-023`

输入：

- 静态排序结果
- LLM 任务结果

输出：

- 更新后的排序结果

完成标准：

- 至少把 `label`、`confidence`、`reason` 合并回文件级视图
- 最终打分逻辑可配置
- 无 LLM 结果时可自动退化到静态版

### RS-025 [TODO] 组装模型增强版 ContextPack

目标：

基于融合结果输出增强版 `ContextPack`。

前置依赖：

- `RS-024`

输入：

- 融合后的文件排序和标签结果

输出：

- 带 `model evidence` 的 `ContextPack`

完成标准：

- `main_chain` 和 `companion_files` 中能体现模型证据
- `risk_hints` 能引用模型的不确定判断

### RS-026 [TODO] 实现 `reposcout eval` 对照评测

目标：

比较静态版和模型增强版的差异。

前置依赖：

- `RS-020`
- `RS-025`

输入：

- goldens 数据集

输出：

- 对照评测结果

完成标准：

- 至少比较：
  - 静态版 Recall
  - 模型增强版 Recall
  - 样本级提升与退步

### RS-027 [TODO] 实现 MCP 服务入口

目标：

把 RepoScout 作为 MCP 服务暴露给编程 Agent 客户端。

前置依赖：

- `RS-002`
- `RS-016`
- `RS-018`

输入：

- MCP 请求
- recon request
- 可选 runtime config

输出：

- `internal/mcp/server.go`
- MCP 工具定义

完成标准：

- 至少暴露一个主工具用于执行 recon
- MCP 服务能复用 CLI 的核心业务逻辑，而不是复制一套实现
- 支持读取统一运行配置

### RS-028 [TODO] 准备固定 Demo 样本

目标：

准备一套稳定的演示输入和演示输出。

前置依赖：

- `RS-018`
- `RS-025`

输入：

- 选定的真实任务

输出：

- Demo recon request
- Demo expected files
- Demo markdown output

完成标准：

- 可以一条命令重放
- 可以同时展示静态版和模型增强版结果

---

## 26. 每个功能点的实现模板

后续让 Codex 接手时，每次只做一个功能点，并按下面模板输出：

### 功能点

填写功能点 ID，例如 `RS-012`

### 本次修改

说明新增或修改了哪些文件。

### 实现说明

说明输入、输出、关键逻辑和取舍。

### 验证方式

列出执行过的命令或样例。

### 是否满足完成标准

- 是 / 否
- 如果是否，缺什么

### 下一个建议功能点

默认给出依赖已经满足的下一个 `RS-xxx`

---

## 27. 连续开发规则

为了做到“读完本文档就能继续开发”，后续实现时遵守以下规则：

1. 完成一个功能点后，必须把对应状态从 `[TODO]` 改成 `[DONE]`。
2. 如果部分完成但未达完成标准，改成 `[DOING]` 并补一句阻塞原因。
3. 每次新会话开始，先扫描本文件，找到第一个依赖已满足且状态不是 `[DONE]` 的功能点。
4. 默认实现那个功能点，除非用户明确指定别的 ID。
5. 新增功能如果不在本文档内，先补文档，再写代码。

这样这个文件就不仅是 MVP 描述，而是项目的执行索引。
