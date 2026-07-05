package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tencent-docs/golem/internal/tui/pages"
	"github.com/tencent-docs/golem/internal/tui/style"
	"github.com/tencent-docs/golem/internal/skills"
)

// renderView 渲染完整 TUI 视图。
func renderView(m Model) string {
	width := m.width
	if width < 40 {
		width = 40
	}

	if m.activePage == PageWelcome {
		return renderWelcomePanel(m, width, m.height)
	}

	var body strings.Builder
	body.WriteString(renderStatusBar(m.status))
	body.WriteString("\n")
	body.WriteString(renderSeparator(width))
	body.WriteString("\n")

	switch m.activePage {
	case PagePermissions:
		body.WriteString(renderPermissionsPage(m, width))
	case PageSessions:
		body.WriteString(pages.Sessions(width, sessionPageEntries(m.sessions.Entries), m.sessions.Cursor, m.agent.SessionID()))
	case PageMemories:
		body.WriteString(pages.Memories(width, pages.MemoryFactsToView(m.memories.Facts), m.memories.InjectEnabled, m.memories.Cursor))
	case PageSkills:
		body.WriteString(pages.Skills(width, skillPageEntries(m.skillsPage.Skills), m.skillsPage.Cursor, skills.ScanPaths(m.projectRoot)))
	default:
		body.WriteString(renderChatArea(m, width))
	}

	if m.confirm != nil {
		body.WriteString("\n")
		body.WriteString(renderConfirmBox(m.confirm, width, m.projectRoot))
	}

	body.WriteString("\n")
	body.WriteString(renderSeparator(width))
	body.WriteString("\n")

	suggestions := matchSlashSuggestions(m.input, m.skillLoader)
	if len(suggestions) > 0 && m.activePage == PageChat && !m.running {
		body.WriteString(renderSlashSuggestions(suggestions, m.slashSel, width))
		body.WriteString("\n")
	}

	body.WriteString(renderInputLine(m.input, m.running, m.showCursor))

	if m.errMsg != "" && m.activePage == PageChat {
		body.WriteString("\n")
		body.WriteString(style.ErrText.Render(m.errMsg))
	}

	body.WriteString("\n")
	body.WriteString(renderFooter(m, len(suggestions) > 0))

	return body.String()
}

// renderInputLine 渲染输入区：提示符、用户输入文本与光标。
func renderInputLine(input string, running, showCursor bool) string {
	var b strings.Builder
	b.WriteString(style.Prompt.Render("❯ "))
	if input != "" {
		b.WriteString(style.UserText.Render(input))
	}
	if running || showCursor {
		b.WriteString(style.Cursor.Render("▌"))
	}
	return b.String()
}

// renderStatusBar 渲染顶栏：project / approval / sandbox / session / tokens。
func renderStatusBar(s StatusBar) string {
	root := s.ProjectRoot
	if len(root) > 40 {
		root = "…" + root[len(root)-36:]
	}
	sep := style.Muted.Render(" │ ")
	var parts []string
	parts = append(parts, style.Accent.Bold(true).Render("golem"))
	parts = append(parts, style.PathText.Render(root))
	parts = append(parts, style.Muted.Render(s.Approval))
	parts = append(parts, style.Muted.Render(s.Sandbox))
	parts = append(parts, style.AccentAlt.Render(shortID(s.SessionID)))
	parts = append(parts, style.Muted.Render(formatTokens(s.InputTokens, s.ContextLimit)))
	if s.Model != "" {
		parts = append(parts, style.Emphasis.Render(s.Model))
	}
	line := " " + strings.Join(parts, sep)
	return style.StatusBar.Width(0).Render(line)
}

// renderFooter 渲染底部快捷键提示栏。
func renderFooter(m Model, slashActive bool) string {
	if m.confirm != nil {
		hints := fmt.Sprintf("[Y/Enter] 允许  [n/Esc] 拒绝  — 工具: %s", m.confirm.ToolName)
		return style.Footer.Width(m.width).Render(hints)
	}
	var hints string
	switch m.activePage {
	case PagePermissions:
		hints = "[Tab] 切换页  [↑↓] 选择  [Enter] 确认  [Esc] 返回"
	case PageSessions:
		hints = "[↑↓] 选择会话  [Enter] 恢复  [Esc] 返回"
	case PageMemories:
		hints = "[↑↓] 浏览  [i] 切换注入  [c] 清空  [Esc] 返回"
	case PageSkills:
		hints = "[↑↓] 选择  [Enter] 激活  [Esc] 返回"
	default:
		if slashActive {
			hints = "[↑↓] 选择  [Tab] 补全  [Enter] 运行  [/] 命令"
		} else if m.running {
			hints = "[Ctrl+C] 取消  [Enter] 排队  [输入] 可继续编辑"
		} else {
			hints = "[Enter] 发送  [/] 命令  [Shift+Tab] approval  [Ctrl+L] 清屏  [?] /help"
		}
	}
	return style.Footer.Width(m.width).Render(hints)
}

func renderSeparator(width int) string {
	if width < 4 {
		width = 4
	}
	return style.Border.Render(strings.Repeat("─", width))
}

