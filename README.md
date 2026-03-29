# RepoScout

RepoScout 是一个给编程 Agent 使用的仓库侦察工具。

它的目标不是直接改代码，而是在大代码仓里先帮 Agent 找出：

- 主链路文件
- 高概率配套文件
- 建议阅读顺序
- 风险提示

然后把这些信息整理成结构化的 `ContextPack`，交给 Codex、Claude Code、Cursor 这类通用 Agent 继续读代码和执行任务。

## 当前定位

- 产品定位：analysis-first 的 repo recon + context pack builder
- 接入形态：CLI 工具
- 静态层职责：做候选文件扩展搜索和收缩
- LLM 层职责：分析候选文件并做 rerank
- 模型策略：可选接 OpenAI-compatible `v1/chat/completions`
- 优化方向：RWKV-first
- 不做的事：不把 RepoScout 做成 LLM 驱动搜索层
- 关键卖点：单机部署、高并发、小任务候选分析

## 文档

- 项目概念说明：[RepoScout_Concept.md](/home/no22/RepoScout/docs/RepoScout_Concept.md)
- MVP 与当前定位：[RepoScout_MVP.md](/home/no22/RepoScout/docs/RepoScout_MVP.md)
- 极简实现路线：[RepoScout_MVP_Lite.md](/home/no22/RepoScout/docs/RepoScout_MVP_Lite.md)
- 当前实现状态：[IMPLEMENTATION_STATUS.md](/home/no22/RepoScout/docs/IMPLEMENTATION_STATUS.md)
- 开发工作约定：[WORK.md](/home/no22/RepoScout/docs/WORK.md)

## 一句话理解

RepoScout 先用静态扩展搜索缩小候选范围，再可选地用小模型分析这些候选文件，最后输出给编程 Agent 使用的 `ContextPack`。
