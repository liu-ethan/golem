package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/tui/pages"
)

// Model 是 Bubble Tea 主状态机，承载聊天、子页面与 Agent 桥接。
type Model struct {
	agent   *agent.Agent
	store   *session.Store
	policy  *approval.Policy
	program *tea.Program

	lines         []ChatLine
	input         string
	streaming     string
	streamStarted bool
	confirm       *ConfirmState

	status StatusBar

	projectRoot string
	activePage  PageKind
	permissions PermissionsPage
	sessions    SessionsPage
	rulesLines  []string

	running   bool
	runCancel context.CancelFunc
	width     int
	height    int
	errMsg    string
	quitting  bool
}

// Config 启动 TUI 所需的依赖与展示参数。
type Config struct {
	ProjectRoot  string
	Agent        *agent.Agent
	Store        *session.Store
	Policy       *approval.Policy
	Sandbox      string
	ModelName    string
	ContextLimit int
	RulesLines   []string
}

// NewModel 根据 Config 构造初始 Bubble Tea Model。
func NewModel(cfg Config) Model {
	status := StatusBar{
		ProjectRoot:  cfg.ProjectRoot,
		Approval:     cfg.Policy.Mode(),
		Sandbox:      cfg.Sandbox,
		SessionID:    shortID(cfg.Agent.SessionID()),
		Model:        cfg.ModelName,
		ContextLimit: cfg.ContextLimit,
		InputTokens:  cfg.Agent.SessionInputTokens(),
	}
	if status.ContextLimit <= 0 {
		status.ContextLimit = 200000
	}
	if status.Sandbox == "" {
		status.Sandbox = "workspace-write"
	}

	m := Model{
		agent:       cfg.Agent,
		store:       cfg.Store,
		policy:      cfg.Policy,
		status:      status,
		projectRoot: cfg.ProjectRoot,
		activePage:  PageChat,
		rulesLines:  cfg.RulesLines,
		permissions: PermissionsPage{Cursor: approvalModeIndex(cfg.Policy.Mode())},
		width:       80,
		height:      24,
	}
	if len(cfg.Agent.Messages()) > 0 {
		m.lines = rebuildChatFromMessages(cfg.Agent.Messages())
	}
	return m
}

// Init 实现 tea.Model。
func (m Model) Init() tea.Cmd {
	return nil
}

// Update 实现 tea.Model。
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case agentEventMsg:
		if m.running {
			m.handleAgentEvent(agent.Event(msg))
		}
		return m, nil

	case agentDoneMsg:
		m.handleAgentDone(msg)
		return m, nil

	case confirmRequestMsg:
		m.confirm = &ConfirmState{
			ToolName: msg.toolName,
			Input:    msg.input,
			RespCh:   msg.resp,
		}
		m.updateLastToolState(msg.toolName, ToolConfirm)
		return m, nil

	case sessionsOpenMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.activePage = PageSessions
		m.sessions.Entries = msg.entries
		m.sessions.Cursor = 0
		for i, e := range msg.entries {
			if e.ID == m.agent.SessionID() {
				m.sessions.Cursor = i
				break
			}
		}
		return m, nil

	case sessionResumeDataMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: msg.err.Error()})
			m.activePage = PageChat
			return m, nil
		}
		m.agent.SetSessionID(msg.sessionID)
		m.agent.RestoreState(msg.messages, false, msg.summary)
		m.lines = rebuildChatFromMessages(m.agent.Messages())
		m.streaming = ""
		m.syncStatus()
		m.lines = append(m.lines, ChatLine{
			Kind: LineSystem,
			Text: fmt.Sprintf("已恢复会话 %s", shortID(msg.sessionID)),
		})
		m.activePage = PageChat
		return m, nil

	case compactDoneMsg:
		m.syncStatus()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "Compact 失败: " + msg.err.Error()})
			return m, nil
		}
		m.lines = rebuildChatFromMessages(m.agent.Messages())
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: msg.message})
		_ = syncMessages(m.store, m.agent)
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	if m.confirm != nil {
		return m.handleConfirmKey(key)
	}

	switch m.activePage {
	case PagePermissions:
		return m.handlePermissionsKey(key)
	case PageSessions:
		return m.handleSessionsKey(key)
	default:
		return m.handleChatKey(msg, key)
	}
}

func (m Model) handleConfirmKey(key string) (Model, tea.Cmd) {
	if m.confirm == nil {
		return m, nil
	}
	if allow, deny := confirmKeyAllowed(key); allow {
		select {
		case m.confirm.RespCh <- true:
		default:
		}
		m.confirm = nil
	} else if deny {
		select {
		case m.confirm.RespCh <- false:
		default:
		}
		m.confirm = nil
	}
	return m, nil
}

func (m Model) handlePermissionsKey(key string) (Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.permissions.Cursor > 0 {
			m.permissions.Cursor--
		}
	case "down", "j":
		if m.permissions.Cursor < len(approval.Modes)-1 {
			m.permissions.Cursor++
		}
	case "enter":
		mode := approval.Modes[m.permissions.Cursor]
		var err error
		m, err = m.applyApprovalMode(mode)
		if err != nil {
			m.errMsg = err.Error()
		}
	case "esc":
		m.activePage = PageChat
	case "ctrl+c", "ctrl+d":
		return m.quit()
	}
	return m, nil
}

