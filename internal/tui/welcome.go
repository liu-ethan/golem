package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tencent-docs/golem/internal/tui/style"
)

// welcomeTips 主页空态右侧「快速开始」提示。
var welcomeTips = []string{
	"Run /init to create an AGENTS.md file with instructions for Golem",
	"Type a question below to start chatting",
	"/help lists all slash commands",
	"Shift+Tab cycles approval mode",
	"/permissions manages rules and sandbox",
}

// welcomeNews 主页空态「What's new」条目（随版本更新）。
var welcomeNews = []string{
	"Warm theme aligned with Claude Code CLI",
	"Semantic colors for paths, code blocks, user vs assistant",
	"Blinking input cursor shows edit position",
	"Thinking stream visually separated from final answer",
}

// chatIsEmpty 判断聊天区是否处于空态（可展示主页 dashboard）。
func chatIsEmpty(m Model) bool {
	return len(m.lines) == 0 && m.streaming == "" && m.thinkingStreaming == ""
}

// renderWelcomePanel 渲染启动闪屏：居中 dashboard + Enter 提示。
func renderWelcomePanel(m Model, width, height int) string {
	boxed := renderHomeDashboard(m, width, dashboardInnerWidth(width))
	footer := style.Accent.
		Bold(true).
		Render("\n  [Enter] 开始对话  ·  [q] 退出")

	padTop := (height - lipgloss.Height(boxed+footer) - 2) / 2
	if padTop < 0 {
		padTop = 0
	}
	return strings.Repeat("\n", padTop) + boxed + footer
}

// renderHomeDashboard 渲染 Claude Code 风格双栏主页（欢迎闪屏与聊天空态共用）。
func renderHomeDashboard(m Model, width, innerWidth int) string {
	if width < 40 {
		width = 40
	}
	if innerWidth < 36 {
		innerWidth = width - 6
	}

	version := m.version
	if version == "" {
		version = "dev"
	}
	titleBar := fmt.Sprintf(" Golem %s ", version)

	left := renderHomeLeft(m)

	var inner string
	if innerWidth >= 64 {
		rightW := innerWidth - innerWidth/2 - 3
		right := renderHomeRight(rightW)
		inner = joinTwoColumns(left, right, innerWidth)
	} else {
		inner = left + "\n\n" + renderHomeRight(innerWidth)
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(style.ColorAccent)).
		Width(innerWidth).
		Align(lipgloss.Center)

	content := titleStyle.Render(titleBar) + "\n\n" + inner

	return style.WelcomeBorder.
		Padding(1, 2).
		Width(width - 2).
		Render(content)
}

func dashboardInnerWidth(width int) int {
	w := width - 6
	if w < 36 {
		return 36
	}
	return w
}

// renderHomeLeft 渲染主页左栏：问候、Logo、模型与项目信息。
func renderHomeLeft(m Model) string {
	logo := style.Accent.Render("    ▐▛███▜▌") + "\n" +
		style.Accent.Render("   ▝▜█████▛▘") + "\n" +
		style.Muted.Render("     ▘▘ ▝▝")

	modelLine := emptyFallback(m.status.Model, "—")
	sandbox := emptyFallback(m.status.Sandbox, "workspace-write")
	meta := fmt.Sprintf("%s · %s",
		style.AccentAlt.Render(modelLine),
		style.Muted.Render(sandbox),
	)
	project := style.PathText.Render(shortenPath(m.status.ProjectRoot, 48))

	rows := []string{
		"",
		style.Emphasis.Render("  Welcome back!"),
		"",
		logo,
		"",
		"  " + meta,
		"  " + project,
	}
	return strings.Join(rows, "\n")
}

// renderHomeRight 渲染主页右栏：Tips 与 What's new。
func renderHomeRight(colWidth int) string {
	if colWidth < 28 {
		colWidth = 28
	}
	divW := colWidth
	if divW > 40 {
		divW = 40
	}
	divider := style.Border.Render(strings.Repeat("─", divW))

	var b strings.Builder
	b.WriteString(style.Emphasis.Render("Tips for getting started"))
	b.WriteString("\n")
	for _, tip := range welcomeTips {
		b.WriteString(style.Muted.Render("  " + truncateRunes(tip, colWidth-2)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(divider)
	b.WriteString("\n\n")
	b.WriteString(style.Emphasis.Render("What's new"))
	b.WriteString("\n")
	for _, item := range welcomeNews {
		b.WriteString(style.Muted.Render("  " + truncateRunes(item, colWidth-2)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(style.Accent.Render("  /help for commands"))
	return b.String()
}

// panelRow 格式化面板中的键值行（供测试复用）。
func panelRow(label, value string, width int) string {
	return "  " + style.Muted.Width(12).Render(label) + style.Emphasis.Render(truncateRunes(value, width-14))
}

// shortenPath 截断过长路径以便在面板中展示。
func shortenPath(path string, max int) string {
	if path == "" {
		return "."
	}
	if len(path) <= max {
		return path
	}
	base := filepath.Base(path)
	if len(base)+4 <= max {
		return "…/" + base
	}
	return "…" + truncateRunes(path, max-1)
}

func emptyFallback(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// joinTwoColumns 按行对齐双栏内容，中间以 │ 分隔。
func joinTwoColumns(left, right string, totalInner int) string {
	leftW := totalInner / 2
	sep := " " + style.Border.Render("│") + " "
	sepW := lipgloss.Width(sep)
	rightW := totalInner - leftW - sepW
	if rightW < 24 {
		rightW = 24
		leftW = totalInner - rightW - sepW
	}

	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	maxH := len(leftLines)
	if len(rightLines) > maxH {
		maxH = len(rightLines)
	}

	var b strings.Builder
	for i := 0; i < maxH; i++ {
		ll := columnLine(leftLines, i)
		rl := columnLine(rightLines, i)
		b.WriteString(padVisible(ll, leftW))
		b.WriteString(sep)
		b.WriteString(padVisible(rl, rightW))
		if i < maxH-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func columnLine(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return ""
}

// padVisible 按可见宽度右填充空格，兼容 lipgloss ANSI 样式字符串。
func padVisible(s string, width int) string {
	vw := lipgloss.Width(s)
	if vw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vw)
}
