package skills

import (
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/memory"
	"github.com/tencent-docs/golem/internal/testutil"
)

func TestInjectSkillsOnceIncludesCatalog(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	loader := NewLoader(root)
	block, err := InjectSkillsOnce(t.Context(), "hello", loader, memory.NewBM25Retriever(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(block, "可用 Skills") {
		t.Fatalf("block = %q", block)
	}
	if !strings.Contains(block, "golang-expert") {
		t.Fatal("expected golang-expert in catalog")
	}
}

func TestInjectSkillsOnceMatchesGoQuery(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	loader := NewLoader(root)
	block, err := InjectSkillsOnce(t.Context(), "帮我写 Go 代码并 review", loader, memory.NewBM25Retriever(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(block, "自动匹配的相关 Skill") {
		t.Fatalf("block = %q", block)
	}
	if !strings.Contains(block, "## Skill: golang-expert") || !strings.Contains(block, "## Skill: code-reviewer") {
		t.Fatalf("expected matched skill overlays, block = %q", block)
	}
}

func TestSummaryUsesFirstPromptLine(t *testing.T) {
	s := Skill{SystemPrompt: "- first line\n- second"}
	if got := s.Summary(); got != "first line" {
		t.Fatalf("summary = %q", got)
	}
}
