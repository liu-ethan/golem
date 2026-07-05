package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tencent-docs/golem/internal/tui/style"
)

var (
	inlineCodeRe = regexp.MustCompile("`([^`]+)`")
	pathRe       = regexp.MustCompile(`(?:^|[\s(])((?:\./[\w./@-]+|/[\w][\w./@-]*|~/?[\w][\w./@-]*)|[\w./@-]+\.(?:go|py|js|ts|tsx|jsx|md|yaml|yml|json|toml|sh|rs|txt|mod|sum|lock|sql|css|html|xml|csv|env|gitignore|dockerfile|makefile|golem)(?:/[\w./@-]+)?)\b`)
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
				b.WriteString(base.Render(style.CodeText.Render(line)))
				b.WriteString("\n")
			}
			continue
		}
		b.WriteString(renderInlineRich(p.text, base))
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
	kind       int // 0=code, 1=path
	match      string
}

// renderInlineRich 渲染行内代码与文件路径，其余走 base 样式。
func renderInlineRich(text string, base lipgloss.Style) string {
	var spans []richSpan

	for _, loc := range inlineCodeRe.FindAllStringSubmatchIndex(text, -1) {
		if len(loc) >= 4 {
			spans = append(spans, richSpan{start: loc[0], end: loc[1], kind: 0, match: text[loc[2]:loc[3]]})
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
