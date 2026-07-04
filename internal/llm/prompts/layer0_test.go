package prompts

import (
	"strings"
	"testing"
)

func TestLayer0SystemPromptIncludesExtraInstructions(t *testing.T) {
	got := Layer0SystemPrompt("只保留 Go 相关讨论")
	if !strings.Contains(got, "只保留 Go 相关讨论") {
		t.Fatalf("prompt = %q", got)
	}
	if !strings.Contains(got, "输出格式") {
		t.Error("prompt should include output format section")
	}
}

func TestLayer0SystemPromptWithoutExtra(t *testing.T) {
	got := Layer0SystemPrompt("")
	if strings.Contains(got, "用户额外要求") {
		t.Error("should not include extra section when empty")
	}
}
