package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/memory"
	"github.com/tencent-docs/golem/internal/sandbox"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/skills"
	"github.com/tencent-docs/golem/internal/tui/pages"
)

type reviewDoneMsg struct {
	text string
	err  error
}

type initDoneMsg struct {
	err error
}

type forkDoneMsg struct {
	newID string
	err   error
}

type exportDoneMsg struct {
	path string
	err  error
}

type renameDoneMsg struct {
	name string
	err  error
}

type layer2DoneMsg struct {
	err error
}

type diffDoneMsg struct {
	text string
	err  error
}

type deniedOpenMsg struct {
	entries []session.DenialEntry
	err     error
}

type memoriesOpenMsg struct {
	facts         []memory.MemoryFact
	injectEnabled bool
	err           error
}

type skillsOpenMsg struct {
	skills []skills.Skill
	err    error
}

type editorDoneMsg struct {
	text string
	err  error
}

type retryDoneMsg struct {
	text string
	err  error
}

func (m Model) applySlash(raw string, r slashResult) (Model, tea.Cmd) {
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
	if r.setSandbox != "" {
		mode := sandbox.ParseMode(r.setSandbox)
		m.agent.SetSandboxMode(mode)
		m.status.Sandbox = r.setSandbox
		if r.message != "" {
			m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: r.message})
		}
		return m, nil
	}
	if r.setModel != "" {
		if err := m.agent.SetModel(r.setModel); err != nil {
			m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: err.Error()})
		} else {
			m.status.Model = r.setModel
			m.errMsg = ""
			m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "model 已设为 " + r.setModel})
		}
		return m, nil
	}
	if r.openPage == PagePermissions {
		m.activePage = PagePermissions
		m.permissions.Tab = PermTabModes
		m.permissions.Cursor = approvalModeIndex(m.policy.Mode())
		return m, m.loadDenials()
	}
	if r.openPage == PageSessions {
		return m, m.openSessionsPage()
	}
	if r.openPage == PageMemories {
		return m, m.openMemoriesPage()
	}
	if r.openPage == PageSkills {
		return m, m.openSkillsPage()
	}
	if r.runSkill != "" {
		return m, m.startSkillRun(r.runSkill, r.skillQuery)
	}
	if r.compact {
		return m, m.runCompact(r.compactInstructions)
	}
	if r.clearContext {
		newID := m.agent.ClearContext()
		userLine := ChatLine{Kind: LineUser, Text: raw}
		m.lines = []ChatLine{userLine}
		m.streaming = ""
		m.syncStatus()
		m.status.SessionID = shortID(newID)
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "已清空上下文，新 session " + shortID(newID)})
		return m, nil
	}
	if r.showUsage {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: m.agent.FormatUsageSummary()})
		return m, nil
	}
	if r.fork {
		return m, m.runFork()
	}
	if r.runReview {
		return m, m.runReview(r.reviewTarget)
	}
	if r.runInit {
		return m, m.runInit(r.initWrite)
	}
	if r.runPlan != "" {
		return m, m.startPlanRun(r.runPlan)
	}
	if r.runAgent == "__diff__" {
		return m, m.runDiff()
	}
	if r.doExport {
		return m, m.runExport(r.exportPath)
	}
	if r.renameName != "" {
		return m, m.runRename(r.renameName)
	}
	if r.message == "__status__" || r.message == "__context__" || r.message == "__model__" || r.message == "__sandbox_cycle__" {
		return m.handleMagicMessage(r)
	}
	if r.message != "" {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: r.message})
	}
	return m, nil
}

func (m Model) handleMagicMessage(r slashResult) (Model, tea.Cmd) {
	switch r.message {
	case "__status__":
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: m.agent.FormatStatusSummary(m.status.Sandbox, m.status.Model)})
	case "__context__":
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: m.agent.FormatContextBreakdown()})
	case "__model__":
		cur := m.status.Model
		if cur == "" {
			cur = m.agent.ModelName()
		}
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "当前 model: " + cur})
	case "__sandbox_cycle__":
		next := cycleSandboxMode(m.status.Sandbox)
		mode := sandbox.ParseMode(next)
		m.agent.SetSandboxMode(mode)
		m.status.Sandbox = next
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "sandbox 已切换为 " + next})
	}
	return m, nil
}

