package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/memory"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/tui/style"
)

// agentEventMsg 将 Agent 事件从 goroutine 投递到 Bubble Tea Update。
type agentEventMsg agent.Event

// agentDoneMsg 表示一轮 Agent 处理结束。
type agentDoneMsg struct {
	err error
}

// confirmRequestMsg 请求 TUI 弹出工具确认框。
type confirmRequestMsg struct {
	toolName string
	input    map[string]any
	resp     chan bool
}

// sessionsOpenMsg 携带 /sessions 页列表数据。
type sessionsOpenMsg struct {
	entries []session.Entry
	err     error
}

// sessionResumeDataMsg 携带 resume 加载结果，由 Update 写入 Model。
type sessionResumeDataMsg struct {
	sessionID string
	summary   string
	messages  []llm.Message
	err       error
}

// compactDoneMsg 表示 /compact 手动压缩完成。
type compactDoneMsg struct {
	message string
	err     error
}

// clearContextEndMsg 表示 /clear 后台会话收尾完成（无 UI 更新）。
type clearContextEndMsg struct{}

// bindRunConfirm 为当前 Agent 轮次注册工具确认回调；须在启动 goroutine 之前同步调用。
func (m *Model) bindRunConfirm(ctx context.Context) {
	confirm := func(toolName string, input map[string]any) (bool, error) {
		resp := make(chan bool, 1)
		if m.program == nil {
			return false, fmt.Errorf("tui program not initialized")
		}
		m.program.Send(confirmRequestMsg{
			toolName: toolName,
			input:    input,
			resp:     resp,
		})
		select {
		case ok := <-resp:
			return ok, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	m.agent.SetConfirm(confirm)
}

// startAgentRun 在 goroutine 中执行 Agent.HandleInput，通过 program.Send 推送事件。
func (m *Model) startAgentRun(input string) {
	ctx, cancel := context.WithCancel(context.Background())
	m.runCancel = cancel
	m.running = true
	m.streaming = ""
	m.thinkingStreaming = ""
	m.streamStarted = false
	m.errMsg = ""
	m.bindRunConfirm(ctx)

	go func() {
		handler := func(evt agent.Event) {
			if m.program != nil {
				m.program.Send(agentEventMsg(evt))
			}
		}

		_, err := m.agent.HandleInput(ctx, input, handler)
		if err != nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		if m.program != nil {
			m.program.Send(agentDoneMsg{err: err})
		}
	}()
}

// startAgentPlan 在 plan 模式下执行单条 query。
func (m *Model) startAgentPlan(input string) {
	ctx, cancel := context.WithCancel(context.Background())
	m.runCancel = cancel
	m.running = true
	m.streaming = ""
	m.thinkingStreaming = ""
	m.streamStarted = false
	m.errMsg = ""
	m.bindRunConfirm(ctx)

	go func() {
		handler := func(evt agent.Event) {
			if m.program != nil {
				m.program.Send(agentEventMsg(evt))
			}
		}
		err := m.agent.RunPlanOnce(ctx, input, handler)
		if err != nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		if m.program != nil {
			m.program.Send(agentDoneMsg{err: err})
		}
	}()
}

// startAgentSkill 以指定 Skill 执行单条 query，仅本轮生效。
func (m *Model) startAgentSkill(skillName, query string) {
	if m.skillLoader == nil {
		if m.program != nil {
			m.program.Send(agentDoneMsg{err: fmt.Errorf("skill loader unavailable")})
		}
		return
	}
	skill, err := m.skillLoader.LoadByName(skillName)
	if err != nil {
		if m.program != nil {
			m.program.Send(agentDoneMsg{err: err})
		}
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.runCancel = cancel
	m.running = true
	m.streaming = ""
	m.thinkingStreaming = ""
	m.streamStarted = false
	m.errMsg = ""
	m.bindRunConfirm(ctx)

	go func() {
		handler := func(evt agent.Event) {
			if m.program != nil {
				m.program.Send(agentEventMsg(evt))
			}
		}
		err := m.agent.RunSkillOnce(ctx, skill, query, handler)
		if err != nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		if m.program != nil {
			m.program.Send(agentDoneMsg{err: err})
		}
	}()
}

// cancelAgentRun 取消当前 Agent 流式轮次。
func (m *Model) cancelAgentRun() {
	if m.runCancel != nil {
		m.runCancel()
	}
}

// handleAgentEvent 将 Agent 事件映射到聊天区 UI 状态。
func (m *Model) handleAgentEvent(evt agent.Event) {
	switch evt.Type {
	case agent.EventThinkingDelta:
		m.thinkingStreaming += evt.Text
	case agent.EventTextDelta:
		m.flushThinking()
		if !m.streamStarted {
			m.streamStarted = true
		}
		m.streaming += evt.Text
	case agent.EventToolStart:
		m.flushThinking()
		m.flushStreaming()
		m.lines = append(m.lines, ChatLine{
			Kind:      LineTool,
			ToolName:  evt.ToolName,
			ToolInput: cloneMap(evt.ToolInput),
			ToolState: ToolRunning,
		})
	case agent.EventToolComplete:
		m.updateLastTool(evt.ToolName, evt.ToolInput, evt.ToolOutput, evt.ToolError)
	case agent.EventTurnComplete:
		m.flushThinking()
		m.flushStreaming()
	case agent.EventError:
		if evt.Err != nil {
			m.errMsg = evt.Err.Error()
		}
	}
}

// handleAgentDone 在一轮 Agent 结束后清理状态并持久化消息。
func (m *Model) handleAgentDone(msg agentDoneMsg) {
	m.running = false
	m.runCancel = nil
	m.confirm = nil
	m.flushThinking()
	m.flushStreaming()
	m.syncStatus()

	if msg.err != nil && msg.err != context.Canceled {
		m.errMsg = msg.err.Error()
		m.lines = append(m.lines, ChatLine{
			Kind: LineSystem,
			Text: "Error: " + msg.err.Error(),
		})
	} else {
		m.errMsg = ""
	}
	if msg.err == nil || msg.err == context.Canceled {
		_ = syncMessages(m.store, m.agent)
	}
	m.drainInputQueue()
}

func (m *Model) flushThinking() {
	text := strings.TrimSpace(m.thinkingStreaming)
	if text == "" {
		m.thinkingStreaming = ""
		return
	}
	m.lines = append(m.lines, ChatLine{
		Kind: LineThinking,
		Text: text,
	})
	m.thinkingStreaming = ""
}

func (m *Model) flushStreaming() {
	text := strings.TrimSpace(m.streaming)
	if text == "" {
		m.streaming = ""
		m.streamStarted = false
		return
	}
	m.lines = append(m.lines, ChatLine{
		Kind: LineAssistant,
		Text: text,
	})
	m.streaming = ""
	m.streamStarted = false
}

func (m *Model) updateLastTool(name string, input map[string]any, output string, isErr bool) {
	for i := len(m.lines) - 1; i >= 0; i-- {
		line := &m.lines[i]
		if line.Kind != LineTool || line.ToolName != name {
			continue
		}
		if input != nil {
			line.ToolInput = cloneMap(input)
		}
		line.ToolOutput = output
		line.ToolError = isErr
		if toolOutputIsPolicyDenied(output) {
			line.ToolState = ToolDenied
		} else {
			line.ToolState = ToolDone
		}
		return
	}
}

// toolOutputIsPolicyDenied 判断工具结果是否来自审批层或权限规则拒绝，而非 bash 等非零退出码。
func toolOutputIsPolicyDenied(output string) bool {
	return strings.HasPrefix(output, "Error: denied by") ||
		strings.HasPrefix(output, "Error: user denied") ||
		strings.HasPrefix(output, "Error: tool confirmation unavailable")
}

func (m *Model) syncStatus() {
	m.status.Approval = m.policy.Mode()
	m.status.InputTokens = m.agent.SessionInputTokens()
	m.status.SessionID = shortID(m.agent.SessionID())
}

// rebuildChatFromMessages 从持久化消息重建聊天区（resume 时使用）。
func rebuildChatFromMessages(msgs []llm.Message) []ChatLine {
	var lines []ChatLine
	for _, msg := range msgs {
		switch msg.Role {
		case llm.RoleUser:
			for _, block := range msg.Content {
				switch block.Type {
				case "text":
					if strings.HasPrefix(block.Text, memory.SummaryMessagePrefix) {
						lines = append(lines, ChatLine{
							Kind: LineSystem,
							Text: block.Text,
						})
					} else if strings.TrimSpace(block.Text) != "" {
						lines = append(lines, ChatLine{
							Kind: LineUser,
							Text: block.Text,
						})
					}
				}
			}
		case llm.RoleAssistant:
			var textParts []string
			for _, block := range msg.Content {
				switch block.Type {
				case "text":
					if strings.TrimSpace(block.Text) != "" {
						textParts = append(textParts, block.Text)
					}
				case "tool_use":
					if len(textParts) > 0 {
						lines = append(lines, ChatLine{
							Kind: LineAssistant,
							Text: strings.Join(textParts, "\n"),
						})
						textParts = nil
					}
					lines = append(lines, ChatLine{
						Kind:      LineTool,
						ToolName:  block.Name,
						ToolInput: cloneMap(block.Input),
						ToolState: ToolDone,
					})
				}
			}
			if len(textParts) > 0 {
				lines = append(lines, ChatLine{
					Kind: LineAssistant,
					Text: strings.Join(textParts, "\n"),
				})
			}
		}
	}
	return lines
}

func (m *Model) upsertToolConfirmLine(name string, input map[string]any) {
	for i := len(m.lines) - 1; i >= 0; i-- {
		if m.lines[i].Kind == LineTool && m.lines[i].ToolName == name {
			m.lines[i].ToolInput = cloneMap(input)
			m.lines[i].ToolState = ToolConfirm
			return
		}
	}
	m.lines = append(m.lines, ChatLine{
		Kind:      LineTool,
		ToolName:  name,
		ToolInput: cloneMap(input),
		ToolState: ToolConfirm,
	})
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// formatToolCard 渲染单张工具卡片文本（供 view 与测试使用）。
func formatToolCard(line ChatLine, width int, projectRoot string) string {
	if width < 20 {
		width = 20
	}
	var b strings.Builder
	b.WriteString(style.Border.Render("┌─ Tool: "))
	b.WriteString(style.Accent.Render(line.ToolName))
	b.WriteString(" ")
	b.WriteString(style.Border.Render(strings.Repeat("─", max(0, width-len(line.ToolName)-10))))
	b.WriteString("\n")
	if detail := formatToolDetail(line.ToolName, line.ToolInput, projectRoot, width); detail != "" {
		b.WriteString(detail)
		if !strings.HasSuffix(detail, "\n") {
			b.WriteString("\n")
		}
	}
	if line.ToolName == "bash" && line.ToolState == ToolDone {
		b.WriteString(formatBashOutput(line.ToolOutput, line.ToolError, width))
	}
	switch line.ToolState {
	case ToolRunning:
		b.WriteString("│ " + style.AccentAlt.Render("[执行中…]") + "\n")
	case ToolConfirm:
		b.WriteString("│ " + style.Warning.Render("是否允许？ [Y/n]") + "\n")
	case ToolDenied:
		b.WriteString("│ " + style.ErrText.Render("[已拒绝] "+truncateRunes(line.ToolOutput, width-12)) + "\n")
	case ToolDone:
		if line.ToolName == "bash" {
			if line.ToolError {
				b.WriteString("│ " + style.ErrText.Render("[✗ 执行失败]") + "\n")
			} else {
				b.WriteString("│ " + style.Success.Render("[✓ 已执行]") + "\n")
			}
		} else if line.ToolError {
			b.WriteString("│ " + style.ErrText.Render("[错误] "+truncateRunes(line.ToolOutput, width-10)) + "\n")
		} else {
			b.WriteString("│ " + style.Success.Render("[✓ 已执行]") + "\n")
		}
	default:
		b.WriteString("│ " + style.Muted.Render("[等待…]") + "\n")
	}
	b.WriteString(style.Border.Render("└" + strings.Repeat("─", width-2) + "┘"))
	return b.String()
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
