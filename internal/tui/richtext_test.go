package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/tencent-docs/golem/internal/tui/style"
)

func TestRenderUserTextSlashHighlight(t *testing.T) {
	out := renderUserText("/permissions plan")
	if !strings.Contains(stripVisible(out), "/permissions plan") {
		t.Fatalf("missing user text: %s", out)
	}
}

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

func TestNormalizeMarkdownTablePadding(t *testing.T) {
	in := "| 函数                | 时间复杂度 | 空间复杂度 | 说明                                |\n| ------------------- | ---------- | ---------- | ----------------------------------- |\n| twoSum              | O(n)       | O(n)       | **推荐**，哈希表一趟扫描            |"
	out := renderRichText(in, style.AsstText)
	plain := stripVisible(out)
	if strings.Contains(plain, "              ") {
		t.Fatalf("table padding not collapsed: %s", plain)
	}
	if !strings.Contains(plain, "twoSum │ O(n) │ O(n)") {
		t.Fatalf("expected compact table row: %s", plain)
	}
	if strings.Contains(plain, "---") {
		t.Fatal("table separator row should be omitted")
	}
}

func TestNormalizeMarkdownHeaderAndHR(t *testing.T) {
	in := "---\n\n### 文件概览\n\n内容"
	out := renderRichText(in, style.AsstText)
	plain := stripVisible(out)
	if strings.Contains(plain, "###") {
		t.Fatalf("header marker should be stripped: %s", plain)
	}
	if !strings.Contains(plain, "文件概览") {
		t.Fatal("expected header text")
	}
	if strings.Contains(plain, "---") {
		t.Fatal("horizontal rule should be omitted")
	}
}

func TestNormalizeTrailingWhitespace(t *testing.T) {
	out := renderRichText("hello world   \nfoo", style.AsstText)
	if strings.Contains(stripVisible(out), "world   ") {
		t.Fatalf("trailing spaces should be trimmed: %q", stripVisible(out))
	}
}

func TestRenderRichTextBold(t *testing.T) {
	out := renderRichText("这是 **推荐** 方案", style.AsstText)
	if strings.Contains(stripVisible(out), "**") {
		t.Fatalf("bold markers should be stripped: %s", stripVisible(out))
	}
	if !strings.Contains(stripVisible(out), "推荐") {
		t.Fatal("expected bold text content")
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

func TestRenderRichTextTrimsTrailingPadding(t *testing.T) {
	line := "hello                   "
	out := renderRichText(line, style.SysText)
	if strings.HasSuffix(stripVisible(out), " ") {
		t.Fatalf("trailing padding should be trimmed: %q", stripVisible(out))
	}
}

func TestHelpTextAlignment(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.lines = []ChatLine{
		{Kind: LineUser, Text: "/help"},
		{Kind: LineSystem, Text: helpText},
	}
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
