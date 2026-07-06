package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/testutil"
)

func TestTruncateToolOutput(t *testing.T) {
	short := truncateToolOutput("ok")
	if short != "ok" {
		t.Fatalf("short = %q", short)
	}
	long := strings.Repeat("x", maxToolResultBytes+10)
	out := truncateToolOutput(long)
	if len(out) <= maxToolResultBytes {
		t.Fatalf("expected truncated output > %d, got %d", maxToolResultBytes, len(out))
	}
	if !strings.Contains(out, "truncated tool output") {
		t.Fatalf("output = %q", out[:80])
	}
}

// multiToolUseEvents 构造一条 assistant 消息中包含多个 tool_use 的 SSE 事件序列。
func multiToolUseEvents(id1, name1, inputJSON1, id2, name2, inputJSON2 string) []llm.StreamEvent {
	var input1, input2 map[string]any
	_ = json.Unmarshal([]byte(inputJSON1), &input1)
	_ = json.Unmarshal([]byte(inputJSON2), &input2)
	return []llm.StreamEvent{
		{Type: llm.StreamEventTextDelta, Text: "我先读两个文件。"},
		{Type: llm.StreamEventToolUseStart, ToolUseID: id1, ToolName: name1},
		{Type: llm.StreamEventToolUseInputDelta, InputDelta: inputJSON1},
		{Type: llm.StreamEventToolUseInputDelta, ToolInput: input1},
		{Type: llm.StreamEventToolUseStart, ToolUseID: id2, ToolName: name2},
		{Type: llm.StreamEventToolUseInputDelta, InputDelta: inputJSON2},
		{Type: llm.StreamEventToolUseInputDelta, ToolInput: input2},
		{Type: llm.StreamEventMessageEnd, Usage: llm.Usage{InputTokens: 10, OutputTokens: 5}},
	}
}

// TestMultipleToolUseSingleUserMessage 验证同一条 assistant 消息中的多个 tool_use
// 对应的 tool_result 被合并到同一条 user 消息中，而非分散为多条 user 消息。
// Anthropic Messages API 要求所有 tool_result 必须紧跟在 assistant 消息之后的同一条 user 消息里。
func TestMultipleToolUseSingleUserMessage(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("B\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: multiToolUseEvents(
			"tu_1", "read_file", `{"path":"a.txt"}`,
			"tu_2", "read_file", `{"path":"b.txt"}`,
		)},
		{Events: textEvents("两个文件都读完了。")},
	}

	policy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	ag, err := New(root, mock, Options{Policy: policy})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ag.HandleInput(context.Background(), "读 a.txt 和 b.txt", nil)
	if err != nil {
		t.Fatalf("HandleInput: %v", err)
	}

	// 第二次 StreamChat 调用时应包含 4 条消息：
	//   0: user query
	//   1: assistant (text + tool_use_1 + tool_use_2)
	//   2: user (tool_result_1 + tool_result_2)  ← 必须合并为一条
	// 如果 tool_result 被拆成两条 user 消息，API 会拒绝。
	if len(mock.StreamCalls) < 2 {
		t.Fatalf("StreamChat calls = %d, want >= 2", len(mock.StreamCalls))
	}
	secondReq := mock.StreamCalls[1]
	if len(secondReq.Messages) != 3 {
		t.Fatalf("second StreamChat messages = %d, want 3 (user/assistant/user)", len(secondReq.Messages))
	}

	// 第二条消息必须是 assistant，包含两个 tool_use
	asst := secondReq.Messages[1]
	if asst.Role != llm.RoleAssistant {
		t.Fatalf("messages[1].role = %s, want assistant", asst.Role)
	}
	toolUseCount := 0
	for _, block := range asst.Content {
		if block.Type == "tool_use" {
			toolUseCount++
		}
	}
	if toolUseCount != 2 {
		t.Fatalf("assistant tool_use count = %d, want 2", toolUseCount)
	}

	// 第三条消息必须是 user，包含两个 tool_result
	result := secondReq.Messages[2]
	if result.Role != llm.RoleUser {
		t.Fatalf("messages[2].role = %s, want user", result.Role)
	}
	toolResultCount := 0
	for _, block := range result.Content {
		if block.Type == "tool_result" {
			toolResultCount++
		}
	}
	if toolResultCount != 2 {
		t.Fatalf("user tool_result count = %d, want 2 (all results in one message)", toolResultCount)
	}
}
