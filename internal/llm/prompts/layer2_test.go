package prompts

import (
	"strings"
	"testing"
)

func TestLayer2SystemPromptIncludesSections(t *testing.T) {
	got := Layer2SystemPrompt()
	for _, want := range []string{"用户画像", "技术偏好", "项目上下文", "工作习惯", "session_count"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestLayer2SystemPromptIncludesExample(t *testing.T) {
	got := Layer2SystemPrompt()
	if !strings.Contains(got, "modernc.org/sqlite") {
		t.Error("prompt should include few-shot example")
	}
}
