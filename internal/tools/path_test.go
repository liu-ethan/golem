package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePathInsideRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "foo", "bar.txt")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ValidatePath(root, "foo/bar.txt")
	if err != nil {
		t.Fatalf("ValidatePath() error = %v", err)
	}
	if got != inside {
		t.Fatalf("ValidatePath() = %q, want %q", got, inside)
	}
}

func TestValidatePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()

	cases := []string{
		"../../etc/passwd",
		"../outside.txt",
		"foo/../../etc/passwd",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			_, err := ValidatePath(root, path)
			if err == nil {
				t.Fatalf("ValidatePath(%q) expected error", path)
			}
		})
	}
}
