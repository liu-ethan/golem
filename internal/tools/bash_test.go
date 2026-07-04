package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/sandbox"
)

func TestBashEcho(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, sandbox.ModeDangerFullAccess)

	out, err := reg.Execute(context.Background(), "bash", map[string]any{
		"command": "echo hello-golem",
	})
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	if out != "hello-golem" {
		t.Fatalf("bash output = %q, want %q", out, "hello-golem")
	}
}

func TestBashUsesProjectRoot(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, sandbox.ModeDangerFullAccess)

	out, err := reg.Execute(context.Background(), "bash", map[string]any{
		"command": "pwd",
	})
	if err != nil {
		t.Fatalf("bash pwd: %v", err)
	}
	if out != root {
		t.Fatalf("bash pwd = %q, want %q", out, root)
	}
}

func TestBashMissingCommand(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, sandbox.ModeDangerFullAccess)

	_, err := reg.Execute(context.Background(), "bash", map[string]any{})
	if err == nil {
		t.Fatal("bash expected error for missing command")
	}
}

func TestBashNonZeroExitStillReturnsOutput(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, sandbox.ModeDangerFullAccess)

	out, err := reg.Execute(context.Background(), "bash", map[string]any{
		"command": "echo partial && exit 1",
	})
	if err == nil {
		t.Fatal("bash expected error for non-zero exit")
	}
	if !strings.Contains(out, "partial") {
		t.Fatalf("bash output = %q, want partial output", out)
	}
}

func TestBashWorkspaceWriteViaRegistry(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, sandbox.ModeWorkspaceWrite)

	out, err := reg.Execute(context.Background(), "bash", map[string]any{
		"command": "echo sandbox-mode",
	})
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	if strings.Contains(out, sandbox.FallbackWarning) {
		if !strings.Contains(out, "sandbox-mode") {
			t.Fatalf("fallback output = %q", out)
		}
		return
	}
	if out != "sandbox-mode" {
		t.Fatalf("bash output = %q, want sandbox-mode", out)
	}
}