func (m Model) handlePermissionsKey(key string) (Model, tea.Cmd) {
	switch key {
	case "tab":
		m.permissions.Tab = (m.permissions.Tab + 1) % 3
		if m.permissions.Tab == PermTabDenied && len(m.permissions.Denied) == 0 {
			return m, m.loadDenials()
		}
	case "up", "k":
		if m.permissions.Cursor > 0 {
			m.permissions.Cursor--
		}
	case "down", "j":
		if m.permissions.Cursor < m.permissionsMaxCursor() {
			m.permissions.Cursor++
		}
	case "enter":
		if m.permissions.Tab == PermTabModes {
			mode := approval.Modes[m.permissions.Cursor]
			var err error
			m, err = m.applyApprovalMode(mode)
			if err != nil {
				m.errMsg = err.Error()
			}
		}
	case "r":
		if m.permissions.Tab == PermTabDenied && len(m.permissions.Denied) > 0 {
			entry := m.permissions.Denied[m.permissions.Cursor]
			return m, m.retryDenied(entry)
		}
	case "esc":
		m.activePage = PageChat
	case "ctrl+c", "ctrl+d":
		return m.quit()
	}
	return m, nil
}

func (m Model) permissionsMaxCursor() int {
	switch m.permissions.Tab {
	case PermTabModes:
		return len(approval.Modes) - 1
	case PermTabDenied:
		if len(m.permissions.Denied) == 0 {
			return 0
		}
		return len(m.permissions.Denied) - 1
	default:
		if len(m.rulesLines) == 0 {
			return 0
		}
		return len(m.rulesLines) - 1
	}
}

func (m Model) handleMemoriesKey(key string) (Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.memories.Cursor > 0 {
			m.memories.Cursor--
		}
	case "down", "j":
		if len(m.memories.Facts) > 0 && m.memories.Cursor < len(m.memories.Facts)-1 {
			m.memories.Cursor++
		}
	case "i":
		return m, m.toggleMemoryInject()
	case "c":
		return m, m.clearMemoryFacts()
	case "l":
		return m, m.runLayer2Manual()
	case "esc":
		m.activePage = PageChat
	case "ctrl+c", "ctrl+d":
		return m.quit()
	}
	return m, nil
}

func (m Model) handleSkillsKey(key string) (Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.skillsPage.Cursor > 0 {
			m.skillsPage.Cursor--
		}
	case "down", "j":
		if len(m.skillsPage.Skills) > 0 && m.skillsPage.Cursor < len(m.skillsPage.Skills)-1 {
			m.skillsPage.Cursor++
		}
	case "enter":
		if len(m.skillsPage.Skills) == 0 {
			m.activePage = PageChat
			return m, nil
		}
		skill := m.skillsPage.Skills[m.skillsPage.Cursor]
		m.input = "/" + skill.Name + " "
		m.activePage = PageChat
		m.showCursor = true
	case "esc":
		m.activePage = PageChat
	case "ctrl+c", "ctrl+d":
		return m.quit()
	}
	return m, nil
}

func (m Model) handleCompactDone(msg compactDoneMsg) (Model, tea.Cmd) {
	m.syncStatus()
	if msg.err != nil {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "Compact 失败: " + msg.err.Error()})
		return m, nil
	}
	m.lines = rebuildChatFromMessages(m.agent.Messages())
	m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: msg.message})
	_ = syncMessages(m.store, m.agent)
	return m, nil
}

func (m Model) handleReviewDone(msg reviewDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "Review 失败: " + msg.err.Error()})
		return m, nil
	}
	m.lines = append(m.lines, ChatLine{Kind: LineAssistant, Text: msg.text})
	return m, nil
}

func (m Model) handleInitDone(msg initDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "Init 失败: " + msg.err.Error()})
		return m, nil
	}
	m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "已生成 AGENTS.md"})
	return m, nil
}

func (m Model) handleForkDone(msg forkDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "Fork 失败: " + msg.err.Error()})
		return m, nil
	}
	m.agent.SetSessionID(msg.newID)
	m.syncStatus()
	m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "已分叉到新 session " + shortID(msg.newID)})
	return m, nil
}

func (m Model) handleExportDone(msg exportDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "Export 失败: " + msg.err.Error()})
		return m, nil
	}
	m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "已导出到 " + msg.path})
	return m, nil
}

func (m Model) handleRenameDone(msg renameDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "Rename 失败: " + msg.err.Error()})
		return m, nil
	}
	m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "会话已重命名为 " + msg.name})
	return m, nil
}

