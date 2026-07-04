package prompts

import (
	"strings"
	"testing"
)

func TestReviewSystemPromptStructure(t *testing.T) {
	p := ReviewSystemPrompt()
	for _, section := range []string{"Blockers", "Suggestions", "Nits", "测试建议"} {
		if !strings.Contains(p, section) {
			t.Fatalf("review prompt missing section %q", section)
		}
	}
}

func TestInitSystemPromptAndTemplate(t *testing.T) {
	p := InitSystemPrompt()
	if !strings.Contains(p, "AGENTS.md") {
		t.Fatal("init prompt should mention AGENTS.md")
	}
	tmpl := InitTemplate()
	if !strings.Contains(tmpl, "# AGENTS.md") {
		t.Fatal("init template should be markdown")
	}
}
