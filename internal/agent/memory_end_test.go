package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/testutil"
)

func TestMemoryOnEndExtractsFacts(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := session.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	mock := testutil.NewMockLLM()
	payload, _ := json.Marshal([]map[string]string{
		{"content": "用户偏好 tabs", "category": "preference"},
	})
	mock.CompleteText = string(payload)
	mock.StreamResponses = []testutil.MockResponse{
		{Events: []llm.StreamEvent{
			{Type: llm.StreamEventTextDelta, Text: "ok"},
			{Type: llm.StreamEventMessageEnd},
		}},
	}

	sessionID := uuid.NewString()
	var src sessionLazySource
	policy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	ag, err := New(root, mock, Options{
		SessionID: sessionID,
		Policy:    policy,
		OnSession: ChainEndHandler{
			session.PersistOnEnd{Store: st, Source: &src},
			MemoryOnEnd{
				Store:       st,
				Source:      &src,
				ProjectRoot: root,
				MemoryCfg:   config.MemoryConfig{Layer2SessionThreshold: 3},
				LLM:         mock,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	src.ag = ag

	if _, err := ag.HandleInput(context.Background(), "我用 tabs", nil); err != nil {
		t.Fatal(err)
	}
	ag.OnSessionEnd()

	facts, err := st.ListMemoryFacts()
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 {
		t.Fatalf("facts = %d, want 1", len(facts))
	}
	if facts[0].Content != "用户偏好 tabs" {
		t.Errorf("content = %q", facts[0].Content)
	}

	count, err := st.IncrementSessionCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count after OnSessionEnd + manual increment = %d, want 2", count)
	}
}

type sessionLazySource struct {
	ag *Agent
}

func (s *sessionLazySource) SessionID() string {
	if s.ag == nil {
		return ""
	}
	return s.ag.SessionID()
}

func (s *sessionLazySource) Messages() []llm.Message {
	if s.ag == nil {
		return nil
	}
	return s.ag.Messages()
}
