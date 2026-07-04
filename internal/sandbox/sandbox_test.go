package sandbox

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		raw  string
		want SandboxMode
	}{
		{"", ModeWorkspaceWrite},
		{"workspace-write", ModeWorkspaceWrite},
		{"danger-full-access", ModeDangerFullAccess},
		{"unknown", ModeWorkspaceWrite},
	}
	for _, tc := range tests {
		if got := ParseMode(tc.raw); got != tc.want {
			t.Errorf("ParseMode(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestRunBashDangerFullAccess(t *testing.T) {
	root := t.TempDir()
	out, err := RunBash(context.Background(), "echo hello-sandbox", root, ModeDangerFullAccess)
	if err != nil {
		t.Fatalf("RunBash: %v", err)
	}
	if out != "hello-sandbox" {
		t.Fatalf("output = %q", out)
	}
}

func TestRunBashWorkspaceWriteEcho(t *testing.T) {
	root := t.TempDir()
	out, err := RunBash(context.Background(), "echo hello-sandbox", root, ModeWorkspaceWrite)
	if err != nil {
		t.Fatalf("RunBash: %v", err)
	}
	if runtime.GOOS != "linux" || !UserNamespaceSupported() {
		if !strings.HasPrefix(out, FallbackWarning) {
			t.Fatalf("expected fallback warning, got %q", out)
		}
		if !strings.Contains(out, "hello-sandbox") {
			t.Fatalf("expected command output in %q", out)
		}
		return
	}
	if out != "hello-sandbox" {
		t.Fatalf("output = %q", out)
	}
}

func TestRunBashUsesProjectRoot(t *testing.T) {
	root := t.TempDir()
	out, err := RunBash(context.Background(), "pwd", root, ModeDangerFullAccess)
	if err != nil {
		t.Fatalf("RunBash: %v", err)
	}
	if out != root {
		t.Fatalf("pwd = %q, want %q", out, root)
	}
}

func TestRunBashNonZeroExitReturnsOutput(t *testing.T) {
	root := t.TempDir()
	out, err := RunBash(context.Background(), "echo partial && exit 1", root, ModeDangerFullAccess)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if !strings.Contains(out, "partial") {
		t.Fatalf("output = %q, want partial", out)
	}
}

func TestRunBashNamespaceIsolation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	if !UserNamespaceSupported() {
		t.Skip("user namespace not supported")
	}

	parentNS, err := os.Readlink("/proc/self/ns/user")
	if err != nil {
		t.Skip(err)
	}

	root := t.TempDir()
	out, err := RunBash(context.Background(), "readlink /proc/self/ns/user", root, ModeWorkspaceWrite)
	if err != nil {
		t.Fatalf("RunBash: %v", err)
	}
	if strings.HasPrefix(out, FallbackWarning) {
		t.Fatalf("expected namespace execution, got fallback: %q", out)
	}
	if out == parentNS {
		t.Fatalf("bash user namespace = parent %q", parentNS)
	}

	direct, err := RunBash(context.Background(), "readlink /proc/self/ns/user", root, ModeDangerFullAccess)
	if err != nil {
		t.Fatalf("RunBash danger: %v", err)
	}
	if direct != parentNS {
		t.Fatalf("danger mode ns = %q, want parent %q", direct, parentNS)
	}
}

func TestRunBashNonLinuxFallback(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("non-linux fallback test")
	}
	root := t.TempDir()
	out, err := RunBash(context.Background(), "echo ok", root, ModeWorkspaceWrite)
	if err != nil {
		t.Fatalf("RunBash: %v", err)
	}
	if !strings.HasPrefix(out, FallbackWarning) {
		t.Fatalf("expected fallback warning, got %q", out)
	}
}
