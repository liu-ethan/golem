package pages

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tencent-docs/golem/internal/approval"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	activeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	ruleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

// Permissions 渲染 /permissions 子页：上半区 approval 模式列表，下半区 rules 只读展示。
func Permissions(width, height int, currentMode string, cursor int, rulesLines []string) string {
	if width < 20 {
		width = 20
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("/permissions — 审批模式"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓ 选择 · Enter 切换 · Esc 返回聊天"))
	b.WriteString("\n\n")

	for i, mode := range approval.Modes {
		label := modeLabel(mode)
		line := fmt.Sprintf("  %s  %s", modeMarker(i == cursor), label)
		if mode == currentMode {
			line += "  ← 当前"
		}
		if i == cursor {
			b.WriteString(activeStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("权限规则（只读，P1 起可增删）"))
	b.WriteString("\n")

	rulesBudget := height - strings.Count(b.String(), "\n") - 3
	if rulesBudget < 3 {
		rulesBudget = 3
	}
	for i, line := range rulesLines {
		if i >= rulesBudget {
			b.WriteString(dimStyle.Render("  …"))
			b.WriteString("\n")
			break
		}
		b.WriteString(ruleStyle.Render("  " + truncate(line, width-4)))
		b.WriteString("\n")
	}
	if len(rulesLines) == 0 {
		b.WriteString(dimStyle.Render("  （无规则）"))
		b.WriteString("\n")
	}
	return b.String()
}

// SessionEntry 供 sessions 页渲染的单条会话摘要。
type SessionEntry struct {
	ID        string
	CreatedAt time.Time
	Preview   string
}

// Sessions 渲染 /sessions 子页：最近会话列表，Enter 恢复。
func Sessions(width int, entries []SessionEntry, cursor int, currentID string) string {
	if width < 20 {
		width = 20
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("/sessions — 最近会话"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓ 选择 · Enter 恢复 · Esc 返回聊天"))
	b.WriteString("\n\n")

	if len(entries) == 0 {
		b.WriteString(dimStyle.Render("  （暂无历史会话）"))
		b.WriteString("\n")
		return b.String()
	}

	for i, e := range entries {
		marker := modeMarker(i == cursor)
		idShort := e.ID
		if len(idShort) > 8 {
			idShort = idShort[:8]
		}
		when := e.CreatedAt.Local().Format("2006-01-02 15:04")
		preview := e.Preview
		if preview == "" {
			preview = "（无 user 消息）"
		}
		line := fmt.Sprintf("  %s %s  %s  %s", marker, idShort, when, truncate(preview, width-30))
		if e.ID == currentID {
			line += "  ← 当前"
		}
		if i == cursor {
			b.WriteString(activeStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func modeLabel(mode string) string {
	switch mode {
	case approval.ModePlan:
		return "plan — 只读探索"
	case approval.ModeAskBeforeEdit:
		return "ask-before-edit — 写/bash 前确认（默认）"
	case approval.ModeAsk:
		return "ask — 任意 tool 前确认"
	case approval.ModeEditAutomatically:
		return "edit-automatically — 全自动"
	default:
		return mode
	}
}

func modeMarker(active bool) string {
	if active {
		return "▸"
	}
	return " "
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
