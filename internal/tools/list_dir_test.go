package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry(root)
	out, err := reg.Execute(context.Background(), "list_dir", map[string]any{
		"path": ".",
	})
	if err != nil {
		t.Fatalf("list_dir: %v", err)
	}
	if !strings.Contains(out, "README.md") || !strings.Contains(out, "pkg/") {
		t.Fatalf("list_dir output = %q", out)
	}
}

func TestListDirRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	reg := NewRegistry(root)

	_, err := reg.Execute(context.Background(), "list_dir", map[string]any{
		"path": "../../",
	})
	if err == nil {
		t.Fatal("list_dir traversal expected error")
	}
}

func TestListDirNotDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry(root)
	_, err := reg.Execute(context.Background(), "list_dir", map[string]any{
		"path": "file.txt",
	})
	if err == nil {
		t.Fatal("list_dir on file expected error")
	}
}
