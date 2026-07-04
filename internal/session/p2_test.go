package session

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tencent-docs/golem/internal/testutil"
)

func TestDeleteSession(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	id := uuid.NewString()
	if err := st.EnsureSession(id); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteSession(id); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteSession(id); err == nil {
		t.Fatal("expected error deleting missing session")
	}
}

func TestRenameAndForkSession(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	src := uuid.NewString()
	if err := st.EnsureSession(src); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameSession(src, "my-session"); err != nil {
		t.Fatal(err)
	}
	name, err := st.SessionName(src)
	if err != nil || name != "my-session" {
		t.Fatalf("SessionName = %q, err = %v", name, err)
	}

	newID, err := st.ForkSession(src)
	if err != nil {
		t.Fatal(err)
	}
	if newID == src {
		t.Fatal("fork should create new id")
	}
}

func TestDenialLogAndMemoryInjectToggle(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.InsertDenial("bash", `{"command":"rm -rf /"}`, "permission rule"); err != nil {
		t.Fatal(err)
	}
	entries, err := st.ListDenials(5)
	if err != nil || len(entries) != 1 {
		t.Fatalf("ListDenials = %d entries, err = %v", len(entries), err)
	}

	enabled, err := st.MemoryInjectEnabled()
	if err != nil || !enabled {
		t.Fatalf("MemoryInjectEnabled = %v, err = %v", enabled, err)
	}
	if err := st.SetMemoryInjectEnabled(false); err != nil {
		t.Fatal(err)
	}
	enabled, err = st.MemoryInjectEnabled()
	if err != nil || enabled {
		t.Fatalf("after disable MemoryInjectEnabled = %v", enabled)
	}
}

func TestExportMessages(t *testing.T) {
	text := ExportMessages(nil)
	if text != "" {
		t.Fatalf("expected empty export, got %q", text)
	}
}
