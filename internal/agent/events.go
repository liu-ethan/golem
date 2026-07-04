package agent

import "context"

// EventType 标识 Agent 向 TUI 上报的事件种类。
type EventType string

const (
	EventTextDelta    EventType = "text_delta"
	EventToolStart    EventType = "tool_start"
	EventToolComplete EventType = "tool_complete"
	EventTurnComplete EventType = "turn_complete"
	EventError        EventType = "error"
)

// Event 为 Agent 主循环产生的单条 UI 事件。
type Event struct {
	Type EventType

	Text string

	ToolUseID string
	ToolName  string
	ToolInput map[string]any
	ToolOutput string
	ToolError  bool

	Err error
}

// EventHandler 接收 Agent 流式与工具执行事件；由 TUI 在 Step 7 实现。
type EventHandler func(Event)

// ConfirmFunc 在审批层要求确认时询问用户；返回 false 表示拒绝执行。
type ConfirmFunc func(toolName string, input map[string]any) (bool, error)

// SlashHandler 处理以 / 开头的本地斜杠命令；handled 为 true 表示已消费输入、不再送 LLM。
type SlashHandler func(cmd string) (handled bool, err error)

// MemoryProvider 在首条用户消息后、首次 StreamChat 前注入 BM25 记忆块；P0 可用 NoopMemoryProvider。
type MemoryProvider interface {
	InjectOnce(ctx context.Context, query string) (string, error)
}

// NoopMemoryProvider 不注入任何记忆，供 P0 在 BM25 模块（Step 11/13）接入前使用。
type NoopMemoryProvider struct{}

// InjectOnce 返回空字符串，表示无额外记忆可注入。
func (NoopMemoryProvider) InjectOnce(_ context.Context, _ string) (string, error) {
	return "", nil
}

// SessionEndHandler 在会话正常结束时调用；P0 为 stub，P1 由 memory 包实现 Layer 1。
type SessionEndHandler interface {
	OnSessionEnd(sessionID string, hadUserMessages bool)
}

// NoopSessionEndHandler 不执行任何会话结束逻辑。
type NoopSessionEndHandler struct{}

// OnSessionEnd 为空实现。
func (NoopSessionEndHandler) OnSessionEnd(_ string, _ bool) {}
