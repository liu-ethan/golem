package pages

import (
	"fmt"
	"strings"
	"time"

	"github.com/tencent-docs/golem/internal/memory"
)

// MemoryFactView 供 /memories 页展示的单条记忆。
type MemoryFactView struct {
	ID        string
	Content   string
	Category  string
	CreatedAt time.Time
}

// Memories 渲染 /memories 子页。
func Memories(width int, facts []MemoryFactView, injectEnabled bool, cursor int) string {
	if width < 20 {
		width = 20
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("/memories — 情节记忆"))
	b.WriteString("\n")
	status := "注入: 开"
	if !injectEnabled {
		status = "注入: 关"
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf("↑/↓ 选择 · i 切换注入(%s) · c 清空 · l 触发 Layer 2 · Esc 返回", status)))
	b.WriteString("\n\n")

	if len(facts) == 0 {
		b.WriteString(dimStyle.Render("  （暂无 memory_facts）"))
		b.WriteString("\n")
		return b.String()
	}

	for i, f := range facts {
		marker := modeMarker(i == cursor)
		when := f.CreatedAt.Local().Format("2006-01-02")
		line := fmt.Sprintf("  %s [%s] %s  %s", marker, f.Category, when, truncate(f.Content, width-24))
		if i == cursor {
			b.WriteString(activeStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// SkillEntry 供 /skills 页展示的 Skill 摘要。
type SkillEntry struct {
	Name   string
	Source string
}

// Skills 渲染 /skills 子页。
func Skills(width int, entries []SkillEntry, cursor int, scanPaths []string) string {
	if width < 20 {
		width = 20
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("/skills — Skill 列表"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓ 选择 · Enter 填入 /skill-name · Esc 返回"))
	if len(scanPaths) > 0 {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("扫描: " + strings.Join(scanPaths, " · ")))
	}
	b.WriteString("\n\n")

	if len(entries) == 0 {
		b.WriteString(dimStyle.Render("  （无可用 Skill）"))
		b.WriteString("\n")
		return b.String()
	}

	for i, e := range entries {
		marker := modeMarker(i == cursor)
		label := fmt.Sprintf("%s (%s)", e.Name, e.Source)
		line := fmt.Sprintf("  %s %s", marker, truncate(label, width-8))
		if i == cursor {
			b.WriteString(activeStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// DeniedEntry 供 Recently denied tab 展示。
type DeniedEntry struct {
	Tool      string
	Input     string
	Reason    string
	CreatedAt time.Time
}

// DeniedList 渲染 /permissions Recently denied 子页。
func DeniedList(width int, entries []DeniedEntry, cursor int) string {
	if width < 20 {
		width = 20
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Recently denied"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓ 选择 · r 重试 · Tab 切换页 · Esc 返回聊天"))
	b.WriteString("\n\n")

	if len(entries) == 0 {
		b.WriteString(dimStyle.Render("  （暂无拒绝记录）"))
		b.WriteString("\n")
		return b.String()
	}

	for i, e := range entries {
		marker := modeMarker(i == cursor)
		when := e.CreatedAt.Local().Format("15:04:05")
		line := fmt.Sprintf("  %s %s %s  %s", marker, when, e.Tool, truncate(e.Reason, width-30))
		if i == cursor {
			b.WriteString(activeStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// MemoryFactsToView 将 memory.MemoryFact 转为页面视图结构。
func MemoryFactsToView(facts []memory.MemoryFact) []MemoryFactView {
	out := make([]MemoryFactView, len(facts))
	for i, f := range facts {
		out[i] = MemoryFactView{
			ID:        f.ID,
			Content:   f.Content,
			Category:  f.Category,
			CreatedAt: f.CreatedAt,
		}
	}
	return out
}
