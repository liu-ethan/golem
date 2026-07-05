package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tencent-docs/golem/internal/tui/style"
)

var (
	inlineCodeRe = regexp.MustCompile("`([^`]+)`")
	boldRe       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	pathRe       = regexp.MustCompile(`(?:^|[\s(])((?:\./[\w./@-]+|/[\w][\w./@-]*|~/?[\w][\w./@-]*)|[\w./@-]+\.(?:go|py|js|ts|tsx|jsx|md|yaml|yml|json|toml|sh|rs|txt|mod|sum|lock|sql|css|html|xml|csv|env|gitignore|dockerfile|makefile|golem)(?:/[\w./@-]+)?)\b`)
	headerRe     = regexp.MustCompile(`^#{1,6}\s+(.+)$`)
)

// renderUserText 渲染用户消息：斜杠命令词用次强调色，其余正文走 UserText 语义色。
func renderUserText(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "/") {
		fields := strings.Fields(trimmed)
		if len(fields) > 0 {
			var b strings.Builder
			b.WriteString(style.UserSlash.Render(fields[0]))
			if len(fields) > 1 {
				b.WriteString(style.UserText.Render(" " + strings.Join(fields[1:], " ")))
			}
			return b.String()
		}
	}
	return renderRichText(text, style.UserText)
}

// renderRichText 为聊天正文添加语义着色：行内代码、路径、围栏代码块。
func renderRichText(text string, base lipgloss.Style) string {
	if text == "" {
		return ""
	}
	parts := splitFencedCode(text)
	var b strings.Builder
	for _, p := range parts {
		if p.fenced {
			for _, line := range strings.Split(p.text, "\n") {
				b.WriteString(base.Render(style.CodeText.Render(strings.TrimRight(line, " \t"))))
				b.WriteString("\n")
			}
			continue
		}
		b.WriteString(renderPlainBlock(p.text, base))
	}
	out := b.String()
	if strings.HasSuffix(out, "\n") {
		out = strings.TrimSuffix(out, "\n")
	}
	return out
}

type textPart struct {
	text   string
	fenced bool
}

// splitFencedCode 按 ``` 围栏切分文本，保留围栏内为独立块。
func splitFencedCode(text string) []textPart {
	var parts []textPart
	for {
		start := strings.Index(text, "```")
		if start < 0 {
			if text != "" {
				parts = append(parts, textPart{text: text})
			}
			break
		}
		if start > 0 {
			parts = append(parts, textPart{text: text[:start]})
		}
		rest := text[start+3:]
		end := strings.Index(rest, "```")
		if end < 0 {
			parts = append(parts, textPart{text: text[start:]})
			break
		}
		block := rest[:end]
		if nl := strings.Index(block, "\n"); nl >= 0 {
			block = block[nl+1:]
		}
		block = strings.TrimSuffix(block, "\n")
		parts = append(parts, textPart{text: block, fenced: true})
		text = rest[end+3:]
	}
	if len(parts) == 0 {
		return []textPart{{text: text}}
	}
	return parts
}

type richSpan struct {
	start, end int
	kind       int // 0=code, 1=path, 2=bold
	match      string
}

const (
	lineKindSkip = iota
	lineKindBlank
	lineKindHeader
	lineKindTable
	lineKindNormal
)

// renderPlainBlock 逐行规范化 markdown 后再着色，避免表格对齐空格污染终端布局。
func renderPlainBlock(text string, base lipgloss.Style) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	var b strings.Builder
	prevBlank := false
	for _, line := range lines {
		kind, content := normalizeChatLine(line)
		switch kind {
		case lineKindSkip:
			continue
		case lineKindBlank:
			if prevBlank {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			prevBlank = true
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		prevBlank = false
		switch kind {
		case lineKindHeader:
			b.WriteString(base.Render(style.Emphasis.Render(content)))
		default:
			b.WriteString(renderInlineRich(content, base))
		}
	}
	return b.String()
}

// normalizeChatLine 清理单行 markdown：去尾空白、压缩表格列、剥离标题与分隔线。
func normalizeChatLine(line string) (kind int, text string) {
	line = strings.TrimRight(line, " \t")
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return lineKindBlank, ""
	}
	if isHorizontalRule(trimmed) {
		return lineKindSkip, ""
	}
	if isMarkdownTableRow(trimmed) {
		cells := parseTableCells(trimmed)
		if isTableSeparatorRow(cells) {
			return lineKindSkip, ""
		}
		return lineKindTable, formatTableCells(cells)
	}
	if m := headerRe.FindStringSubmatch(trimmed); len(m) == 2 {
		return lineKindHeader, m[1]
	}
	return lineKindNormal, trimmed
}

