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

func TestHandleChatKeyAllowsTypingWhileRunning(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.running = true

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h', 'i'}})
	m2 := next.(Model)
	if m2.input != "hi" {
		t.Fatalf("input = %q, want typing while running", m2.input)
	}
}

func TestHandleChatKeyEnterQueuesWhileRunning(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.running = true
	m.input = "next question"

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := next.(Model)
	if m2.input != "" {
		t.Fatalf("input should clear after queue, got %q", m2.input)
	}
	if len(m2.inputQueue) != 1 || m2.inputQueue[0] != "next question" {
		t.Fatalf("inputQueue = %v", m2.inputQueue)
	}
}

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

func TestSubmitInputShowsUserLineForSlash(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.input = "/help"

	m2, _ := m.submitInput()
	if len(m2.lines) < 2 {
		t.Fatalf("lines = %+v, want user + system", m2.lines)
	}
	if m2.lines[0].Kind != LineUser || m2.lines[0].Text != "/help" {
		t.Fatalf("first line = %+v", m2.lines[0])
	}
	if m2.lines[1].Kind != LineSystem {
		t.Fatalf("second line = %+v", m2.lines[1])
	}

	out := renderChatArea(m2, 120, 20)
	if !strings.Contains(out, "You") {
		t.Fatal("expected user label")
	}
	if !strings.Contains(stripVisible(out), "/help") {
		t.Fatal("expected /help in chat area")
	}
}

func TestSubmitInputShowsUserLineForChat(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.input = "hello"

	m2, _ := m.submitInput()
	if len(m2.lines) != 1 || m2.lines[0].Kind != LineUser || m2.lines[0].Text != "hello" {
		t.Fatalf("lines = %+v", m2.lines)
	}
}

func TestApplySlashSetMode(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	policy, _ := approval.New(approval.ModeAskBeforeEdit)
	ag, _ := agent.New(root, mock, agent.Options{Policy: policy})

	m := NewModel(Config{ProjectRoot: root, Agent: ag, Policy: policy})
	m2, _ := m.applySlash("/permissions plan", dispatchSlash("/permissions plan", nil))
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
