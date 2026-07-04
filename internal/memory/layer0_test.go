package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/testutil"
)

type stubSummaryStore struct {
	summary string
}

func (s *stubSummaryStore) UpdateSummary(_ string, summary string) error {
	s.summary = summary
	return nil
}

func makeMessages(n int) []llm.Message {
	msgs := make([]llm.Message, n)
	for i := range msgs {
		role := llm.RoleUser
		if i%2 == 1 {
			role = llm.RoleAssistant
		}
		msgs[i] = llm.Message{
			Role: role,
			Content: []llm.ContentBlock{{
				Type: "text",
				Text: strings.Repeat("x", i+1),
			}},
		}
	}
	return msgs
}

func TestMaybeCompactSkipsBelowThreshold(t *testing.T) {
	mock := testutil.NewMockLLM()
	mock.CompleteText = "should not run"
	store := &stubSummaryStore{}
	msgs := makeMessages(15)

	result, err := MaybeCompact(
		context.Background(), "sess-1", msgs, 50, 100,
		config.MemoryConfig{CompactBatchSize: 10, CompactThreshold: 0.8},
		mock, store, false, "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Compacted {
		t.Fatal("expected no compaction below threshold")
	}
	if len(mock.CompleteCalls) != 0 {
		t.Fatalf("Complete calls = %d, want 0", len(mock.CompleteCalls))
	}
}

func TestMaybeCompactSkipsWhenTooFewMessages(t *testing.T) {
	mock := testutil.NewMockLLM()
	store := &stubSummaryStore{}
	msgs := makeMessages(8)

	result, err := MaybeCompact(
		context.Background(), "sess-1", msgs, 90, 100,
		config.MemoryConfig{CompactBatchSize: 10, CompactThreshold: 0.8},
		mock, store, false, "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Compacted {
		t.Fatal("expected skip when message count <= batch size")
	}
}

func TestMaybeCompactReplacesOldestBatch(t *testing.T) {
	mock := testutil.NewMockLLM()
	mock.CompleteText = "用户讨论了多项主题。"
	store := &stubSummaryStore{}
	msgs := makeMessages(15)

	result, err := MaybeCompact(
		context.Background(), "sess-1", msgs, 90, 100,
		config.MemoryConfig{CompactBatchSize: 10, CompactThreshold: 0.8},
		mock, store, false, "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Compacted {
		t.Fatal("expected compaction")
	}
	if len(result.Messages) != 6 {
		t.Fatalf("messages = %d, want 6 (1 summary + 5 remaining)", len(result.Messages))
	}
	if !IsSummaryMessage(result.Messages[0]) {
		t.Fatalf("first message should be summary, got %+v", result.Messages[0])
	}
	if store.summary != "用户讨论了多项主题。" {
		t.Errorf("stored summary = %q", store.summary)
	}
	if len(mock.CompleteCalls) != 1 {
		t.Fatalf("Complete calls = %d", len(mock.CompleteCalls))
	}
	if len(mock.CompleteCalls[0].Messages) != 10 {
		t.Errorf("Complete batch size = %d, want 10", len(mock.CompleteCalls[0].Messages))
	}
}

func TestMaybeCompactForceBypassesThreshold(t *testing.T) {
	mock := testutil.NewMockLLM()
	mock.CompleteText = "forced summary"
	store := &stubSummaryStore{}
	msgs := makeMessages(12)

	result, err := MaybeCompact(
		context.Background(), "sess-1", msgs, 1, 100,
		config.MemoryConfig{CompactBatchSize: 10, CompactThreshold: 0.8},
		mock, store, true, "保留文件路径",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Compacted {
		t.Fatal("expected forced compaction")
	}
	if !strings.Contains(mock.CompleteCalls[0].System, "保留文件路径") {
		t.Errorf("system prompt = %q", mock.CompleteCalls[0].System)
	}
}

func TestSummaryMessageFormat(t *testing.T) {
	msg := SummaryMessage("hello")
	if msg.Content[0].Text != SummaryMessagePrefix+"\nhello" {
		t.Errorf("text = %q", msg.Content[0].Text)
	}
}
