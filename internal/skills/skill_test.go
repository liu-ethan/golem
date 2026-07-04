package skills

import (
	"strings"
	"testing"
)

func TestBuiltinSkillsList(t *testing.T) {
	list := builtinSkills()
	if len(list) < 2 {
		t.Fatalf("expected builtin skills, got %d", len(list))
	}
	found := false
	for _, s := range list {
		if s.Name == "golang-expert" {
			found = true
			if !s.ToolAllowed("bash") {
				t.Fatal("bash should be allowed")
			}
			if s.ToolAllowed("web_search") {
				t.Fatal("web_search should be denied")
			}
		}
	}
	if !found {
		t.Fatal("golang-expert not found")
	}
}

func TestParseSkillMarkdown(t *testing.T) {
	content := `# demo-skill

You are a demo assistant.

## 工具权限
allowed: bash, read_file
denied: web_search

## 规则覆盖
allow go *
`
	s, err := parseSkillMarkdown(content, "project", "/tmp/demo")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "demo-skill" {
		t.Fatalf("name = %q", s.Name)
	}
	if !strings.Contains(s.SystemPrompt, "demo assistant") {
		t.Fatalf("prompt = %q", s.SystemPrompt)
	}
	if len(s.AllowedTools) != 2 || s.AllowedTools[0] != "bash" {
		t.Fatalf("allowed = %v", s.AllowedTools)
	}
}

func TestInstallGitHubInvalidRef(t *testing.T) {
	if _, err := InstallGitHub("invalid-ref"); err == nil {
		t.Fatal("expected error for invalid ref")
	}
}
