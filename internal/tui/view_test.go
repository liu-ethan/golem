package tui

import (
	"strings"
	"testing"
)

func TestRenderInputLineShowsCursor(t *testing.T) {
	out := renderInputLine("hello", false, true)
	if !strings.Contains(out, "hello") {
		t.Fatalf("missing input: %s", out)
	}
	if !strings.Contains(out, "▌") {
		t.Fatal("expected cursor when showCursor=true")
	}
}

func TestRenderInputLineHidesCursorWhenBlinkOff(t *testing.T) {
	out := renderInputLine("", false, false)
	if strings.Contains(out, "▌") {
		t.Fatal("cursor should be hidden when showCursor=false")
	}
}

func TestRenderInputLineRunningAlwaysShowsCursor(t *testing.T) {
	out := renderInputLine("x", true, false)
	if !strings.Contains(out, "▌") {
		t.Fatal("cursor should show while agent running")
	}
}

func TestRenderThinkingBlockCapsWidthOnWideTerminal(t *testing.T) {
	out := renderThinkingBlock("hello", 160, false)
	lines := strings.Split(stripANSI(out), "\n")
	topRunes := len([]rune(lines[0]))
	if topRunes > thinkingBoxMaxWidth {
		t.Fatalf("top line %d runes exceeds cap %d", topRunes, thinkingBoxMaxWidth)
	}
	if topRunes != len([]rune(lines[len(lines)-1])) {
		t.Fatal("top and bottom border width mismatch")
	}
}

func TestRenderThinkingBlockBorderWidthMatches(t *testing.T) {
	out := renderThinkingBlock("hello", 80, false)
	lines := strings.Split(stripANSI(out), "\n")
	topRunes := len([]rune(lines[0]))
	bottomRunes := len([]rune(lines[len(lines)-1]))
	if topRunes != bottomRunes {
		t.Fatalf("top (%d) and bottom (%d) rune width mismatch", topRunes, bottomRunes)
	}
}

func TestRenderSlashSuggestionsScrollsWithSelection(t *testing.T) {
	suggestions := matchSlashSuggestions("/", nil)
	if len(suggestions) < 12 {
		t.Fatalf("need enough suggestions, got %d", len(suggestions))
	}
	sel := 10
	out := stripANSI(renderSlashSuggestions(suggestions, sel, 80))
	selected := "/" + suggestions[sel].Name
	if !strings.Contains(out, selected) {
		t.Fatalf("selected %q not in scrolled output", selected)
	}
	first := "/" + suggestions[0].Name
	if strings.Contains(out, first+" ") || strings.Contains(out, first+"\t") {
		t.Fatalf("scrolled view should not show first item %q when sel=%d", first, sel)
	}
	if !strings.Contains(out, "上方") {
		t.Fatal("expected scroll-up hint when selection moved down")
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	esc := false
	for _, r := range s {
		if esc {
			if r == 'm' {
				esc = false
			}
			continue
		}
		if r == '\x1b' {
			esc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
