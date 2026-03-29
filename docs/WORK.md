# WORK

你是当前会话中负责推进 RepoScout 的开发 Agent。

当前项目不再使用按 `RS-xxx` 顺序滚动推进的方式。
历史功能点已经基本落地，后续开发以“当前定位”和“当前重点”为准。

---

## 1. 会话开始先读什么

每次新会话开始，按下面顺序读：

1. `README.md`
2. `docs/RepoScout_MVP_Lite.md`
3. `docs/RepoScout_MVP.md`
4. `docs/IMPLEMENTATION_STATUS.md`
5. `docs/WORK.md`

目的：

- 确认当前产品定位
- 确认哪些方向已经废弃
- 确认当前优先级

---

## 2. 当前默认开发方向

如果用户没有明确指定，本项目默认按下面顺序推进：

1. 增强静态扩展搜索
2. 增强 LLM 上下文构建
3. 验证纯静态 vs LLM rerank 的收益
4. 视收益决定是否接更多分析型 LLM 任务
5. 最后补 MCP

不默认推进的方向：

- `should_expand`
- 模型参与候选扩展
- LLM 驱动搜索

---

## 3. 当前执行原则

必须遵守：

- 先读文档，再动代码
- 先明确本轮目标，再实现
- 改代码就要验证
- 代码、测试、文档一起收口
- 不要把 RepoScout 推回到 LLM 驱动搜索路线

---

## 4. 每轮默认 workflow

1. 阅读当前相关文档
2. 明确本轮目标
3. 实现代码
4. 补测试或更新测试
5. 运行格式化
6. 运行相关测试
7. 更新文档
8. 汇报结果

---

## 5. 测试要求

### 5.1 至少要做的事

- 运行直接受影响包的测试
- 如果改动配置、排序、输出结构或主流程，扩大测试范围
- 不能不测就宣称完成

### 5.2 常见需要扩大测试范围的改动

- 配置结构
- 候选扩展搜索
- 排序逻辑
- LLM 上下文构建
- CLI 主入口
- 输出结构

### 5.3 当前常用命令

```bash
gofmt -w <files>
GOCACHE=/tmp/go-build go test ./internal/config ./internal/runner ./internal/heuristics
GOCACHE=/tmp/go-build go run ./cmd/reposcout run examples/recon_request.sample.json --format json --no-rerank
```

---

## 6. 文档更新要求

每轮结束后，至少检查并在必要时更新：

- `docs/RepoScout_MVP_Lite.md`
- `docs/RepoScout_MVP.md`
- `docs/IMPLEMENTATION_STATUS.md`
- `docs/WORK.md`

要求：

- 产品定位和实现一致
- 废弃方向不再继续写成主卖点
- 已实现历史不要继续堆成冗长 todo list

---

## 7. 一句话

RepoScout 现在要做的是：

**继续增强静态扩展搜索和 LLM 候选分析，把结果稳定整理成上游 Agent 可直接使用的 `ContextPack`。**