func (m Model) handleLayer2Done(msg layer2DoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "Layer 2 失败: " + msg.err.Error()})
		return m, nil
	}
	m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "Layer 2 合并完成，memory_facts 已清空"})
	return m, m.openMemoriesPage()
}

func (m Model) handleDiffDone(msg diffDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "Diff 失败: " + msg.err.Error()})
		return m, nil
	}
	m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: msg.text})
	return m, nil
}

func (m Model) handleDeniedOpen(msg deniedOpenMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m, nil
	}
	m.permissions.Denied = msg.entries
	m.permissions.Cursor = 0
	return m, nil
}

func (m Model) handleMemoriesOpen(msg memoriesOpenMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m, nil
	}
	m.activePage = PageMemories
	m.memories.Facts = msg.facts
	m.memories.InjectEnabled = msg.injectEnabled
	m.memories.Cursor = 0
	return m, nil
}

func (m Model) handleSkillsOpen(msg skillsOpenMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m, nil
	}
	m.activePage = PageSkills
	m.skillsPage.Skills = msg.skills
	m.skillsPage.Cursor = 0
	return m, nil
}

func (m Model) handleEditorDone(msg editorDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: msg.err.Error()})
		return m, nil
	}
	if msg.text == "" {
		return m, nil
	}
	m.input = msg.text
	return m, nil
}

func (m Model) handleRetryDone(msg retryDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "重试失败: " + msg.err.Error()})
		return m, nil
	}
	m.lines = append(m.lines, ChatLine{Kind: LineSystem, Text: "重试成功: " + truncateRunes(msg.text, 200)})
	return m, nil
}

func (m Model) lastUserMessage() string {
	for i := len(m.lines) - 1; i >= 0; i-- {
		if m.lines[i].Kind == LineUser {
			return m.lines[i].Text
		}
	}
	return ""
}

func (m Model) loadDenials() tea.Cmd {
	store := m.store
	return func() tea.Msg {
		entries, err := store.ListDenials(20)
		if err != nil {
			return deniedOpenMsg{err: err}
		}
		return deniedOpenMsg{entries: entries}
	}
}

func (m Model) openMemoriesPage() tea.Cmd {
	store := m.store
	return func() tea.Msg {
		facts, err := store.ListMemoryFacts()
		if err != nil {
			return memoriesOpenMsg{err: err}
		}
		enabled, err := store.MemoryInjectEnabled()
		if err != nil {
			return memoriesOpenMsg{err: err}
		}
		return memoriesOpenMsg{facts: facts, injectEnabled: enabled}
	}
}

func (m Model) openSkillsPage() tea.Cmd {
	loader := m.skillLoader
	return func() tea.Msg {
		if loader == nil {
			return skillsOpenMsg{skills: nil}
		}
		list, err := loader.List()
		if err != nil {
			return skillsOpenMsg{err: err}
		}
		return skillsOpenMsg{skills: list}
	}
}

func (m Model) runReview(target string) tea.Cmd {
	ag := m.agent
	return func() tea.Msg {
		text, err := ag.RunReview(context.Background(), target)
		return reviewDoneMsg{text: text, err: err}
	}
}

func (m Model) runInit(write bool) tea.Cmd {
	ag := m.agent
	return func() tea.Msg {
		_, err := ag.RunInit(context.Background(), write)
		return initDoneMsg{err: err}
	}
}

func (m Model) runFork() tea.Cmd {
	store := m.store
	ag := m.agent
	return func() tea.Msg {
		_ = syncMessages(store, ag)
		newID, err := store.ForkSession(ag.SessionID())
		return forkDoneMsg{newID: newID, err: err}
	}
}

func (m Model) runExport(path string) tea.Cmd {
	ag := m.agent
	projectRoot := m.projectRoot
	return func() tea.Msg {
		text := ag.ExportSessionMarkdown()
		if strings.TrimSpace(path) == "" {
			path = "session-" + shortID(ag.SessionID()) + ".md"
		}
		full := path
		if !filepath.IsAbs(path) {
			full = filepath.Join(projectRoot, path)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			return exportDoneMsg{err: err}
		}
		return exportDoneMsg{path: full}
	}
}

func (m Model) runRename(name string) tea.Cmd {
	store := m.store
	sessionID := m.agent.SessionID()
	return func() tea.Msg {
		if strings.TrimSpace(name) == "" {
			return renameDoneMsg{err: fmt.Errorf("name is required")}
		}
		err := store.RenameSession(sessionID, name)
		return renameDoneMsg{name: name, err: err}
	}
}

