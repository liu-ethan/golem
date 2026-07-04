package memory

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/testutil"
)

type stubFactStore struct {
	projectID    string
	facts        []MemoryFact
	sessionID    string
	count        int
	insertErr    error
	incrementErr error
}

func (s *stubFactStore) ProjectIDValue() string { return s.projectID }

func (s *stubFactStore) InsertMemoryFacts(sessionID string, facts []MemoryFact) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	s.sessionID = sessionID
	s.facts = append([]MemoryFact(nil), facts...)
	return nil
}

func (s *stubFactStore) IncrementSessionCount() (int, error) {
	if s.incrementErr != nil {
		return 0, s.incrementErr
	}
	s.count++
	return s.count, nil
}

func (s *stubFactStore) ListMemoryFacts() ([]MemoryFact, error) {
	return append([]MemoryFact(nil), s.facts...), nil
}

func (s *stubFactStore) SessionCount() (int, error) {
	return s.count, nil
}

func (s *stubFactStore) DeleteAllFacts() error {
	s.facts = nil
	return nil
}
func (s *stubFactStore) ResetSessionCount() error { s.count = 0; return nil }

func TestParseExtractedFactsValidJSON(t *testing.T) {
	raw := `[
  {"content": "用户偏好 tabs 缩进", "category": "preference"},
  {"content": "golem 使用 SQLite", "category": "project_fact"}
]`
	facts, err := parseExtractedFacts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 2 {
		t.Fatalf("facts = %d, want 2", len(facts))
	}
	if facts[0].Category != categoryPreference {
		t.Errorf("category[0] = %q", facts[0].Category)
	}
}

func TestParseExtractedFactsStripsCodeFence(t *testing.T) {
	raw := "```json\n[{\"content\":\"任务进行中\",\"category\":\"task_progress\"}]\n```"
	facts, err := parseExtractedFacts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 {
		t.Fatalf("facts = %d, want 1", len(facts))
	}
}

func TestParseExtractedFactsSkipsInvalidCategory(t *testing.T) {
	raw := `[{"content": "有效", "category": "preference"}, {"content": "无效", "category": "other"}]`
	facts, err := parseExtractedFacts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 {
		t.Fatalf("facts = %d, want 1", len(facts))
	}
}

func TestParseExtractedFactsEmptyArray(t *testing.T) {
	facts, err := parseExtractedFacts("[]")
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 0 {
		t.Fatalf("facts = %d, want 0", len(facts))
	}
}

func TestFormatConversationIncludesRoles(t *testing.T) {
	msgs := []llm.Message{
		{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{{
				Type: "text",
				Text: "hello",
			}},
		},
		{
			Role: llm.RoleAssistant,
			Content: []llm.ContentBlock{{
				Type: "text",
				Text: "world",
			}},
		},
	}
	got := formatConversation(msgs)
	if !strings.Contains(got, "user: hello") {
		t.Errorf("got = %q", got)
	}
	if !strings.Contains(got, "assistant: world") {
		t.Errorf("got = %q", got)
	}
}

func TestOnSessionEndExtractsAndStoresFacts(t *testing.T) {
	mock := testutil.NewMockLLM()
	payload, _ := json.Marshal([]extractedFact{
		{Content: "用户偏好 tabs", Category: "preference"},
		{Content: "项目用 Go", Category: "project_fact"},
	})
	mock.CompleteText = string(payload)

	store := &stubFactStore{projectID: "proj-1"}
	msgs := []llm.Message{{
		Role: llm.RoleUser,
		Content: []llm.ContentBlock{{
			Type: "text",
			Text: "我用 tabs 缩进",
		}},
	}}

	err := OnSessionEnd(context.Background(), SessionEndParams{
		SessionID: "sess-1",
		Messages:  msgs,
		Config:    config.MemoryConfig{Layer2SessionThreshold: 3},
		LLM:       mock,
		Store:     store,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.facts) != 2 {
		t.Fatalf("stored facts = %d, want 2", len(store.facts))
	}
	if store.sessionID != "sess-1" {
		t.Errorf("sessionID = %q", store.sessionID)
	}
	if store.count != 1 {
		t.Errorf("session count = %d, want 1", store.count)
	}
	if len(mock.CompleteCalls) != 1 {
		t.Fatalf("Complete calls = %d", len(mock.CompleteCalls))
	}
}

func TestOnSessionEndSkipsWithoutMessages(t *testing.T) {
	mock := testutil.NewMockLLM()
	store := &stubFactStore{projectID: "proj-1"}

	err := OnSessionEnd(context.Background(), SessionEndParams{
		SessionID: "sess-1",
		LLM:       mock,
		Store:     store,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.CompleteCalls) != 0 {
		t.Fatalf("Complete calls = %d, want 0", len(mock.CompleteCalls))
	}
	if store.count != 0 {
		t.Errorf("count = %d, want 0", store.count)
	}
}

func TestOnSessionEndTriggersLayer2AtThreshold(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	store := &stubFactStore{
		projectID: "proj-1",
		count:     2,
	}
	layer1Payload, _ := json.Marshal([]extractedFact{
		{Content: "用户偏好 tabs", Category: "preference"},
	})
	mock.CompleteText = string(layer1Payload)

	err := OnSessionEnd(context.Background(), SessionEndParams{
		SessionID:   "sess-3",
		ProjectRoot: root,
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{{
				Type: "text",
				Text: "第三次会话",
			}},
		}},
		Config: config.MemoryConfig{Layer2SessionThreshold: 3},
		LLM:    mock,
		Store:  store,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.count != 0 {
		t.Errorf("count after layer2 = %d, want 0", store.count)
	}
	if len(store.facts) != 0 {
		t.Fatalf("facts after layer2 = %d, want 0", len(store.facts))
	}
	if len(mock.CompleteCalls) != 2 {
		t.Fatalf("Complete calls = %d, want 2 (layer1 + layer2)", len(mock.CompleteCalls))
	}
}
