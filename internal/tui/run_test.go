package tui

import (
	"testing"

	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/testutil"
)

type stubSessionEndHandler struct {
	calls int
}

func (s *stubSessionEndHandler) OnSessionEnd(_ string, _ bool) {
	s.calls++
}

func TestQuitDoesNotCallOnSessionEnd(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	policy, err := approval.New(approval.ModePlan)
	if err != nil {
		t.Fatal(err)
	}
	stub := &stubSessionEndHandler{}
	ag, err := agent.New(root, mock, agent.Options{
		Policy:    policy,
		OnSession: stub,
	})
	if err != nil {
		t.Fatal(err)
	}

	m := NewModel(Config{ProjectRoot: root, Agent: ag, Policy: policy})
	m.activePage = PageChat
	next, cmd := m.applySlash("/exit", dispatchSlash("/exit", nil))
	_ = next
	if cmd == nil {
		t.Fatal("expected tea.Quit command")
	}
	if stub.calls != 0 {
		t.Fatalf("OnSessionEnd during quit = %d, want 0", stub.calls)
	}
}

func TestFinalizeSessionCallsOnSessionEnd(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	policy, err := approval.New(approval.ModePlan)
	if err != nil {
		t.Fatal(err)
	}
	stub := &stubSessionEndHandler{}
	ag, err := agent.New(root, mock, agent.Options{
		Policy:    policy,
		OnSession: stub,
	})
	if err != nil {
		t.Fatal(err)
	}

	finalizeSession(Config{ProjectRoot: root, Agent: ag, Policy: policy})
	if stub.calls != 1 {
		t.Fatalf("OnSessionEnd after finalize = %d, want 1", stub.calls)
	}
}
