# RepoScout 项目说明

## 前言项目定位

我想做一个用来辅助大模型的项目，这个项目的目的，不是做一个自动改代码的 Agent，而是做一个给通用代码 Agent 提供“仓库侦察 + 上下文小抄”的工具

## 解决的问题

大代码仓里，像 AOSP 这种，任务来了以后，普通 Agent 往往只能先找到主文件，把功能勉强做出来，但很容易漏掉一些工程上必须考虑的配套文件，比如暗色模式资源、默认配置、测试、注册项之类。 这个项目就是想先用静态分析把范围缩小，再做一层有限的探索，把这些主文件和隐式依赖一起找出来，最后整理成一个 Context Pack，交给 Codex、Claude Code 这种通用 Agent 去继续读代码、规划、甚至改代码

## 大致路线：

1. 主Agent先理解用户需求，然后产出一个ReconRequest，这是调用方传给工具的参数，重点关注seed_files，focus_symbols，focus_checks，这些参数影响到首轮静态分析选出的文件的深度
参考：
```json
{
  "task": "Add a new config option and wire it into the browser settings UI",
  "repo_root": "/path/to/repo",
  "profile": "general",
  "seed_files": [
    "src/browser/settings/foo_page.tsx",
    "src/browser/settings/foo_handler.cc"
  ],
  "seed_dirs": [
    "src/browser/settings"
  ],
  "already_read_files": [
    "src/browser/settings/foo_page.tsx"
  ],
  "focus_symbols": [
    "foo",
    "enable_foo"
  ],
  "focus_checks": [
    "tests",
    "build_registration",
    "default_config",
    "resources_or_strings",
    "docs"
  ],
  "budget": {
    "max_seed_neighbors": 40,
    "expand_depth": 2,
    "max_output_files": 20,
    "max_llm_jobs": 64
  }
}
```

2. 由静态分析+代码逻辑处理，产出FileCard，作为提供给小模型后端的上下文
参考：
```json
{
  "path": "src/browser/settings/foo_handler.cc",
  "kind": "code",
  "lang": "cpp",
  "module": "browser/settings",
  "symbols": [
    "HandleFooToggle",
    "RegisterFooPrefs"
  ],
  "snippets": [
    "void FooHandler::HandleFooToggle(...)",
    "prefs::kEnableFoo"
  ],
  "neighbors": [
    {
      "path": "src/browser/settings/foo_page.tsx",
      "edge": "same_module"
    },
    {
      "path": "src/components/prefs/browser_prefs.cc",
      "edge": "config_registration"
    }
  ],
  "discovered_by": [
    "caller_seed",
    "symbol_hit",
    "same_dir"
  ],
  "hints": [
    "possible_controller",
    "possible_config_bridge"
  ]
}
```

3. 批量并发启动 LLM Worker，附带任务参数
参考：
```jsonc
{
  "task_type": "classify_file_role | judge_relevance | should_expand | is_implicit_dependency",
  "label": "controller",
  "confidence": 0.82,
  "reason": "Contains handler logic and bridges UI action to preference update"
}
```
统一输出参考：
```json
{
  "task_type": "...",
  "label": "...",
  "confidence": 0.0,
  "reason": "..."
}
```

4. 基于模型结果，输出ContextPack，交付给上游Agent，也就是调用方
参考：
```json
{
  "task": "Add a new config option and wire it into the browser settings UI",
  "repo_family": "browser",
  "main_chain": [
    {
      "path": "src/browser/settings/foo_page.tsx",
      "role": "ui_entry",
      "confidence": 0.87,
      "evidence": ["seed", "same_module", "model:entry"]
    },
    {
      "path": "src/browser/settings/foo_handler.cc",
      "role": "controller",
      "confidence": 0.84,
      "evidence": ["symbol_hit", "model:controller"]
    },
    {
      "path": "src/components/prefs/browser_prefs.cc",
      "role": "default_config",
      "confidence": 0.76,
      "evidence": ["config_registration", "model:implicit_dep"]
    }
  ],
  "companion_files": [
    {
      "path": "src/browser/settings/foo_page_strings.grdp",
      "kind": "resources_or_strings",
      "importance": "high",
      "reason": "UI option likely needs visible strings"
    },
    {
      "path": "src/browser/settings/foo_page_browsertest.cc",
      "kind": "test",
      "importance": "high",
      "reason": "Nearby browser test likely covers settings behavior"
    }
  ],
  "uncertain_nodes": [
    {
      "path": "src/browser/flags/foo_features.cc",
      "reason": "May gate feature rollout, but direct usage is weak"
    }
  ],
  "reading_order": [
    "src/browser/settings/foo_page.tsx",
    "src/browser/settings/foo_handler.cc",
    "src/components/prefs/browser_prefs.cc",
    "src/browser/settings/foo_page_browsertest.cc"
  ],
  "risk_hints": [
    "Default value may be registered outside current directory",
    "UI strings and tests are likely required even if main logic compiles"
  ],
  "summary_markdown": "..."
}
```

## 关于RWKV

RWKV的上下文不是优势（只有8k），但是可以单机部署，高并发推理，单张显卡即可达到百级并发会话，这是在线API比不了的
