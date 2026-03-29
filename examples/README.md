# Examples

本目录里的文件分成两类：

- `recon_request.sample.json`
  可直接对当前仓库 `/home/no22/RepoScout` 运行的示例请求。

- `runtime_config.sample.json`
  运行配置样例。默认用于演示如何开启可选的 LLM rerank。
  其中 `runtime.max_input_tokens` 用于控制单次 LLM 输入预算：
  元数据 prompt 先保留，再尽量用相关代码片段填满剩余预算。

## 直接运行

纯静态模式：

```bash
go run ./cmd/reposcout run examples/recon_request.sample.json
```

Markdown 输出：

```bash
go run ./cmd/reposcout run examples/recon_request.sample.json --format markdown
```

开启 LLM rerank：

```bash
go run ./cmd/reposcout run examples/recon_request.sample.json --config examples/runtime_config.sample.json
```

## 注意

- `recon_request.sample.json` 默认绑定当前仓库路径 `/home/no22/RepoScout`
- 如果你在其他机器或其他路径下运行，需要先修改 `repo_root`
- 如果没有可用的 OpenAI-compatible provider，请不要直接开启 `enable_model_rerank`
