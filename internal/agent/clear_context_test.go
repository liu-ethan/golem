package agent

import (
	"sync/atomic"
	"testing"

	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/testutil"
)

type blockingSessionEndHandler struct {
	calls atomic.Int32
}

func (h *blockingSessionEndHandler) OnSessionEnd(_ string, _ bool) {
	h.calls.Add(1)
}

func (h *blockingSessionEndHandler) OnSessionEndSnapshot(snap SessionEndSnapshot) {
	h.calls.Add(1)
	if snap.SessionID == "" {
		return
	}
}

func TestClearContextDoesNotCallOnSessionEnd(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	client := testutil.NewMockLLM()
	handler := &blockingSessionEndHandler{}
	ag, err := New(root, client, Options{OnSession: handler})
	if err != nil {
		t.Fatal(err)
	}
	ag.messages = append(ag.messages, llm.Message{
		Role: llm.RoleUser,
		Content: []llm.ContentBlock{{
			Type: "text",
			Text: "hello",
		}},
	})
	ag.hadUserMessages = true
	oldID := ag.SessionID()

	newID, snap := ag.ClearContext()
	if handler.calls.Load() != 0 {
		t.Fatalf("OnSessionEnd during ClearContext = %d, want 0", handler.calls.Load())
	}
	if newID == "" || newID == oldID {
		t.Fatalf("new session id = %q, old = %q", newID, oldID)
	}
	if snap.SessionID != oldID {
		t.Fatalf("snapshot session = %q, want %q", snap.SessionID, oldID)
	}
	if !snap.HadUserMessages || len(snap.Messages) != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if len(ag.Messages()) != 0 {
		t.Fatal("messages should be cleared immediately")
	}

	ag.OnSessionEndSnapshot(snap)
	if handler.calls.Load() != 1 {
		t.Fatalf("OnSessionEndSnapshot calls = %d, want 1", handler.calls.Load())
	}
}