func renderChatArea(m Model, width int) string {
	if chatIsEmpty(m) {
		return renderChatHome(m, width)
	}

	var b strings.Builder
	visible := m.lines
	start := 0
	maxLines := m.height - 12
	if maxLines < 5 {
		maxLines = 5
	}
	if len(visible) > maxLines {
		start = len(visible) - maxLines
	}

	for _, line := range visible[start:] {
		switch line.Kind {
		case LineUser:
			b.WriteString(style.UserLabel.Render("  You "))
			b.WriteString(renderUserText(line.Text))
			b.WriteString("\n")
		case LineAssistant:
			b.WriteString(style.AsstLabel.Render("  Golem "))
			b.WriteString(renderRichText(line.Text, style.AsstText))
			b.WriteString("\n\n")
		case LineThinking:
			b.WriteString(renderThinkingBlock(line.Text, width, false))
			b.WriteString("\n")
		case LineSystem:
			b.WriteString(style.SysText.Render("  ◦ "))
			b.WriteString(style.SysText.Render(line.Text))
			b.WriteString("\n")
		case LineTool:
			b.WriteString(formatToolCard(line, width, m.projectRoot))
			b.WriteString("\n")
		}
	}

	if m.thinkingStreaming != "" {
		b.WriteString(renderThinkingBlock(m.thinkingStreaming, width, true))
		b.WriteString("\n")
	}

	if m.streaming != "" {
		b.WriteString(style.AsstLabel.Render("  Golem "))
		b.WriteString(renderRichText(m.streaming, style.AsstText))
		b.WriteString(style.Cursor.Render("▌"))
		b.WriteString("\n")
	}
	return b.String()
}

// renderChatHome 在聊天空态渲染 Claude Code 风格主页 dashboard。
func renderChatHome(m Model, width int) string {
	areaHeight := m.height - 12
	if areaHeight < 8 {
		areaHeight = 8
	}
	boxed := renderHomeDashboard(m, width, dashboardInnerWidth(width))
	boxH := lipgloss.Height(boxed)
	padTop := (areaHeight - boxH) / 2
	if padTop < 0 {
		padTop = 0
	}
	return strings.Repeat("\n", padTop) + boxed + "\n"
}

const (
	thinkingTitlePrefix = "  ┌─ Thinking "
	thinkingBoxMaxWidth = 72
)

// renderThinkingBlock 渲染思考过程区块，与最终答案视觉分离。
func renderThinkingBlock(text string, width int, streaming bool) string {
	if width < 20 {
		width = 20
	}
	boxWidth := width
	if boxWidth > thinkingBoxMaxWidth {
		boxWidth = thinkingBoxMaxWidth
	}
	inner := boxWidth - 6
	titleWidth := len([]rune(thinkingTitlePrefix))
	// 顶栏与底栏同宽：titleWidth + topDash + 1 = 3 + inner + 1
	topDash := max(0, inner-titleWidth+3)
	title := style.ThinkTitle.Render(thinkingTitlePrefix)
	border := style.Border.Render(strings.Repeat("─", topDash))
	var b strings.Builder
	b.WriteString(title)
	b.WriteString(border)
	b.WriteString("┐\n")

	for _, line := range strings.Split(text, "\n") {
		line = truncateRunes(line, inner)
		b.WriteString(style.ThinkBody.Render("  │ " + line))
		b.WriteString("\n")
	}
	if streaming {
		b.WriteString(style.ThinkBody.Render("  │ "))
		b.WriteString(style.Cursor.Render("▌"))
		b.WriteString("\n")
	}
	b.WriteString(style.Border.Render("  └" + strings.Repeat("─", inner) + "┘"))
	return b.String()
}

// renderSlashSuggestions 渲染斜杠命令补全下拉列表。
func renderSlashSuggestions(suggestions []SlashSuggestion, sel int, width int) string {
	if len(suggestions) == 0 {
		return ""
	}
	if sel < 0 {
		sel = 0
	}
	if sel >= len(suggestions) {
		sel = len(suggestions) - 1
	}
	descWidth := 28
	if width > 80 {
		descWidth = 36
	}
	start, end := slashSuggestionViewport(sel, len(suggestions), slashSuggestionMaxVisible)
	var b strings.Builder
	b.WriteString(style.Muted.Render("  命令与 Skill 补全"))
	b.WriteString("\n")
	if start > 0 {
		b.WriteString(style.SlashDesc.Render(fmt.Sprintf("  … 上方 %d 条", start)))
		b.WriteString("\n")
	}
	for i := start; i < end; i++ {
		cmd := suggestions[i]
		name := "/" + cmd.Name
		pad := 16 - len(cmd.Name)
		if pad < 1 {
			pad = 1
		}
		if i == sel {
			b.WriteString("  ")
			b.WriteString(style.SlashSel.Render(name + strings.Repeat(" ", pad) + truncateRunes(cmd.Desc, descWidth)))
		} else {
			b.WriteString("  ")
			b.WriteString(style.SlashItem.Render(name))
			b.WriteString(strings.Repeat(" ", pad))
			b.WriteString(style.SlashDesc.Render(truncateRunes(cmd.Desc, descWidth)))
		}
		b.WriteString("\n")
	}
	if end < len(suggestions) {
		b.WriteString(style.SlashDesc.Render(fmt.Sprintf("  … 下方 %d 条", len(suggestions)-end)))
		b.WriteString("\n")
	}
	return b.String()
}

func renderConfirmBox(c *ConfirmState, width int, projectRoot string) string {
	line := ChatLine{
		Kind:      LineTool,
		ToolName:  c.ToolName,
		ToolInput: c.Input,
		ToolState: ToolConfirm,
	}
	return formatToolCard(line, width, projectRoot)
}
