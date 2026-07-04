package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/tencent-docs/golem/internal/tui/style"
)

func TestRenderRichTextInlineCode(t *testing.T) {
	out := renderRichText("use `main.go` here", style.AsstText)
	if !strings.Contains(out, "main.go") {
		t.Fatalf("missing code: %s", out)
	}
	if out == lipgloss.NewStyle().Render("use `main.go` here") {
		t.Fatal("expected styled code")
	}
}

func TestRenderRichTextPath(t *testing.T) {
	out := renderRichText("edit internal/tui/view.go please", style.AsstText)
	if !strings.Contains(out, "internal/tui/view.go") {
		t.Fatalf("missing path: %s", out)
	}
}

func TestRenderRichTextFencedBlock(t *testing.T) {
	in := "before\n```go\nfunc main() {}\n```\nafter"
	out := renderRichText(in, style.AsstText)
	if !strings.Contains(out, "func main()") {
		t.Fatalf("missing fenced code: %s", out)
	}
}

func TestSplitFencedCode(t *testing.T) {
	parts := splitFencedCode("a ```txt\nline\n``` b")
	if len(parts) != 3 {
		t.Fatalf("parts = %d", len(parts))
	}
	if !parts[1].fenced || parts[1].text != "line" {
		t.Fatalf("fenced part = %+v", parts[1])
	}
}

func TestRenderRichTextPreservesColumnWidth(t *testing.T) {
	line := "  /status                   显示 model / approval / sandbox / session / tokens"
	base := style.SysText
	plainW := lipgloss.Width(base.Render(line))
	richW := lipgloss.Width(renderRichText(line, base))
	if plainW != richW {
		t.Fatalf("width mismatch plain=%d rich=%d", plainW, richW)
	}
}

func TestHelpTextAlignment(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.lines = []ChatLine{{Kind: LineSystem, Text: helpText}}
	out := renderChatArea(m, 120)
	plain := style.SysText.Render(helpText)
	if lipgloss.Width(out) < lipgloss.Width(plain) {
		t.Fatal("help output narrower than plain text")
	}
	for _, want := range []string{
		"/help",
		"/permissions",
		"列出命令与快捷键",
		"快捷键：",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q", want)
		}
	}
	// 对齐列：第二行命令应以两个空格开头（可见宽度一致）。
	helpLine := "  /permissions              权限页"
	if !strings.Contains(stripVisible(out), stripVisible(style.SysText.Render(helpLine))) {
		t.Fatal("help line alignment drift")
	}
}

func stripVisible(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\x1b' {
			return -1
		}
		return r
	}, s)
}
