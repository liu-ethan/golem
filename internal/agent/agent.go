package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/llm/prompts"
	"github.com/tencent-docs/golem/internal/memory"
	"github.com/tencent-docs/golem/internal/rules"
	"github.com/tencent-docs/golem/internal/sandbox"
	"github.com/tencent-docs/golem/internal/tools"
)

// Agent 实现 LLM 主循环：流式对话、tool_use 解析与工具分发。
type Agent struct {
	projectRoot string
	sessionID   string

	llm       llm.LLMClient
	tools     *tools.Registry
	policy    approval.ApprovalPolicy
	rules     []rules.Rule
	gate      ToolGate
	confirm   ConfirmFunc
	onSlash   SlashHandler
	memory    MemoryProvider
	onSession SessionEndHandler

	systemPrompt   string
	memoryInjected bool
	messages       []llm.Message

	maxTokens int

	memoryCfg    config.MemoryConfig
	contextLimit int
	summaryStore memory.SummaryStore

	sessionInputTokens  int
	sessionOutputTokens int
	hadUserMessages     bool
}

// Options 配置 Agent 可选依赖；未设置的项使用 P0 安全默认值。
type Options struct {
	SessionID   string
	MaxTokens   int
	Policy      approval.ApprovalPolicy
	Rules       []rules.Rule
	Gate        ToolGate
	Confirm     ConfirmFunc
	Slash       SlashHandler
	Memory      MemoryProvider
	OnSession   SessionEndHandler
	InitialMsgs  []llm.Message
	MemoryCfg    config.MemoryConfig
	ContextLimit int
	SummaryStore memory.SummaryStore
	SandboxMode  sandbox.SandboxMode
}

// New 创建绑定 projectRoot 的 Agent，冻结项目根并构建基础 system prompt。
func New(projectRoot string, client llm.LLMClient, opts Options) (*Agent, error) {
	if client == nil {
		return nil, fmt.Errorf("llm client is required")
	}
	systemPrompt, err := prompts.BuildBaseSystemPrompt(projectRoot)
	if err != nil {
		return nil, err
	}
	sessionID := opts.SessionID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	policy := opts.Policy
	if policy == nil && opts.Gate == nil {
		var err error
		policy, err = approval.New(approval.ModeAskBeforeEdit)
		if err != nil {
			return nil, err
		}
	}
	gate := opts.Gate
	if gate == nil {
		gate = GateFromPolicy(policy)
	}
	if len(opts.Rules) > 0 {
		gate = GateWithRules(opts.Rules, gate)
	}
	memory := opts.Memory
	if memory == nil {
		memory = NoopMemoryProvider{}
	}
	onSession := opts.OnSession
	if onSession == nil {
		onSession = NoopSessionEndHandler{}
	}
	msgs := opts.InitialMsgs
	if msgs == nil {
		msgs = []llm.Message{}
	}

	return &Agent{
		projectRoot:  projectRoot,
		sessionID:    sessionID,
		llm:          client,
		tools:        tools.NewRegistry(projectRoot, opts.SandboxMode),
		policy:       policy,
		rules:        opts.Rules,
		gate:         gate,
		confirm:      opts.Confirm,
		onSlash:      opts.Slash,
		memory:       memory,
		onSession:    onSession,
		systemPrompt: systemPrompt,
		messages:     msgs,
		maxTokens:    opts.MaxTokens,
		memoryCfg:    opts.MemoryCfg,
		contextLimit: opts.ContextLimit,
		summaryStore: opts.SummaryStore,
	}, nil
}

// HandleInput 处理用户输入：斜杠命令本地路由，普通消息进入 LLM 主循环。
// 返回 handled 表示输入已被消费（斜杠命令或已成功完成一轮对话）。
func (a *Agent) HandleInput(ctx context.Context, input string, handler EventHandler) (handled bool, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return false, nil
	}
	if strings.HasPrefix(input, "/") {
		if a.onSlash != nil {
			return a.onSlash(input)
		}
		return false, fmt.Errorf("unknown slash command: %s", input)
	}
	if err := a.handleUserMessage(ctx, input, handler); err != nil {
		return true, err
	}
	return true, nil
}

// handleUserMessage 将用户消息送入主循环，首条消息前注入 BM25 记忆（若尚未注入）。
func (a *Agent) handleUserMessage(ctx context.Context, text string, handler EventHandler) error {
	if !a.memoryInjected {
		block, err := a.memory.InjectOnce(ctx, text)
		if err != nil {
			return fmt.Errorf("inject memory: %w", err)
		}
		if block != "" {
			a.systemPrompt += block
		}
		a.memoryInjected = true
	}

	a.messages = append(a.messages, llm.Message{
		Role: llm.RoleUser,
		Content: []llm.ContentBlock{{
			Type: "text",
			Text: text,
		}},
	})
	a.hadUserMessages = true

	return a.runAgentLoop(ctx, handler)
}

