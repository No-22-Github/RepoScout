# RepoScout

RepoScout 是一个给编程 Agent 使用的仓库侦察工具。

它的目标不是直接改代码，而是在大代码仓里先帮 Agent 找出：

- 主链路文件
- 高概率配套文件
- 建议阅读顺序
- 风险提示

然后把这些信息整理成结构化的 `ContextPack`，交给 Codex、Claude Code、Cursor 这类通用 Agent 继续读代码和执行任务。

## 当前定位

- 产品定位：repo recon + context pack builder
- 接入形态：CLI 工具
- 模型策略：默认静态侦察，可选接 OpenAI 兼容 `v1/chat/completions` 做 rerank
- 优化方向：RWKV-first
- 关键卖点：单机部署、高并发、小任务批量判断

## 文档

- 项目概念说明：[RepoScout_Concept.md](/home/no22/RepoScout/docs/RepoScout_Concept.md)
- MVP 与功能点拆解：[RepoScout_MVP.md](/home/no22/RepoScout/docs/RepoScout_MVP.md)
- 极简实现路线：[RepoScout_MVP_Lite.md](/home/no22/RepoScout/docs/RepoScout_MVP_Lite.md)
- 施工协议：[WORK.md](/home/no22/RepoScout/docs/WORK.md)

## 一句话理解

RepoScout 先用静态分析缩小候选范围，再可选地用高并发小模型任务做轻量重排，最后输出给编程 Agent 使用的上下文小抄。
