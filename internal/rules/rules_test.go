package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tencent-docs/golem/internal/testutil"
)

func TestMatchBashGoTest(t *testing.T) {
	rules := []Rule{{Action: "allow", Pattern: "go *"}}
	if got := MatchBash("go test ./...", rules); got != ActionAllow {
		t.Errorf("MatchBash = %q, want allow", got)
	}
}

func TestMatchBashDenyRmRf(t *testing.T) {
	rules := []Rule{{Action: "deny", Pattern: "rm -rf *"}}
	if got := MatchBash("rm -rf /tmp/x", rules); got != ActionDeny {
		t.Errorf("MatchBash = %q, want deny", got)
	}
}

func TestMatchBashDefaultAllow(t *testing.T) {
	rules := []Rule{{Action: "allow", Pattern: "go *"}}
	if got := MatchBash("curl https://example.com", rules); got != ActionAllow {
		t.Errorf("MatchBash = %q, want allow (no match)", got)
	}
	if got := MatchBash("curl https://example.com", nil); got != ActionAllow {
		t.Errorf("MatchBash with nil rules = %q, want allow", got)
	}
}

func TestMatchBashPriorityDenyOverAsk(t *testing.T) {
	rules := []Rule{
		{Action: "ask", Pattern: "go *"},
		{Action: "deny", Pattern: "go test *"},
	}
	if got := MatchBash("go test ./...", rules); got != ActionDeny {
		t.Errorf("MatchBash = %q, want deny", got)
	}
}

func TestMatchBashPriorityAskOverAllow(t *testing.T) {
	rules := []Rule{
		{Action: "allow", Pattern: "curl *"},
		{Action: "ask", Pattern: "curl https://*"},
	}
	if got := MatchBash("curl https://example.com", rules); got != ActionAsk {
		t.Errorf("MatchBash = %q, want ask", got)
	}
}

func TestLoadProjectAndGlobal(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	projectRules := "rules:\n  - action: allow\n    pattern: \"go *\"\n"
	if err := os.WriteFile(filepath.Join(root, ".golem", "rules.yaml"), []byte(projectRules), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	globalDir := filepath.Join(home, ".golem")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	globalRules := "rules:\n  - action: deny\n    pattern: \"rm -rf *\"\n"
	if err := os.WriteFile(filepath.Join(globalDir, "rules.yaml"), []byte(globalRules), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("rules count = %d, want 2", len(loaded))
	}
	if loaded[0].Pattern != "go *" || loaded[1].Pattern != "rm -rf *" {
		t.Fatalf("rules order = %+v", loaded)
	}
}

func TestLoadMissingFiles(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 {
		t.Fatalf("rules = %+v, want empty", loaded)
	}
}
