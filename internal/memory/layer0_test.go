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

func TestMaybeCompactSkipsWhenNeitherTrigger(t *testing.T) {
	mock := testutil.NewMockLLM()
	mock.CompleteText = "should not run"
	store := &stubSummaryStore{}
	msgs := makeMessages(8)

	result, err := MaybeCompact(
		context.Background(), "sess-1", msgs, 50, 100,
		config.MemoryConfig{CompactBatchSize: 10, CompactThreshold: 0.8},
		mock, store, false, "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Compacted {
		t.Fatal("expected no compaction when neither token nor message threshold met")
	}
	if len(mock.CompleteCalls) != 0 {
		t.Fatalf("Complete calls = %d, want 0", len(mock.CompleteCalls))
	}
}

func TestMaybeCompactTriggersOnMessageCount(t *testing.T) {
	mock := testutil.NewMockLLM()
	mock.CompleteText = "count trigger summary"
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
	if !result.Compacted {
		t.Fatal("expected compaction when message count exceeds batch size")
	}
	if result.CompactedCount != 10 {
		t.Errorf("CompactedCount = %d, want 10", result.CompactedCount)
	}
}

func TestMaybeCompactTriggersOnTokenThresholdWithSmallHistory(t *testing.T) {
	mock := testutil.NewMockLLM()
	mock.CompleteText = "token trigger summary"
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
	if !result.Compacted {
		t.Fatal("expected compaction when token threshold met")
	}
	if len(result.Messages) != 1 {
		t.Fatalf("messages = %d, want 1 summary", len(result.Messages))
	}
	if result.CompactedCount != 8 {
		t.Errorf("CompactedCount = %d, want 8", result.CompactedCount)
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

func TestMaybeCompactForceReplacesNewestBatch(t *testing.T) {
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
	if len(result.Messages) != 3 {
		t.Fatalf("messages = %d, want 3 (2 kept + 1 summary)", len(result.Messages))
	}
	if !IsSummaryMessage(result.Messages[2]) {
		t.Fatal("summary should be appended after kept prefix")
	}
	if !strings.Contains(mock.CompleteCalls[0].System, "保留文件路径") {
		t.Errorf("system prompt = %q", mock.CompleteCalls[0].System)
	}
	if len(mock.CompleteCalls[0].Messages) != 10 {
		t.Errorf("Complete batch size = %d, want 10", len(mock.CompleteCalls[0].Messages))
	}
}

func TestMaybeCompactForceCompressesAllWhenWithinBatchSize(t *testing.T) {
	mock := testutil.NewMockLLM()
	mock.CompleteText = "all summary"
	store := &stubSummaryStore{}
	msgs := makeMessages(5)

	result, err := MaybeCompact(
		context.Background(), "sess-1", msgs, 1, 100,
		config.MemoryConfig{CompactBatchSize: 10, CompactThreshold: 0.8},
		mock, store, true, "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Compacted {
		t.Fatal("expected forced compaction")
	}
	if len(result.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(result.Messages))
	}
	if result.CompactedCount != 5 {
		t.Errorf("CompactedCount = %d, want 5", result.CompactedCount)
	}
}

func TestSummaryMessageFormat(t *testing.T) {
	msg := SummaryMessage("hello")
	if msg.Content[0].Text != SummaryMessagePrefix+"\nhello" {
		t.Errorf("text = %q", msg.Content[0].Text)
	}
}
