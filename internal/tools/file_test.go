package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryDefinitions(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, "")

	defs := reg.Definitions()
	if len(defs) != 7 {
		t.Fatalf("Definitions() len = %d, want 7", len(defs))
	}
	names := make(map[string]bool, len(defs))
	for _, def := range defs {
		names[def.Name] = true
	}
	for _, want := range []string{"bash", "read_file", "write_file", "edit_file", "list_dir", "grep", "web_search"} {
		if !names[want] {
			t.Fatalf("missing tool definition: %s", want)
		}
	}
}

func TestReadFileRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, "")

	_, err := reg.Execute(context.Background(), "read_file", map[string]any{
		"path": "../../etc/passwd",
	})
	if err == nil {
		t.Fatal("read_file traversal expected error")
	}
}

func TestReadWriteEditFile(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, "")
	ctx := context.Background()

	_, err := reg.Execute(ctx, "write_file", map[string]any{
		"path":    "notes/hello.txt",
		"content": "hello world",
	})
	if err != nil {
		t.Fatalf("write_file: %v", err)
	}

	got, err := reg.Execute(ctx, "read_file", map[string]any{
		"path": "notes/hello.txt",
	})
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("read_file = %q, want %q", got, "hello world")
	}

	_, err = reg.Execute(ctx, "edit_file", map[string]any{
		"path":       "notes/hello.txt",
		"old_string": "world",
		"new_string": "golem",
	})
	if err != nil {
		t.Fatalf("edit_file: %v", err)
	}

	got, err = reg.Execute(ctx, "read_file", map[string]any{
		"path": "notes/hello.txt",
	})
	if err != nil {
		t.Fatalf("read_file after edit: %v", err)
	}
	if got != "hello golem" {
		t.Fatalf("after edit = %q, want %q", got, "hello golem")
	}
}

func TestWriteFileRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, "")

	_, err := reg.Execute(context.Background(), "write_file", map[string]any{
		"path":    "../../outside.txt",
		"content": "nope",
	})
	if err == nil {
		t.Fatal("write_file traversal expected error")
	}
}

func TestEditFileMissingOldString(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, "")
	ctx := context.Background()

	if _, err := reg.Execute(ctx, "write_file", map[string]any{
		"path":    "a.txt",
		"content": "alpha",
	}); err != nil {
		t.Fatal(err)
	}

	_, err := reg.Execute(ctx, "edit_file", map[string]any{
		"path":       "a.txt",
		"old_string": "missing",
		"new_string": "beta",
	})
	if err == nil {
		t.Fatal("edit_file expected error for missing old_string")
	}
}

func TestReadFileRejectsBinary(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bin")
	if err := os.WriteFile(path, []byte{0x7f, 'E', 'L', 'F', 0x00, 0x01}, 0o644); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry(root, "")
	_, err := reg.Execute(context.Background(), "read_file", map[string]any{"path": "bin"})
	if err == nil {
		t.Fatal("expected error for binary file")
	}
	if !strings.Contains(err.Error(), "binary file") {
		t.Fatalf("error = %v", err)
	}
}

func TestReadFileTruncatesLargeText(t *testing.T) {
	root := t.TempDir()
	content := strings.Repeat("a", readFileMaxReturnBytes+100)
	path := filepath.Join(root, "big.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry(root, "")
	got, err := reg.Execute(context.Background(), "read_file", map[string]any{"path": "big.txt"})
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if !strings.Contains(got, "(truncated:") {
		t.Fatalf("expected truncation notice, got len=%d", len(got))
	}
	if len(got) > readFileMaxReturnBytes+128 {
		t.Fatalf("output too large: %d bytes", len(got))
	}
}

func TestReadFileRejectsDirectory(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root, "")
	_, err := reg.Execute(context.Background(), "read_file", map[string]any{"path": "."})
	if err == nil {
		t.Fatal("expected error for directory")
	}
}
