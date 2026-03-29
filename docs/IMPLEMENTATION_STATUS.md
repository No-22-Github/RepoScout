# RepoScout 实现状态

更新时间：2026-03-29

## 1. 当前已经有的能力

- 静态侦察主链路：
  - `ReconRequest -> Candidate Expansion -> FileCard -> Ranker -> ContextPack`
- 基础 CLI：
  - `reposcout run`
  - `reposcout eval`
- 静态候选扩展：
  - 同目录
  - 同模块
  - 文件名前缀匹配
  - companion sibling 匹配
- 基础规则与 `browser_settings` profile 规则
- 轻量符号抽取
- `ContextPack` 的 JSON / Markdown 输出
- OpenAI-compatible provider 适配
- 可选 LLM rerank：
  - 当前只接 `classify_file_role`
- 基于 token 预算的上下文构建：
  - 文件概要
  - imports / include 提示
  - 相关声明
  - 相关代码片段

## 2. 当前还没有的能力

- MCP 服务
- `judge_relevance` 主流程接入
- `is_implicit_dependency` 主流程接入
- import / include / registration 关系驱动的更强静态扩展
- 面向真实 provider 的系统级联调文档

## 3. 已明确降级或废弃的方向

下面这些不再作为 RepoScout 的核心方向推进：

- `should_expand`
- 模型参与候选扩展
- LLM 驱动搜索
- 让 RepoScout 接管上游 Agent 的规划职责

原因：

- 上游编程大模型通常已经会给 seed files
- 调用方也可以自行传入额外候选文件
- 再引入一层 LLM 驱动搜索会造成功能重叠
- 会模糊“谁负责扩展决策”的边界

## 4. 当前最重要的工作

1. 增强静态扩展搜索质量
2. 增强 LLM 上下文构建质量
3. 在真实仓库上验证 `classify_file_role` rerank 的收益
4. 再决定是否接更多分析型 LLM 任务
5. 最后补 MCP

## 5. 当前最重要的判断标准

继续开发时，优先看这些问题：

1. 静态扩展是否能稳定给出高质量候选
2. LLM rerank 是否真的改善了 `main_chain` / `companion_files`
3. 额外耗时是否值得
4. 输出结构是否足够稳定，能被上游 Agent 直接消费

## 6. 一句话结论

当前仓库已经是一个可运行的静态分析 + 可选 LLM 分析工具。

下一步不应再扩张成 LLM 驱动搜索系统，而应继续做强：

- 静态扩展搜索
- 候选文件分析
- `ContextPack` 质量
