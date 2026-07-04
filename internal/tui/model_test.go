package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/testutil"
)

func TestModelShiftTabCyclesApproval(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	policy, err := approval.New(approval.ModePlan)
	if err != nil {
		t.Fatal(err)
	}
	ag, err := agent.New(root, mock, agent.Options{Policy: policy})
	if err != nil {
		t.Fatal(err)
	}

	m := NewModel(Config{
		ProjectRoot: root,
		Agent:       ag,
		Policy:      policy,
		Sandbox:     "workspace-write",
		ContextLimit: 200000,
	})
	m.activePage = PageChat

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m2 := next.(Model)
	if m2.status.Approval != approval.ModeAskBeforeEdit {
		t.Fatalf("approval = %q", m2.status.Approval)
	}
	if ag.ApprovalPolicy().Mode() != approval.ModeAskBeforeEdit {
		t.Fatalf("agent policy = %q", ag.ApprovalPolicy().Mode())
	}
}

func TestModelConfirmKeys(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	policy, _ := approval.New(approval.ModeAskBeforeEdit)
	ag, _ := agent.New(root, mock, agent.Options{Policy: policy})

	m := NewModel(Config{ProjectRoot: root, Agent: ag, Policy: policy})
	resp := make(chan bool, 1)
	next, _ := m.Update(confirmRequestMsg{
		toolName: "write_file",
		input:    map[string]any{"path": "x.go"},
		resp:     resp,
	})
	m2 := next.(Model)
	if m2.confirm == nil {
		t.Fatal("expected confirm state")
	}

	next, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m3 := next.(Model)
	if m3.confirm != nil {
		t.Fatal("confirm should be cleared")
	}
	select {
	case ok := <-resp:
		if !ok {
			t.Fatal("expected confirm true")
		}
	default:
		t.Fatal("expected response on channel")
	}
}

func TestModelConfirmRejectEsc(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	policy, _ := approval.New(approval.ModeAskBeforeEdit)
	ag, _ := agent.New(root, mock, agent.Options{Policy: policy})

	m := NewModel(Config{ProjectRoot: root, Agent: ag, Policy: policy})
	resp := make(chan bool, 1)
	next, _ := m.Update(confirmRequestMsg{
		toolName: "bash",
		input:    map[string]any{"command": "go test ./..."},
		resp:     resp,
	})
	m2 := next.(Model)

	next, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := next.(Model)
	if m3.confirm != nil {
		t.Fatal("confirm should be cleared on esc")
	}
	select {
	case ok := <-resp:
		if ok {
			t.Fatal("expected confirm false")
		}
	default:
		t.Fatal("expected response on channel")
	}
}

func TestApplySlashSetMode(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	policy, _ := approval.New(approval.ModeAskBeforeEdit)
	ag, _ := agent.New(root, mock, agent.Options{Policy: policy})

	m := NewModel(Config{ProjectRoot: root, Agent: ag, Policy: policy})
	m2, _ := m.applySlash(dispatchSlash("/permissions plan", nil))
	if m2.status.Approval != approval.ModePlan {
		t.Fatalf("approval = %q", m2.status.Approval)
	}
}

func TestRebuildChatFromMessages(t *testing.T) {
	lines := rebuildChatFromMessages([]llm.Message{
		{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{{Type: "text", Text: "hello"}},
		},
	})
	if len(lines) != 1 || lines[0].Kind != LineUser || lines[0].Text != "hello" {
		t.Fatalf("lines = %+v", lines)
	}
}

func TestLoadRulesDisplayProject(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	content := "rules:\n  - action: allow\n    pattern: \"go *\"\n"
	if err := os.WriteFile(filepath.Join(root, ".golem", "rules.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := LoadRulesDisplay(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range lines {
		if strings.Contains(l, "allow:") && strings.Contains(l, "go *") {
			found = true
		}
	}
	if !found {
		t.Fatalf("lines = %v", lines)
	}
}
