# golem

**Go LLM Execution Model** — 用 Go 构建的轻量级 AI 编程 Agent CLI。

[![CI](https://github.com/yourname/golem/actions/workflows/ci.yml/badge.svg)](https://github.com/yourname/golem/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourname/golem)](https://goreportcard.com/report/github.com/yourname/golem)

> 研究 Claude Code 与 Codex CLI 的核心架构差异，在 Go 生态中实现具备**分层记忆管道**、**声明式权限引擎**和**审批交互模式**的下一代 AI 编程助手。

---

## 项目亮点

| 能力 | 传统方案 | golem |
|------|----------|-------|
| 记忆检索 | 全量注入（5000 token 上限） | **BM25 按需检索**，相关记忆 top-K 注入 |
| 记忆分层 | 单层或无 | **三层架构**（Working / Episodic / Semantic） |
| 用户画像 | 无 | 每项目一份 `user_profile.md`，每 3 次会话自动更新 |
| 上下文压缩 | 手动触发 | **滑动窗口自动压缩**（context 超 80% 触发）+ `/compact` + `/context` 可视化 |
| 审批交互 | 无或单一模式 | **四种模式**：plan / 写前确认 / 全手动 / 全自动 |
| 模型接入 | 厂商绑定 | **统一 Anthropic 风格接口**，`base_url` 可配置 |
| 进程隔离 | 仅应用层规则 | **project_root 路径隔离** + bash namespace（P1）；bind mount 为 TODO |
| 部署形态 | 运行时依赖多 | **单静态二进制**，冷启动目标 < 20ms，零运行时依赖 |

**核心差异化：**

- **会记住你的 Agent** — 跨会话提取偏好与项目事实，BM25 检索按需注入，相比全量注入显著降低 Token 消耗
- **安全可控的执行环境** — 审批模式（UX 层）+ YAML 规则（bash 策略层）+ project_root 路径限制（文件工具）+ bash namespace（P1 内核层）
- **模型无关** — Anthropic Messages API 协议，`base_url` 可配 Claude / DeepSeek 等兼容端点
- **Go 原生工程化** — 纯 Go SQLite 驱动（无 CGO）、Bubble Tea TUI、GitHub Actions CI，适合嵌入 CI/CD 与本地开发工作流

---

## 系统架构

```
用户输入
  ↓
[会话管理层]
  ├── 读取历史会话（SQLite）
  ├── BM25 检索相关记忆片段（memory_facts 表）
  └── 注入 system prompt（user_profile.md + top-K 记忆）
  ↓
[Agent 主循环]
  ↓
  LLM API（Streaming SSE，goroutine + channel）
  ↓
  解析 tool_use 块
  ↓
[权限规则层]          ← 仅 bash；deny > ask > allow，先于审批层
  ├── deny 命中 → 直接拒绝，不弹框
  └── allow/ask 命中 → 进入审批层
  ↓
[审批交互层]          ← plan / ask-before-edit / ask / edit-automatically
  ├── plan：只读探索，write/bash 直接拒绝
  ├── ask-before-edit：write / edit / bash 前确认（默认）
  └── ask / edit-automatically
  ↓
[沙箱 / 路径校验]      ← bash：namespace fork；文件工具：project_root 内
  ↓
  工具执行
  ↓（循环直到无 tool_use）
最终回答输出
  ↓
[记忆提取层]
  └── 会话结束（/exit、Ctrl+D、SIGINT）同步提取 → 写入 SQLite
      每 3 次会话触发 Layer 2 合并 → 更新 user_profile.md
```

---

## 功能概览

### Agent 主循环

- Streaming SSE 流式响应，goroutine + channel 解耦 HTTP 与 TUI 渲染
- 自动识别 `tool_use` 块，分发执行并将结果回传 LLM，循环直至完成
- 内置工具集（P0）：`bash`、`read_file`、`write_file`、`edit_file`、`list_dir`、`grep`
- `web_search` 为 P2 可选
- 每次 LLM 调用通过 `TokenUsageHook` 记录 API `usage`，TUI 展示会话 token 累计

### 三层记忆系统

```
Layer 0: Working Memory（工作记忆）
  • 当前会话完整 message history 存于内存
  • 会话累计 input tokens 达 context_limit × 80% 时自动压缩
  • 支持 /compact 手动触发（P1）

Layer 1: Episodic Memory（情节记忆）
  • 会话结束（/exit、Ctrl+D、SIGINT）后同步提取 3–5 条事实片段
  • 存入 .golem/data/golem.db 的 memory_facts 表
  • 本会话第一条 user 消息时 BM25 检索 top-K（StreamChat 之前，仅一次）

Layer 2: Semantic Memory（语义记忆）
  • 每 3 次会话触发，合并 facts → .golem/user_profile.md
  • 合并后清空 memory_facts，避免与 profile 重复注入
  • 始终注入 system prompt
```

### 审批交互模式

| 模式 | 行为 | 适用场景 |
|------|------|----------|
| `plan` | 只读：read/list/grep 自动；write/edit/bash **直接拒绝** | 探索代码库、做方案 |
| `ask-before-edit` | 读操作自动，write/edit/bash 前确认 | **默认推荐** |
| `ask` | 任意 tool 执行前均弹确认 | 谨慎模式 |
| `edit-automatically` | 全部自动执行（rules.ask 仍确认） | 完全信任环境 |

启动：`golem --approval <mode>`。运行时：`Shift+Tab` 循环切换，或 `/permissions` 打开权限页（切换 mode + 查看 rules）。

### Sandbox × Approval（常见组合）

| 组合 | 适用 |
|------|------|
| `workspace-write` + `plan` | 安全浏览，只看不改 |
| `workspace-write` + `ask-before-edit` | 日常开发（**默认**） |
| `workspace-write` + `edit-automatically` | 可信仓库快速迭代 |
| `danger-full-access` + 任意 | bash 无 namespace；**文件仍限 project_root** |

### TUI 斜杠命令（对齐 Claude Code / Codex CLI）

| 命令 | 优先级 | 说明 |
|------|--------|------|
| `/help` | P0 | 命令与快捷键 |
| `/permissions` | P0 | **权限页**（≈ Claude / Codex）：切换 approval 模式 + 查看 rules；P1 起支持增删；P2 起加 Recently denied |
| `/permissions <mode>` | P0 | 直接设定四种 mode |
| `/sessions` | P0 | **会话列表页**，点选 resume（≈ Claude `/resume`） |
| `/exit` | P0 | 正常结束并退出 |
| `/status` | P1 | 显示 model / approval / sandbox / session / tokens 概览 |
| `/model [model]` | P1 | 运行时切换 LLM 模型 |
| `/clear` | P1 | 清空上下文开新会话（保留 user_profile） |
| `/compact [instructions]` | P1 | 手动触发 Layer 0 压缩 |
| `/context` | P1 | 可视化 context 占用 |
| `/diff` | P1 | 显示 working tree git diff（含 untracked） |
| `/sandbox` | P1 | 循环切换 sandbox |
| `/sandbox <mode>` | P1 | 设定 sandbox 模式 |
| `/review [target]` | P2 | 对 working tree / diff / commit 跑 code review |
| `/memories` | P2 | 查看/管理 `memory_facts`（golem 差异化卖点） |
| `/usage` | P2 | 会话 token 成本与统计（alias `/cost`） |
| `/fork` | P2 | 当前会话分叉到新 session |
| `/export [file]` | P2 | 导出当前会话为 markdown / 纯文本 |
| `/rename [name]` | P2 | 重命名当前 session |
| `/plan <query>` | P2 | 单 prompt plan 模式前缀，跑完回原 mode |
| `/skills` | P2 | **Skill 列表页**，浏览并切换 |
| `/init` | P2 | 为当前项目生成 `AGENTS.md` 模板 |
| `/rewind` | P3 | 回滚到检查点（代码 + 对话；alias `/undo`） |
| `/doctor` | P3 | 环境诊断：API 连通、namespace、SQLite、磁盘 |

**快捷键：** `Shift+Tab` 切换 approval；流式中 `Ctrl+C` 取消当前轮；空闲 `Ctrl+C` / `Ctrl+D` 等同 `/exit`；`Ctrl+L` 清屏；`Esc×2` 编辑上一条（P2）；`Tab` 排队下轮输入（P2）；`Ctrl+G` 调外部编辑器（P2）。工具确认：`Y`/Enter 执行，`n`/Esc 拒绝。

### 权限规则引擎

**仅匹配 `bash` 工具的 command 字符串**；文件工具靠 project_root 路径限制。

基于 YAML 的声明式规则，无需手写 DSL parser：

```yaml
# ~/.golem/rules.yaml
rules:
  - action: allow
    pattern: "go *"
  - action: allow
    pattern: "git *"
  - action: ask
    pattern: "curl *"
  - action: deny
    pattern: "rm -rf *"
  - action: deny
    pattern: "wget *"

priority: deny > ask > allow
```

### Sandbox 与路径隔离

| 模式 | P1 行为 | 说明 |
|------|---------|------|
| `workspace-write`（默认） | bash：namespace fork；文件工具：project_root 内 | bind mount 磁盘隔离为进阶 TODO |
| `danger-full-access` | bash 无 namespace | 文件工具仍限制在 project_root（≠ Codex 完全放开） |

**项目根目录：** 你在哪个目录执行 `golem`，哪里就是 project_root（启动时冻结）。文件工具（read_file/write_file/edit_file/list_dir/grep）路径校验限制在此目录内；bash 的 cwd 设为 project_root，但 bash 自身可 `cd` 越界——P1 namespace 仅额外隔离，无 bind mount 时不能阻止 bash 写项目外文件。

不支持 user namespace 时 bash 降级为直接 exec + 警告，路径校验与 rules 仍生效。

### Skills 系统（P2）

三层能力架构：Core Tools（编译进二进制）→ Skills（专家人格配置）→ MCP Servers（P4，暂不实现）。

```bash
golem --skill golang-expert              # 启动时指定 Skill
/skills                                  # P2：Skill 列表页
golem skill install github:user/repo/skill-name
```

Skill 支持 `SKILL.md` 与 `skill.json` 两种格式，来源优先级：项目级 > 全局 > 内置。

---

## 快速开始

### 环境要求

- Go 1.22+
- Linux（Namespace 沙箱功能；其他平台可降级为权限规则模式）

### 安装

```bash
git clone https://github.com/yourname/golem.git
cd golem
go build -o golem ./cmd/golem
sudo mv golem /usr/local/bin/   # 可选
```

### 配置

优先级：**字段级 merge**（`~/.golem/config.yaml` 为 base，项目 `.golem/config.yaml` 覆盖同名字段）；CLI flag 覆盖 `defaults`。

```yaml
# .golem/config.yaml（推荐）或 ~/.golem/config.yaml
provider:
  base_url: "https://api.anthropic.com"          # DeepSeek: https://api.deepseek.com/anthropic
  api_key: "${ANTHROPIC_API_KEY}"
  model: "claude-sonnet-4-5"
  context_limit: 200000                          # TUI token 分母 & Layer 0 阈值

defaults:
  approval: ask-before-edit    # plan | ask-before-edit | ask | edit-automatically
  sandbox: workspace-write

memory:
  layer2_session_threshold: 3
  bm25_top_k: 5
  compact_batch_size: 10
  compact_threshold: 0.8
```

权限规则：合并加载 `.golem/rules.yaml` + `~/.golem/rules.yaml`（项目在前）。
数据目录：`.golem/data/golem.db`（SQLite，自动创建）。

### 基本用法

```bash
cd my-project                              # 此处即为 project_root
golem                                      # 启动 TUI，新建会话
golem --resume <session-id>              # 恢复历史（也可用 TUI /sessions）
golem --approval plan                    # 只读探索模式
golem --approval ask-before-edit         # 默认
golem --sandbox workspace-write          # 默认 sandbox
golem "帮我读一下 main.go 并总结"         # headless（P2）
golem sessions list                      # CLI 列会话（P2）；P0 用 /sessions
golem sessions delete <id>               # 删除会话（P2）
```

TUI：`Shift+Tab` 切 approval；`/permissions` `/sessions` `/status` `/model` `/clear` `/compact` `/context` `/diff` `/help` `/exit`。

---

## TUI 界面预览

```
╭─ golem ──────────────────────────────────────────────╮
│ 📁 ~/projects/my-app  🔒 ask-before-edit  📦 workspace-write │
│ 💬 Session: abc123  📊 Tokens: 12.4k / 200k         │
╰──────────────────────────────────────────────────────╯
  Claude: 我已分析了 internal/memory/ 目录，发现...

  ┌─ Tool: read_file ─────────────────────────────────┐
  │ path: internal/memory/layer1.go                   │
  │ [✓ 已执行]                                         │
  └───────────────────────────────────────────────────┘

  ┌─ Tool: write_file ────────────────────────────────┐
  │ path: internal/memory/layer1.go                   │
  │ [ask-before-edit 模式] 是否允许？ [Y/n]            │
  └───────────────────────────────────────────────────┘

> _
```

基于 [Bubble Tea](https://github.com/charmbracelet/bubbletea) 构建，支持流式逐字渲染与工具调用卡片展示。

---

## 项目结构

```
golem/
├── cmd/golem/              # CLI 入口
├── internal/
│   ├── llm/                # LLMClient 接口与 Streaming 实现
│   ├── agent/              # Agent 主循环与 tool_use 分发
│   ├── tools/              # 内置工具集
│   ├── approval/           # 审批交互模式
│   ├── rules/              # YAML 权限规则引擎
│   ├── sandbox/            # Linux Namespace 沙箱
│   ├── session/            # SQLite 会话持久化（.golem/data/golem.db）
│   ├── memory/             # 三层记忆系统 + BM25 检索
│   └── tui/                # Bubble Tea + pages（permissions/sessions；P1 起追加 model/context；P2 起追加 memories/skills）
│   # P2: internal/skills/
├── .github/workflows/      # CI（go test + go vet + go build）
└── .golem/                 # 项目级配置
    ├── config.yaml         # LLM + defaults（gitignore）
    ├── rules.yaml          # 权限规则
    ├── user_profile.md     # Layer 2 用户画像
    └── data/
        └── golem.db        # SQLite
```

---

## 技术栈

| 类别 | 技术 |
|------|------|
| 语言 | Go 1.22+ |
| LLM | Anthropic Messages API，`base_url` 可配 Claude / DeepSeek 等 |
| 数据库 | SQLite（[modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)，纯 Go，无 CGO） |
| TUI | [Bubble Tea](https://github.com/charmbracelet/bubbletea) |
| 配置 | YAML（gopkg.in/yaml.v3） |
| 沙箱 | golang.org/x/sys/unix（Linux namespace） |
| CI | GitHub Actions |

---

## 设计参考

golem 在架构设计上深入研究了以下开源项目：

| 项目 | 语言 | 借鉴点 |
|------|------|--------|
| [openai/codex](https://github.com/openai/codex) | Rust | 审批模式、两阶段记忆管道、权限规则引擎 |
| [AlleyBo55/gocode](https://github.com/AlleyBo55/gocode) | Go | Skills 系统、多 Agent、Model Fallback |
| [ProjectBarks/gopher-code](https://github.com/ProjectBarks/gopher-code) | Go | 极简 Go 实现，毫秒级冷启动 |
| [Kuberwastaken/claurst](https://github.com/Kuberwastaken/claurst) | Rust | ACP 协议支持 |

golem 在上述基础上，针对 Go 生态做了记忆检索、用户画像和安全沙箱的差异化增强。

---

## 开发

```bash
# 运行测试
go test ./...

# 静态检查
go vet ./...

# 构建
go build ./cmd/golem
```

核心模块（记忆提取、BM25 检索、权限规则引擎、Agent tool_use 解析）均覆盖单元测试，CI 在每次 push 时自动运行。

---

## 路线图

| 优先级 | 模块 | 状态 |
|--------|------|------|
| P0 | Agent 主循环、工具集、四种 approval + Shift+Tab、TUI 斜杠页（/permissions、/sessions、/help、/exit）、Session、配置 | 进行中 |
| P1 | 三层记忆系统（BM25）、权限规则引擎、Namespace 沙箱、CI Pipeline、日常命令（/status、/model、/clear、/compact、/context、/diff、/sandbox、Ctrl+L）、/permissions 增删 rules | 计划中 |
| P2 | headless、/review、/memories、/usage、/fork、/export、/rename、/plan\<query\>、/init、/skills 列表页、/permissions Recently denied、Skills install、web_search、sessions delete、Esc×2/Tab/Ctrl+G | 计划中 |
| P3 | /rewind、/doctor、Embedding 向量检索（HybridRetriever）、Web UI | 待定 |
| P4 | MCP Servers | 不做 |
