package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tencent-docs/golem/internal/tui/style"
)

// layoutSections 计算顶栏、底栏高度及中间区域可用行数。
func layoutSections(m Model, width int) (header, footer string, middleHeight int) {
	header = renderStatusBar(m.status) + "\n" + renderSeparator(width)

	var footerParts []string
	if m.confirm != nil {
		footerParts = append(footerParts, renderConfirmBox(m.confirm, width, m.projectRoot))
	}
	footerParts = append(footerParts, renderSeparator(width))

	suggestions := matchSlashSuggestions(m.input, m.skillLoader)
	slashActive := len(suggestions) > 0 && m.activePage == PageChat && !m.running
	if slashActive {
		footerParts = append(footerParts, renderSlashSuggestions(suggestions, m.slashSel, width))
	}
	footerParts = append(footerParts, renderInputLine(m.input, m.running, m.showCursor))
	if m.errMsg != "" && m.activePage == PageChat {
		footerParts = append(footerParts, styleErrLine(m.errMsg))
	}
	footerParts = append(footerParts, renderFooter(m, slashActive))
	footer = strings.Join(footerParts, "\n")

	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	middleHeight = m.height - headerH - footerH
	if middleHeight < 1 {
		middleHeight = 1
	}
	return header, footer, middleHeight
}

func styleErrLine(msg string) string {
	return style.ErrText.Render(msg)
}

// viewportStart 根据滚动状态计算中间区域起始行。
func (m Model) viewportStart(totalLines, viewportHeight int) int {
	if viewportHeight <= 0 {
		return 0
	}
	maxStart := totalLines - viewportHeight
	if maxStart < 0 {
		maxStart = 0
	}
	if m.chatPinnedBottom {
		return maxStart
	}
	start := m.chatScrollTop
	if start > maxStart {
		start = maxStart
	}
	if start < 0 {
		start = 0
	}
	return start
}

// clipToViewport 将内容裁剪到指定高度，并从 start 行开始显示。
func clipToViewport(content string, viewportHeight, start int) string {
	if viewportHeight <= 0 {
		viewportHeight = 1
	}
	lines := strings.Split(content, "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	maxStart := len(lines) - viewportHeight
	if maxStart < 0 {
		maxStart = 0
	}
	if start > maxStart {
		start = maxStart
	}
	if start < 0 {
		start = 0
	}
	end := start + viewportHeight
	if end > len(lines) {
		end = len(lines)
	}
	var visible []string
	if start < len(lines) {
		visible = lines[start:end]
	}
	for len(visible) < viewportHeight {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n")
}

func (m *Model) scrollChatUp(lines int) {
	if lines < 1 {
		lines = 1
	}
	m.chatPinnedBottom = false
	m.chatScrollTop -= lines
	if m.chatScrollTop < 0 {
		m.chatScrollTop = 0
	}
}

func (m *Model) scrollChatDown(lines int, width int) {
	if lines < 1 {
		lines = 1
	}
	_, _, midH := layoutSections(*m, width)
	content := renderMiddleContent(*m, width, midH)
	totalLines := lipgloss.Height(content)
	maxStart := totalLines - midH
	if maxStart < 0 {
		maxStart = 0
	}
	m.chatScrollTop += lines
	if m.chatScrollTop >= maxStart {
		m.chatScrollTop = maxStart
		m.chatPinnedBottom = true
	}
}

func (m *Model) pinChatBottom() {
	m.chatPinnedBottom = true
	m.chatScrollTop = 0
}

func (m Model) canScrollChat() bool {
	return m.activePage == PageChat && !chatIsEmpty(m) &&
		strings.TrimSpace(m.input) == "" &&
		len(matchSlashSuggestions(m.input, m.skillLoader)) == 0
}