// OnSessionEnd 在 /exit、Ctrl+D、SIGINT 等正常退出时调用；P0 委托 SessionEndHandler stub。
func (a *Agent) OnSessionEnd() {
	a.onSession.OnSessionEnd(a.sessionID, a.hadUserMessages)
}

// SessionID 返回当前会话 ID。
func (a *Agent) SessionID() string {
	return a.sessionID
}

// ProjectRoot 返回启动时冻结的项目根目录。
func (a *Agent) ProjectRoot() string {
	return a.projectRoot
}

// SystemPrompt 返回当前 system prompt（含 profile 与已注入的记忆块）。
func (a *Agent) SystemPrompt() string {
	return a.systemPrompt
}

// Messages 返回对话消息的浅拷贝，供测试与持久化使用。
func (a *Agent) Messages() []llm.Message {
	out := make([]llm.Message, len(a.messages))
	copy(out, a.messages)
	return out
}

// MemoryInjected 报告本会话是否已完成 BM25 一次性注入。
func (a *Agent) MemoryInjected() bool {
	return a.memoryInjected
}

// SessionInputTokens 返回本会话累计 input token（经 StreamChat usage 累加）。
func (a *Agent) SessionInputTokens() int {
	return a.sessionInputTokens
}

// ApprovalPolicy 返回当前审批策略；未配置 Policy 时返回 nil。
func (a *Agent) ApprovalPolicy() approval.ApprovalPolicy {
	return a.policy
}

// SetApprovalPolicy 运行时更换审批策略并同步 ToolGate（供 /permissions 与 Shift+Tab）。
func (a *Agent) SetApprovalPolicy(policy approval.ApprovalPolicy) {
	a.policy = policy
	if policy != nil {
		a.gate = GateFromPolicy(policy)
		if len(a.rules) > 0 {
			a.gate = GateWithRules(a.rules, a.gate)
		}
	}
}

// SetGate 运行时更换审批门控；直接设置 gate 时不更新 policy 指针。
func (a *Agent) SetGate(gate ToolGate) {
	if gate == nil {
		gate = GateFromPolicy(mustPolicy(approval.ModeEditAutomatically))
	}
	a.gate = gate
}

// SetConfirm 运行时设置工具确认回调。
func (a *Agent) SetConfirm(fn ConfirmFunc) {
	a.confirm = fn
}

// SandboxMode 返回当前 bash 沙箱模式。
func (a *Agent) SandboxMode() sandbox.SandboxMode {
	return a.tools.SandboxMode()
}

// SetSandboxMode 运行时切换 bash 沙箱模式（供 /sandbox 与 CLI 默认值同步）。
func (a *Agent) SetSandboxMode(mode sandbox.SandboxMode) {
	a.tools.SetSandboxMode(mode)
}

// SetSessionID 切换当前会话 ID，供 TUI /sessions resume 使用。
func (a *Agent) SetSessionID(id string) {
	if id != "" {
		a.sessionID = id
	}
}

// RestoreState 从持久化还原消息历史；resume 后 memoryInjected 应为 false，由调用方设置。
func (a *Agent) RestoreState(messages []llm.Message, memoryInjected bool, summary string) {
	a.messages = messages
	a.memoryInjected = memoryInjected
	if summary != "" && (len(a.messages) == 0 || !memory.IsSummaryMessage(a.messages[0])) {
		a.messages = append([]llm.Message{memory.SummaryMessage(summary)}, a.messages...)
	}
}

// Compact 手动触发 Layer 0 压缩（/compact）；instructions 追加到摘要 prompt。
func (a *Agent) Compact(ctx context.Context, instructions string) (string, error) {
	result, err := a.runCompact(ctx, true, instructions)
	if err != nil {
		return "", err
	}
	if !result.Compacted {
		batch := a.memoryCfg.CompactBatchSize
		if batch <= 0 {
			batch = 10
		}
		return fmt.Sprintf("未压缩：消息数不足（需 > %d 条非 system 消息）或 LLM 未配置", batch), nil
	}
	return fmt.Sprintf("已压缩最旧 %d 条消息为摘要", compactBatchSize(a.memoryCfg)), nil
}

// AddTokenUsage 累加 LLM 调用的 token 用量，供 TokenUsageHook 与 Layer 0 压缩后更新。
func (a *Agent) AddTokenUsage(u llm.Usage) {
	a.sessionInputTokens += u.InputTokens
	a.sessionOutputTokens += u.OutputTokens
}
