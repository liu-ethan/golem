package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderViewKeepsStatusBarVisible(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.height = 24
	m.width = 100
	for i := 0; i < 40; i++ {
		m.lines = append(m.lines, ChatLine{
			Kind: LineAssistant,
			Text: strings.Repeat("line ", 20),
		})
	}

	out := renderView(m)
	if lipgloss.Height(out) != m.height {
		t.Fatalf("view height = %d, want %d", lipgloss.Height(out), m.height)
	}
	if !strings.Contains(stripANSI(out), "golem") {
		t.Fatal("expected status bar at top of view")
	}
	firstLine := strings.Split(stripANSI(out), "\n")[0]
	if !strings.Contains(firstLine, "golem") {
		t.Fatalf("first line should be status bar, got %q", firstLine)
	}
}

func TestClipToViewportScrollsHistory(t *testing.T) {
	content := "a\nb\nc\nd\ne"
	top := clipToViewport(content, 2, 0)
	if top != "a\nb" {
		t.Fatalf("top viewport = %q", top)
	}
	bottom := clipToViewport(content, 2, 3)
	if bottom != "d\ne" {
		t.Fatalf("bottom viewport = %q", bottom)
	}
}

func TestScrollChatUpUnpinsBottom(t *testing.T) {
	m := testModel(t)
	m.chatPinnedBottom = true
	m.scrollChatUp(1)
	if m.chatPinnedBottom {
		t.Fatal("scroll up should unpin bottom")
	}
}

func TestScrollChatDownPinsAtBottom(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.height = 12
	m.width = 80
	m.lines = []ChatLine{
		{Kind: LineUser, Text: "one"},
		{Kind: LineAssistant, Text: "two"},
		{Kind: LineUser, Text: "three"},
		{Kind: LineAssistant, Text: "four"},
	}
	m.chatPinnedBottom = false
	m.chatScrollTop = 0
	m.scrollChatDown(100, m.width)
	if !m.chatPinnedBottom {
		t.Fatal("expected pinned to bottom after scrolling down past end")
	}
}