func (m Model) handleSessionsKey(key string) (Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.sessions.Cursor > 0 {
			m.sessions.Cursor--
		}
	case "down", "j":
		if len(m.sessions.Entries) > 0 && m.sessions.Cursor < len(m.sessions.Entries)-1 {
			m.sessions.Cursor++
		}
	case "enter":
		if len(m.sessions.Entries) == 0 {
			return m, nil
		}
		entry := m.sessions.Entries[m.sessions.Cursor]
		return m, m.resumeSession(entry.ID)
	case "esc":
		m.activePage = PageChat
	case "ctrl+c", "ctrl+d":
		return m.quit()
	}
	return m, nil
}

func (m Model) handleChatKey(msg tea.KeyMsg, key string) (Model, tea.Cmd) {
	if isShiftTab(msg) {
		mode := m.policy.CycleMode()
		m.agent.SetApprovalPolicy(m.policy)
		m.status.Approval = mode
		m.permissions.Cursor = approvalModeIndex(mode)
		return m, nil
	}

	if m.running {
		if key == "ctrl+c" {
			m.cancelAgentRun()
			m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "（已取消当前轮次）"})
			return m, nil
		}
		return m, nil
	}

	switch key {
	case "enter":
		return m.submitInput()
	case "ctrl+c", "ctrl+d":
		return m.quit()
	case "backspace":
		if len(m.input) > 0 {
			r := []rune(m.input)
			m.input = string(r[:len(r)-1])
		}
	case "esc":
		// 空闲时忽略 Esc
	default:
		if len(msg.Runes) > 0 {
			m.input += string(msg.Runes)
		}
	}
	return m, nil
}

func (m Model) submitInput() (Model, tea.Cmd) {
	raw := m.input
	m.input = ""
	if raw == "" {
		return m, nil
	}

	if slash := dispatchSlash(raw); slash.handled {
		return m.applySlash(slash)
	}

	m.lines = append(m.lines, ChatLine{Kind: LineUser, Text: raw})
	m.startAgentRun(raw)
	return m, nil
}

func (m Model) applySlash(r slashResult) (Model, tea.Cmd) {
	if r.quit {
		return m.quit()
	}
	if r.setMode != "" {
		var err error
		m, err = m.applyApprovalMode(r.setMode)
		if err != nil {
			m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: err.Error()})
		} else if r.message != "" {
			m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: r.message})
		}
		return m, nil
	}
	if r.openPage == PagePermissions {
		m.activePage = PagePermissions
		m.permissions.Cursor = approvalModeIndex(m.policy.Mode())
		return m, nil
	}
	if r.openPage == PageSessions {
		return m, m.openSessionsPage()
	}
	if r.compact {
		return m, m.runCompact(r.compactInstructions)
	}
	if r.message != "" {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: r.message})
	}
	return m, nil
}

func (m Model) applyApprovalMode(mode string) (Model, error) {
	if err := m.policy.SetMode(mode); err != nil {
		return m, err
	}
	m.agent.SetApprovalPolicy(m.policy)
	m.status.Approval = mode
	m.permissions.Cursor = approvalModeIndex(mode)
	return m, nil
}

func (m Model) openSessionsPage() tea.Cmd {
	store := m.store
	return func() tea.Msg {
		entries, err := store.ListSessions(0)
		if err != nil {
			return sessionsOpenMsg{err: fmt.Errorf("list sessions: %w", err)}
		}
		return sessionsOpenMsg{entries: entries}
	}
}

func (m Model) resumeSession(sessionID string) tea.Cmd {
	store := m.store
	agentRef := m.agent
	return func() tea.Msg {
		if err := syncMessages(store, agentRef); err != nil {
			return sessionResumeDataMsg{err: fmt.Errorf("save current session: %w", err)}
		}
		summary, msgs, err := store.LoadSession(sessionID)
		if err != nil {
			return sessionResumeDataMsg{err: fmt.Errorf("resume session: %w", err)}
		}
		return sessionResumeDataMsg{
			sessionID: sessionID,
			summary:   summary,
			messages:  msgs,
		}
	}
}

func (m Model) quit() (Model, tea.Cmd) {
	m.quitting = true
	m.agent.OnSessionEnd()
	return m, tea.Quit
}

func (m *Model) updateLastToolState(name string, state ToolState) {
	for i := len(m.lines) - 1; i >= 0; i-- {
		if m.lines[i].Kind == LineTool && m.lines[i].ToolName == name {
			m.lines[i].ToolState = state
			return
		}
	}
}

// View 实现 tea.Model。
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	return renderView(m)
}

// SetProgram 注入 Bubble Tea Program，供 Agent goroutine 回传消息。
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

func syncMessages(store *session.Store, ag *agent.Agent) error {
	if store == nil || ag == nil {
		return nil
	}
	return session.SyncFromSource(store, ag)
}

func (m *Model) runCompact(instructions string) tea.Cmd {
	ag := m.agent
	return func() tea.Msg {
		msg, err := ag.Compact(context.Background(), instructions)
		return compactDoneMsg{message: msg, err: err}
	}
}

// sessionPageEntries 将会话 Store 条目转为 pages 包视图结构。
func sessionPageEntries(entries []session.Entry) []pages.SessionEntry {
	out := make([]pages.SessionEntry, len(entries))
	for i, e := range entries {
		out[i] = pages.SessionEntry{
			ID:        e.ID,
			CreatedAt: e.CreatedAt,
			Preview:   e.Preview,
		}
	}
	return out
}
