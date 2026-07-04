package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tencent-docs/golem/internal/tui/pages"
)

var (
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)
	promptStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	userStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	asstStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	sysStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

// renderView 渲染完整 TUI 视图。
func renderView(m Model) string {
	width := m.width
	if width < 40 {
		width = 40
	}

	var body strings.Builder
	body.WriteString(renderStatusBar(m.status))
	body.WriteString("\n")

	switch m.activePage {
	case PagePermissions:
		body.WriteString(pages.Permissions(width, m.height, m.status.Approval, m.permissions.Cursor, m.rulesLines))
	case PageSessions:
		body.WriteString(pages.Sessions(width, sessionPageEntries(m.sessions.Entries), m.sessions.Cursor, m.agent.SessionID()))
	default:
		body.WriteString(renderChatArea(m, width))
	}

	if m.confirm != nil {
		body.WriteString("\n")
		body.WriteString(renderConfirmBox(m.confirm, width))
	}

	body.WriteString("\n")
	body.WriteString(promptStyle.Render("> "))
	body.WriteString(m.input)
	if m.running && m.activePage == PageChat {
		body.WriteString(" ")
		body.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("▌"))
	}

	if m.errMsg != "" && m.activePage == PageChat {
		body.WriteString("\n")
		body.WriteString(errStyle.Render(m.errMsg))
	}

	return body.String()
}

// renderStatusBar 渲染顶栏：project / approval / sandbox / session / tokens。
func renderStatusBar(s StatusBar) string {
	root := s.ProjectRoot
	if home := filepath.Base(root); home != "" && home != "." {
		if len(root) > 40 {
			root = "…" + root[len(root)-36:]
		}
	}
	line := fmt.Sprintf("📁 %s  🔒 %s  📦 %s  💬 %s  📊 %s",
		root,
		s.Approval,
		s.Sandbox,
		shortID(s.SessionID),
		formatTokens(s.InputTokens, s.ContextLimit),
	)
	if s.Model != "" {
		line += fmt.Sprintf("  🤖 %s", s.Model)
	}
	return statusBarStyle.Width(0).Render(line)
}

func renderChatArea(m Model, width int) string {
	var b strings.Builder
	visible := m.lines
	start := 0
	maxLines := m.height - 8
	if maxLines < 5 {
		maxLines = 5
	}
	if len(visible) > maxLines {
		start = len(visible) - maxLines
	}

	for _, line := range visible[start:] {
		switch line.Kind {
		case LineUser:
			b.WriteString(userStyle.Render("  You: "))
			b.WriteString(line.Text)
			b.WriteString("\n")
		case LineAssistant:
			b.WriteString(asstStyle.Render("  Claude: "))
			b.WriteString(line.Text)
			b.WriteString("\n")
		case LineSystem:
			b.WriteString(sysStyle.Render("  "))
			b.WriteString(line.Text)
			b.WriteString("\n")
		case LineTool:
			b.WriteString(formatToolCard(line, width))
			b.WriteString("\n")
		}
	}

	if m.streaming != "" {
		b.WriteString(asstStyle.Render("  Claude: "))
		b.WriteString(m.streaming)
		b.WriteString("▌\n")
	}
	return b.String()
}

func renderConfirmBox(c *ConfirmState, width int) string {
	line := ChatLine{
		Kind:      LineTool,
		ToolName:  c.ToolName,
		ToolInput: c.Input,
		ToolState: ToolConfirm,
	}
	return formatToolCard(line, width)
}