func (m Model) runDiff() tea.Cmd {
	ag := m.agent
	return func() tea.Msg {
		text, err := ag.RetryDeniedTool(context.Background(), "bash", `{"command":"git diff HEAD 2>/dev/null; git diff --cached 2>/dev/null; git status --porcelain 2>/dev/null"}`, nil)
		return diffDoneMsg{text: text, err: err}
	}
}

func (m Model) toggleMemoryInject() tea.Cmd {
	store := m.store
	enabled := m.memories.InjectEnabled
	return func() tea.Msg {
		if err := store.SetMemoryInjectEnabled(!enabled); err != nil {
			return memoriesOpenMsg{err: err}
		}
		facts, err := store.ListMemoryFacts()
		if err != nil {
			return memoriesOpenMsg{err: err}
		}
		newEnabled, err := store.MemoryInjectEnabled()
		if err != nil {
			return memoriesOpenMsg{err: err}
		}
		return memoriesOpenMsg{facts: facts, injectEnabled: newEnabled}
	}
}

func (m Model) clearMemoryFacts() tea.Cmd {
	store := m.store
	return func() tea.Msg {
		if err := store.DeleteAllFacts(); err != nil {
			return memoriesOpenMsg{err: err}
		}
		enabled, err := store.MemoryInjectEnabled()
		if err != nil {
			return memoriesOpenMsg{err: err}
		}
		return memoriesOpenMsg{facts: nil, injectEnabled: enabled}
	}
}

func (m Model) runLayer2Manual() tea.Cmd {
	store := m.store
	projectRoot := m.projectRoot
	llmGetter := m.llmClient
	return func() tea.Msg {
		if llmGetter == nil {
			return layer2DoneMsg{err: fmt.Errorf("llm client unavailable")}
		}
		err := memory.RunLayer2(context.Background(), store.ProjectIDValue(), projectRoot, store, llmGetter)
		return layer2DoneMsg{err: err}
	}
}

func (m Model) openExternalEditor() tea.Cmd {
	path, err := writeEditorTemp(m.input)
	if err != nil {
		return func() tea.Msg {
			return editorDoneMsg{err: err}
		}
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	return tea.ExecProcess(exec.Command(editor, path), func(runErr error) tea.Msg {
		defer os.Remove(path)
		if runErr != nil {
			return editorDoneMsg{err: runErr}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return editorDoneMsg{err: err}
		}
		return editorDoneMsg{text: strings.TrimSpace(string(data))}
	})
}

// writeEditorTemp 将初始内容写入临时文件，供外部编辑器打开。
func writeEditorTemp(initial string) (string, error) {
	tmp, err := os.CreateTemp("", "golem-edit-*.md")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	if _, err := tmp.WriteString(initial); err != nil {
		tmp.Close()
		os.Remove(path)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

func (m Model) retryDenied(entry session.DenialEntry) tea.Cmd {
	ag := m.agent
	return func() tea.Msg {
		text, err := ag.RetryDeniedTool(context.Background(), entry.Tool, entry.Input, nil)
		return retryDoneMsg{text: text, err: err}
	}
}

func (m *Model) startPlanRun(query string) tea.Cmd {
	m.startAgentPlan(query)
	return nil
}

func (m *Model) startSkillRun(skillName, query string) tea.Cmd {
	m.startAgentSkill(skillName, query)
	return nil
}

func (m *Model) drainInputQueue() {
	if len(m.inputQueue) == 0 || m.running {
		return
	}
	next := m.inputQueue[0]
	m.inputQueue = m.inputQueue[1:]
	m.lines = append(m.lines, ChatLine{Kind: LineUser, Text: next})
	m.startAgentRun(next)
}

func skillPageEntries(list []skills.Skill) []pages.SkillEntry {
	out := make([]pages.SkillEntry, len(list))
	for i, s := range list {
		out[i] = pages.SkillEntry{
			Name:   s.Name,
			Source: s.Source,
		}
	}
	return out
}

func deniedPageEntries(entries []session.DenialEntry) []pages.DeniedEntry {
	out := make([]pages.DeniedEntry, len(entries))
	for i, e := range entries {
		out[i] = pages.DeniedEntry{
			Tool:      e.Tool,
			Input:     e.Input,
			Reason:    e.Reason,
			CreatedAt: e.CreatedAt,
		}
	}
	return out
}
