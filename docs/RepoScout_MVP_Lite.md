# RepoScout MVP Lite

## 目标

做一个给编程 Agent 使用的 repo recon 工具。

输入：

- 任务描述
- seed files

输出：

- `ContextPack`
- 主链路文件
- 配套文件
- 阅读顺序
- 风险提示

原则：

- 先做 CLI，再做 MCP
- 先做静态版，再接模型增强
- 模型接口走 OpenAI 兼容 `v1/chat/completions`
- 后端可替换，但默认按 RWKV-first 优化

## 实现路线

### Phase 1：跑通静态版

1. 初始化项目骨架
2. 定义配置结构
3. 定义 `ReconRequest` / `FileCard` / `ContextPack`
4. 实现 repo 扫描
5. 实现语言识别和模块归一化
6. 基于 seed 做候选集扩展
7. 实现基础规则
8. 生成 `FileCard`
9. 排序候选集
10. 组装静态版 `ContextPack`
11. 接入 `reposcout run --format json`
12. 接入 `reposcout run --format markdown`

Phase 1 完成标准：

- 能从一个 recon request 输出结构完整的 `ContextPack`
- 不依赖模型也能工作

### Phase 2：接入模型增强

1. 定义 `TaskCard`
2. 实现 provider 抽象层
3. 实现 LLM Worker Pool
4. 把模型结果融合回排序
5. 输出模型增强版 `ContextPack`

Phase 2 完成标准：

- 支持高并发小任务判断
- 无模型结果时可自动退化到静态版

### Phase 3：评测与接入

1. 定义 goldens 数据集格式
2. 实现 `reposcout eval`
3. 比较静态版和模型增强版
4. 实现 MCP 服务入口
5. 准备固定 demo 样本

Phase 3 完成标准：

- 能做基础召回评测
- 能作为 CLI 或 MCP 工具接入编程 Agent

## 最小模块

```text
cmd/reposcout/
internal/config/
internal/schema/
internal/scanner/
internal/heuristics/
internal/ranking/
internal/llm/
internal/output/
internal/eval/
internal/mcp/
examples/
```

## 当前推荐顺序

按下面顺序做就行：

1. `RS-001` 项目骨架
2. `RS-002` 统一配置
3. `RS-003` 到 `RS-005` 三个 schema
4. `RS-006` 到 `RS-015` 静态链路
5. `RS-016` 到 `RS-020` CLI 与评测基础
6. `RS-021` 到 `RS-026` 模型增强
7. `RS-027` MCP
8. `RS-028` demo

## 一句话

RepoScout 的 MVP 就是：先用静态规则把仓库范围缩小，再用高并发小模型任务做轻量判断，最后把结果整理成给 Agent 用的 `ContextPack`。
