package prompts

import (
	"strings"
	"testing"
)

func TestLayer1SystemPromptIncludesCategories(t *testing.T) {
	got := Layer1SystemPrompt()
	for _, want := range []string{"preference", "project_fact", "task_progress", "JSON 数组"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestLayer1SystemPromptIncludesExample(t *testing.T) {
	got := Layer1SystemPrompt()
	if !strings.Contains(got, "golem 权限规则引擎") {
		t.Error("prompt should include few-shot example")
	}
}
