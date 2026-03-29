# RepoScout MVP Lite

## 目标

做一个给编程 Agent 使用的 repo recon 工具。

输入：

- 任务描述
- 仓库路径
- seed files

输出：

- `ContextPack`
- 主链路文件
- 配套文件
- 阅读顺序
- 风险提示

原则：

- 先做 CLI，再做 MCP
- 先做静态扩展搜索，再做 LLM 分析增强
- 模型接口走 OpenAI-compatible `v1/chat/completions`
- 默认按 RWKV-first 优化
- 不把 LLM 驱动搜索当作核心方向

## 当前产品收口

RepoScout 当前定位是：

- 静态层负责找候选文件
- LLM 层负责分析候选文件
- 上游大模型负责自己的规划

也就是说：

- 保留并增强静态文件扩展搜索
- 保留并增强 `classify_file_role` 这类分析任务
- 不再把 `should_expand` 一类 LLM 驱动搜索能力作为主方向

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

## 当前重点

1. 增强静态扩展搜索
2. 增强 LLM 上下文构建
3. 在真实仓库上验证纯静态 vs LLM rerank 的差异
4. 视收益决定是否继续接更多分析任务
5. 最后补 MCP

## 一句话

RepoScout 的 MVP 是：先用静态规则把仓库范围缩小，再用小模型分析候选文件，最后把结果整理成给 Agent 用的 `ContextPack`。
