package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/skills"
	"github.com/tencent-docs/golem/internal/tui/pages"
)

// Model 是 Bubble Tea 主状态机，承载聊天、子页面与 Agent 桥接。
type Model struct {
	agent   *agent.Agent
	store   *session.Store
	policy  *approval.Policy
	program *tea.Program

	lines              []ChatLine
	input              string
	streaming          string
	thinkingStreaming  string
	streamStarted      bool
	confirm            *ConfirmState
	slashSel           int

	status StatusBar

	projectRoot string
	version     string
	activePage  PageKind
	permissions PermissionsPage
	sessions    SessionsPage
	memories    MemoriesPage
	skillsPage  SkillsPage
	rulesLines  []string

	skillLoader *skills.Loader
	llmClient   llm.LLMClient
	inputQueue  []string
	lastEscAt   time.Time

	running    bool
	runCancel  context.CancelFunc
	width      int
	height     int
	errMsg     string
	quitting   bool
	showCursor bool

	needsSetup          bool
	setupStep           int
	setupDefaultBaseURL string
	setupDefaultModel   string
	setupBaseURL        string
	setupAPIKey         string
	setupModel          string
	setupErrMsg         string
}

// cursorBlinkMsg 驱动输入区光标闪烁。
type cursorBlinkMsg struct{}

// Config 启动 TUI 所需的依赖与展示参数。
type Config struct {
	ProjectRoot      string
	Version          string
	Agent            *agent.Agent
	Store            *session.Store
	Policy           *approval.Policy
	Sandbox          string
	ModelName        string
	ContextLimit     int
	RulesLines       []string
	SkillLoader      *skills.Loader
	LLMClient        llm.LLMClient
	NeedsSetup       bool
	DefaultBaseURL   string
	DefaultModel     string
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
		agent:               cfg.Agent,
		store:               cfg.Store,
		policy:              cfg.Policy,
		status:              status,
		projectRoot:         cfg.ProjectRoot,
		version:             cfg.Version,
		activePage:          PageWelcome,
		rulesLines:          cfg.RulesLines,
		skillLoader:         cfg.SkillLoader,
		llmClient:           cfg.LLMClient,
		permissions:         PermissionsPage{Tab: PermTabModes, Cursor: approvalModeIndex(cfg.Policy.Mode())},
		width:               80,
		height:              24,
		showCursor:          true,
		needsSetup:          cfg.NeedsSetup,
		setupDefaultBaseURL: cfg.DefaultBaseURL,
		setupDefaultModel:   cfg.DefaultModel,
	}
	if cfg.NeedsSetup && len(cfg.Agent.Messages()) > 0 {
		m.activePage = PageSetup
	} else if len(cfg.Agent.Messages()) > 0 {
		m.lines = rebuildChatFromMessages(cfg.Agent.Messages())
		m.activePage = PageChat
	}
	return m
}

// Init 实现 tea.Model。
func (m Model) Init() tea.Cmd {
	return blinkCursor()
}

// Update 实现 tea.Model。
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case cursorBlinkMsg:
		m.showCursor = !m.showCursor
		return m, blinkCursor()

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.FocusMsg:
		m.showCursor = true
		return m, blinkCursor()

	case tea.ResumeMsg:
		m.showCursor = true
		return m, blinkCursor()

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
		m.upsertToolConfirmLine(msg.toolName, msg.input)
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
		return m.handleCompactDone(msg)
	case reviewDoneMsg:
		return m.handleReviewDone(msg)
	case initDoneMsg:
		return m.handleInitDone(msg)
	case forkDoneMsg:
		return m.handleForkDone(msg)
	case exportDoneMsg:
		return m.handleExportDone(msg)
	case renameDoneMsg:
		return m.handleRenameDone(msg)
	case layer2DoneMsg:
		return m.handleLayer2Done(msg)
	case diffDoneMsg:
		return m.handleDiffDone(msg)
	case deniedOpenMsg:
		return m.handleDeniedOpen(msg)
	case memoriesOpenMsg:
		return m.handleMemoriesOpen(msg)
	case skillsOpenMsg:
		return m.handleSkillsOpen(msg)
	case editorDoneMsg:
		return m.handleEditorDone(msg)
	case retryDoneMsg:
		return m.handleRetryDone(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	if m.confirm != nil {
		return m.handleConfirmKey(key)
	}

	switch m.activePage {
	case PageWelcome:
		return m.handleWelcomeKey(key)
	case PageSetup:
		return m.handleSetupKey(msg, key)
	case PagePermissions:
		return m.handlePermissionsKey(key)
	case PageSessions:
		return m.handleSessionsKey(key)
	case PageMemories:
		return m.handleMemoriesKey(key)
	case PageSkills:
		return m.handleSkillsKey(key)
	default:
		return m.handleChatKey(msg, key)
	}
}

