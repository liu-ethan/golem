package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepFindsMatch(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "agent.go"), []byte("func RunAgent() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry(root, "")
	out, err := reg.Execute(context.Background(), "grep", map[string]any{
		"pattern": "RunAgent",
	})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "internal/agent.go:1:func RunAgent() {}") {
		t.Fatalf("grep output = %q", out)
	}
}

func TestGrepSkipsGitDirectory(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git", "objects")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "secret.txt"), []byte("needle"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), []byte("needle"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry(root, "")
	out, err := reg.Execute(context.Background(), "grep", map[string]any{
		"pattern": "needle",
	})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if strings.Contains(out, ".git/") {
		t.Fatalf("grep searched .git: %q", out)
	}
	if !strings.Contains(out, "visible.txt:1:needle") {
		t.Fatalf("grep output = %q", out)
	}
}

func TestGrepResultLimit(t *testing.T) {
	root := t.TempDir()
	var lines strings.Builder
	for i := 0; i < 250; i++ {
		lines.WriteString("match line\n")
	}
	if err := os.WriteFile(filepath.Join(root, "many.txt"), []byte(lines.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry(root, "")
	out, err := reg.Execute(context.Background(), "grep", map[string]any{
		"pattern": "match",
	})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "truncated at 200 matches") {
		t.Fatalf("grep should truncate at 200 matches, got %q", out)
	}
}

func TestGrepInvalidPattern(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, "")

	_, err := reg.Execute(context.Background(), "grep", map[string]any{
		"pattern": "[",
	})
	if err == nil {
		t.Fatal("grep expected error for invalid pattern")
	}
}

func TestGrepRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, "")

	_, err := reg.Execute(context.Background(), "grep", map[string]any{
		"pattern": "root",
		"path":    "../../",
	})
	if err == nil {
		t.Fatal("grep traversal expected error")
	}
}
