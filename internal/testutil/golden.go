package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden test files")

// AssertGolden 将 got 与 dir 下 name 对应的 golden 文件比对；传入 -update 时写入 golden。
func AssertGolden(t *testing.T, dir, name string, got []byte) {
	t.Helper()

	path := filepath.Join(dir, name)
	if *updateGolden {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated golden file %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (re-run with -update to create)", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden mismatch %s\n--- want ---\n%s--- got ---\n%s", path, want, got)
	}
}