func (m Model) handleConfirmKey(key string) (Model, tea.Cmd) {
	if m.confirm == nil {
		return m, nil
	}
	allow, deny := confirmKeyAllowed(key)
	if !allow && !deny {
		return m, nil
	}
	resp := m.confirm.RespCh
	m.confirm = nil
	resp <- allow
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

func (m Model) handleWelcomeKey(key string) (Model, tea.Cmd) {
	switch key {
	case "enter", " ":
		if m.needsSetup {
			m.activePage = PageSetup
			m.setupStep = setupStepBaseURL
			m.input = ""
			m.setupErrMsg = ""
		} else {
			m.activePage = PageChat
		}
	case "q", "ctrl+c", "ctrl+d":
		return m.quit()
	}
	return m, nil
}

func (m Model) handleChatKey(msg tea.KeyMsg, key string) (Model, tea.Cmd) {
	suggestions := matchSlashSuggestions(m.input, m.skillLoader)
	slashActive := len(suggestions) > 0

	if slashActive && key == "tab" {
		m.input = completeSlashInput(m.input, m.slashSel, suggestions)
		m.slashSel = 0
		return m, nil
	}

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
		if key == "enter" {
			if strings.TrimSpace(m.input) != "" {
				m.inputQueue = append(m.inputQueue, m.input)
				m.input = ""
				m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "（已排队下一条输入）"})
			}
			return m, nil
		}
		if key == "tab" && strings.TrimSpace(m.input) != "" {
			m.inputQueue = append(m.inputQueue, m.input)
			m.input = ""
			m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "（已排队下一条输入）"})
			return m, nil
		}
		// Agent 流式/等待中仍允许编辑输入框，避免切回终端后误以为卡住。
	}

	if key == "ctrl+l" {
		m.lines = nil
		m.streaming = ""
		m.thinkingStreaming = ""
		return m, nil
	}
	if key == "ctrl+g" {
		return m, m.openExternalEditor()
	}

	switch key {
	case "up", "k":
		if slashActive && m.slashSel > 0 {
			m.slashSel--
			m.showCursor = true
			return m, nil
		}
	case "down", "j":
		if slashActive && m.slashSel < len(suggestions)-1 {
			m.slashSel++
			m.showCursor = true
			return m, nil
		}
	case "enter":
		return m.submitInput()
	case "ctrl+c", "ctrl+d":
		return m.quit()
	case "backspace":
		if len(m.input) > 0 {
			r := []rune(m.input)
			m.input = string(r[:len(r)-1])
			m.slashSel = 0
		}
	case "esc":
		if strings.TrimSpace(m.input) == "" {
			if time.Since(m.lastEscAt) < 500*time.Millisecond {
				if prev := m.lastUserMessage(); prev != "" {
					m.input = prev
				}
				m.lastEscAt = time.Time{}
				return m, nil
			}
			m.lastEscAt = time.Now()
		}
	default:
		if len(msg.Runes) > 0 {
			m.input += string(msg.Runes)
			m.slashSel = 0
			m.showCursor = true
		}
	}
	return m, nil
}

// blinkCursor 返回周期性光标闪烁 tick。
func blinkCursor() tea.Cmd {
	return tea.Tick(530*time.Millisecond, func(time.Time) tea.Msg {
		return cursorBlinkMsg{}
	})
}

func (m Model) submitInput() (Model, tea.Cmd) {
	raw := m.input
	suggestions := matchSlashSuggestions(raw, m.skillLoader)
	if len(suggestions) > 0 {
		raw = resolveSlashInput(raw, m.slashSel, suggestions)
	}
	m.input = ""
	m.slashSel = 0
	if raw == "" {
		return m, nil
	}

	m.lines = append(m.lines, ChatLine{Kind: LineUser, Text: raw})
	if m.activePage == PageWelcome {
		m.activePage = PageChat
	}

	if slash := dispatchSlash(raw, m.skillLoader); slash.handled {
		return m.applySlash(raw, slash)
	}

	m.startAgentRun(raw)
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
