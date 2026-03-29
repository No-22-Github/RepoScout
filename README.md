# RepoScout

RepoScout 是一个给编程 Agent 用的仓库侦察工具。

它不负责替你改代码，也不打算接管上游 Agent 的规划。它做的事情更朴素一点：先在仓库里把可能相关的文件范围缩出来，再把这些文件整理成一个更适合继续读的 `ContextPack`。

如果你在大仓库里做任务，通常一开始只能抓到主文件，后面很容易漏掉测试、配置、注册项、资源文件，或者一些不在当前目录里的配套实现。RepoScout 想解决的就是这一步。

## 它现在做什么

- 基于 `seed_files` 做静态扩展搜索
- 为候选文件构建 `FileCard`
- 输出 `main_chain`、`companion_files`、`reading_order`、`risk_hints`
- 可选接 OpenAI-compatible 模型，对候选文件做轻量分析和 rerank

一句话说，RepoScout 负责“找候选、理顺上下文”，上游 Agent 负责“继续读、继续想、继续改”。

## 它现在不做什么

- 不做 LLM 驱动搜索
- 不接管上游 Agent 的规划职责
- 不试图一次性把整仓代码都塞给模型

## 适合什么场景

- 你已经有少量 seed files，但不想漏掉配套文件
- 你希望先拿到一份更像“阅读地图”的结果，再继续让 Agent 工作
- 你想把模型调用压在一个比较小、比较稳的候选集合上

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

## 模型接入

当前模型层是可选的，走 OpenAI-compatible `v1/chat/completions`。RepoScout 的默认思路不是“让模型负责搜索”，而是“让模型分析已经找到的候选文件”。

这也意味着：

- 静态层负责召回
- LLM 层负责判断和重排
- `runtime.max_input_tokens` 控制单次候选分析的输入预算

## 文档

- 项目说明与实现思路：[RepoScout_MVP.md](/home/no22/RepoScout/docs/RepoScout_MVP.md)

## 当前状态

项目已经能跑通完整 CLI 主链路，也已经支持接入本地 OpenAI-compatible 后端，例如 RWKV。接下来主要还是继续增强两件事：静态扩展搜索质量，以及候选文件的上下文构建质量。