// isHorizontalRule 判断 markdown 水平分隔线（---、*** 等）。
func isHorizontalRule(s string) bool {
	compact := strings.ReplaceAll(s, " ", "")
	if len(compact) < 3 {
		return false
	}
	var ch rune
	for i, r := range compact {
		if r != '-' && r != '*' && r != '_' {
			return false
		}
		if i == 0 {
			ch = r
			continue
		}
		if r != ch {
			return false
		}
	}
	return true
}

// isMarkdownTableRow 判断是否为 markdown 表格行。
func isMarkdownTableRow(s string) bool {
	return strings.HasPrefix(s, "|") && strings.Count(s, "|") >= 2
}

// parseTableCells 按 | 切分表格行并 trim 各单元格。
func parseTableCells(line string) []string {
	parts := strings.Split(line, "|")
	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	if len(cells) > 0 && cells[0] == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// isTableSeparatorRow 判断 |---|---| 形式的表头分隔行。
func isTableSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if c == "" {
			continue
		}
		for _, r := range c {
			if r != '-' && r != ':' && r != ' ' {
				return false
			}
		}
	}
	return true
}

// formatTableCells 将表格单元格用 │ 紧凑拼接，去掉源 markdown 的对齐 padding。
func formatTableCells(cells []string) string {
	return strings.Join(cells, " │ ")
}

// renderInlineRich 渲染行内代码与文件路径，其余走 base 样式。
func renderInlineRich(text string, base lipgloss.Style) string {
	var spans []richSpan

	for _, loc := range inlineCodeRe.FindAllStringSubmatchIndex(text, -1) {
		if len(loc) >= 4 {
			spans = append(spans, richSpan{start: loc[0], end: loc[1], kind: 0, match: text[loc[2]:loc[3]]})
		}
	}
	for _, loc := range boldRe.FindAllStringSubmatchIndex(text, -1) {
		if len(loc) >= 4 {
			spans = append(spans, richSpan{start: loc[0], end: loc[1], kind: 2, match: text[loc[2]:loc[3]]})
		}
	}
	for _, loc := range pathRe.FindAllStringSubmatchIndex(text, -1) {
		if len(loc) >= 4 {
			spans = append(spans, richSpan{
				start: loc[2],
				end:   loc[3],
				kind:  1,
				match: text[loc[2]:loc[3]],
			})
		}
	}
	if len(spans) == 0 {
		return base.Render(text)
	}

	spans = dedupeSpans(spans)

	var b strings.Builder
	pos := 0
	for _, sp := range spans {
		if sp.start < pos {
			continue
		}
		b.WriteString(base.Render(text[pos:sp.start]))
		switch sp.kind {
		case 0:
			b.WriteString(base.Render(style.CodeText.Render(sp.match)))
		case 1:
			b.WriteString(base.Render(style.PathText.Render(sp.match)))
		case 2:
			b.WriteString(base.Render(style.Emphasis.Render(sp.match)))
		}
		pos = sp.end
	}
	b.WriteString(base.Render(text[pos:]))
	return b.String()
}

func dedupeSpans(spans []richSpan) []richSpan {
	if len(spans) <= 1 {
		return spans
	}
	// 简单排序：code 优先，再按 start。
	for i := 0; i < len(spans); i++ {
		for j := i + 1; j < len(spans); j++ {
			if spans[j].start < spans[i].start || (spans[j].start == spans[i].start && spans[j].kind < spans[i].kind) {
				spans[i], spans[j] = spans[j], spans[i]
			}
		}
	}
	out := spans[:0]
	end := 0
	for _, sp := range spans {
		if sp.start >= end {
			out = append(out, sp)
			end = sp.end
		}
	}
	return out
}
