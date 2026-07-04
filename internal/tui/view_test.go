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
