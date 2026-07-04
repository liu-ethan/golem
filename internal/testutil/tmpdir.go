package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TempProjectRoot 创建临时目录作为 project_root，并确保 .golem 子目录存在。
func TempProjectRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".golem"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}
