package tui

import (
	"testing"

	"github.com/tencent-docs/golem/internal/skills"
	"github.com/tencent-docs/golem/internal/testutil"
)

func TestMatchSlashSuggestionsPartial(t *testing.T) {
	matches := matchSlashSuggestions("/mod", nil)
	if len(matches) == 0 {
		t.Fatal("expected matches for /mod")
	}
	if matches[0].Name != "model" {
		t.Fatalf("first match = %q", matches[0].Name)
	}
}

func TestMatchSlashSuggestionsNoArgs(t *testing.T) {
	if got := matchSlashSuggestions("/model foo", nil); got != nil {
		t.Fatalf("expected nil when args present, got %v", got)
	}
}

func TestResolveSlashInput(t *testing.T) {
	suggestions := matchSlashSuggestions("/mod", nil)
	got := resolveSlashInput("/mod", 0, suggestions)
	if got != "/model" {
		t.Fatalf("got %q", got)
	}
}

func TestCompleteSlashInputAddsSpace(t *testing.T) {
	suggestions := matchSlashSuggestions("/mod", nil)
	got := completeSlashInput("/mod", 0, suggestions)
	if got != "/model " {
		t.Fatalf("got %q", got)
	}
}

func TestMatchSlashSuggestionsAllCommands(t *testing.T) {
	matches := matchSlashSuggestions("/", nil)
	if len(matches) != len(slashCommandCatalog) {
		t.Fatalf("got %d matches, want %d", len(matches), len(slashCommandCatalog))
	}
}

func TestMatchSlashSuggestionsIncludesSkills(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	loader := skills.NewLoader(root)
	matches := matchSlashSuggestions("/", loader)
	if len(matches) <= len(slashCommandCatalog) {
		t.Fatalf("expected commands + skills, got %d", len(matches))
	}
	found := false
	for _, m := range matches {
		if m.Name == "golang-expert" {
			found = true
			if m.Desc == "" {
				t.Fatal("skill suggestion missing desc")
			}
		}
	}
	if !found {
		t.Fatal("golang-expert not in slash suggestions")
	}
}

func TestMatchSlashSuggestionsSkillPrefix(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	loader := skills.NewLoader(root)
	matches := matchSlashSuggestions("/code", loader)
	found := false
	for _, m := range matches {
		if m.Name == "code-reviewer" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected code-reviewer in %v", matches)
	}
}

func TestDispatchSlashRunSkill(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	loader := skills.NewLoader(root)
	r := dispatchSlash("/golang-expert 帮我写代码", loader)
	if !r.handled || r.runSkill != "golang-expert" || r.skillQuery != "帮我写代码" {
		t.Fatalf("result = %+v", r)
	}
}

func TestDispatchSlashSkillRequiresQuery(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	loader := skills.NewLoader(root)
	r := dispatchSlash("/golang-expert", loader)
	if !r.handled || r.message == "" || r.runSkill != "" {
		t.Fatalf("result = %+v", r)
	}
}

func TestDispatchSlashCommandWinsOverSkillName(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	loader := skills.NewLoader(root)
	r := dispatchSlash("/help", loader)
	if r.message == "" && r.runSkill != "" {
		t.Fatalf("help should be command not skill: %+v", r)
	}
}

func TestSlashSuggestionViewport(t *testing.T) {
	tests := []struct {
		sel, total, max      int
		wantStart, wantEnd   int
	}{
		{0, 22, 8, 0, 8},
		{7, 22, 8, 0, 8},
		{8, 22, 8, 1, 9},
		{10, 22, 8, 3, 11},
		{21, 22, 8, 14, 22},
		{0, 5, 8, 0, 5},
	}
	for _, tc := range tests {
		start, end := slashSuggestionViewport(tc.sel, tc.total, tc.max)
		if start != tc.wantStart || end != tc.wantEnd {
			t.Errorf("viewport(sel=%d total=%d max=%d) = (%d,%d), want (%d,%d)",
				tc.sel, tc.total, tc.max, start, end, tc.wantStart, tc.wantEnd)
		}
	}
}
