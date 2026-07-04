package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/testutil"
)

func toolUseEvents(id, name, inputJSON string) []llm.StreamEvent {
	var input map[string]any
	_ = json.Unmarshal([]byte(inputJSON), &input)
	return []llm.StreamEvent{
		{Type: llm.StreamEventToolUseStart, ToolUseID: id, ToolName: name},
		{Type: llm.StreamEventToolUseInputDelta, InputDelta: inputJSON},
		{Type: llm.StreamEventToolUseInputDelta, ToolInput: input},
		{Type: llm.StreamEventMessageEnd, Usage: llm.Usage{InputTokens: 10, OutputTokens: 5}},
	}
}

// TestAgentLoopRoundTrip 验证 mock LLM 驱动的 read → edit → 回复完整 round-trip，并持久化到 SQLite。
func TestAgentLoopRoundTrip(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	target := filepath.Join(root, "note.txt")
	if err := os.WriteFile(target, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := session.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: toolUseEvents("tu_read", "read_file", `{"path":"note.txt"}`)},
		{Events: toolUseEvents("tu_edit", "edit_file", `{"path":"note.txt","old_string":"world","new_string":"golem"}`)},
		textStreamResponse("已修改 note.txt。"),
	}

	sessionID := newSessionID()
	ag, _ := newProductionAgent(t, root, st, mock, sessionID)

	var toolNames []string
	if _, err := ag.HandleInput(context.Background(), "读 note.txt 并把 world 改成 golem", func(evt agent.Event) {
		if evt.Type == agent.EventToolComplete {
			toolNames = append(toolNames, evt.ToolName)
		}
	}); err != nil {
		t.Fatal(err)
	}
	ag.OnSessionEnd()

	if len(toolNames) != 2 || toolNames[0] != "read_file" || toolNames[1] != "edit_file" {
		t.Fatalf("tool order = %v", toolNames)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello golem\n" {
		t.Errorf("file = %q", string(data))
	}

	summary, loaded, err := st.LoadSession(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if summary != "" {
		t.Errorf("unexpected summary %q", summary)
	}
	if len(loaded) < 2 {
		t.Fatalf("persisted messages = %d", len(loaded))
	}
	if loaded[0].Role != llm.RoleUser {
		t.Errorf("first role = %s", loaded[0].Role)
	}
}
