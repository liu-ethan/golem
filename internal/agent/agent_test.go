package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/llm"
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

func textEvents(text string) []llm.StreamEvent {
	return []llm.StreamEvent{
		{Type: llm.StreamEventTextDelta, Text: text},
		{Type: llm.StreamEventMessageEnd, Usage: llm.Usage{InputTokens: 8, OutputTokens: 3}},
	}
}

// TestAgentToolChainReadEditVerify 验证 read_file → edit_file → 文本回复的完整工具链。
func TestAgentToolChainReadEditVerify(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	target := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(target, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: toolUseEvents("tu_read", "read_file", `{"path":"hello.txt"}`)},
		{Events: toolUseEvents("tu_edit", "edit_file", `{"path":"hello.txt","old_string":"world","new_string":"golem"}`)},
		{Events: textEvents("已读取并修改 hello.txt，内容为 hello golem。")},
	}

	ag, err := New(root, mock, Options{Gate: AllowAllGate{}})
	if err != nil {
		t.Fatal(err)
	}

	var toolNames []string
	_, err = ag.HandleInput(context.Background(), "读 hello.txt，把 world 改成 golem，再确认结果", func(evt Event) {
		if evt.Type == EventToolComplete {
			toolNames = append(toolNames, evt.ToolName)
		}
	})
	if err != nil {
		t.Fatalf("HandleInput: %v", err)
	}

	if len(toolNames) != 2 || toolNames[0] != "read_file" || toolNames[1] != "edit_file" {
		t.Fatalf("tool execution order = %v", toolNames)
	}
	if len(mock.StreamCalls) != 3 {
		t.Fatalf("StreamChat calls = %d, want 3", len(mock.StreamCalls))
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello golem\n" {
		t.Errorf("file content = %q", string(data))
	}

	msgs := ag.Messages()
	if len(msgs) < 5 {
		t.Fatalf("messages count = %d", len(msgs))
	}
	last := msgs[len(msgs)-1]
	if last.Role != llm.RoleAssistant {
		t.Fatalf("last role = %s", last.Role)
	}
	if last.Content[0].Text != "已读取并修改 hello.txt，内容为 hello golem。" {
		t.Errorf("final text = %q", last.Content[0].Text)
	}
}

// TestMemoryInjectedBeforeFirstStreamChat 验证 BM25 块在首次 StreamChat 之前写入 system prompt。
func TestMemoryInjectedBeforeFirstStreamChat(t *testing.T) {
	root := testutil.TempProjectRoot(t)

	memory := stubMemory{block: "\n\n## 相关记忆\n1. 用户偏好 tabs 缩进\n"}
	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: textEvents("ok")},
	}

	ag, err := New(root, mock, Options{Memory: memory})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ag.SystemPrompt(), "相关记忆") {
		t.Error("memory block should not be in system prompt before first user message")
	}

	_, err = ag.HandleInput(context.Background(), "帮我改个文件", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ag.MemoryInjected() {
		t.Error("memoryInjected should be true")
	}
	if !strings.Contains(ag.SystemPrompt(), "相关记忆") {
		t.Error("system prompt should contain injected memory after first message")
	}
	if len(mock.StreamCalls) != 1 {
		t.Fatal("expected one StreamChat call")
	}
	if !strings.Contains(mock.StreamCalls[0].System, "相关记忆") {
		t.Errorf("first StreamChat system = %q", mock.StreamCalls[0].System)
	}
}

// TestPlanModeDeniesWrite 验证 plan 模式（DenyWriteGate）直接拒绝写操作，不执行工具。
func TestPlanModeDeniesWrite(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: toolUseEvents("tu_write", "write_file", `{"path":"x.txt","content":"bad"}`)},
		{Events: textEvents("写操作在 plan 模式下被拒绝。")},
	}

	ag, err := New(root, mock, Options{Gate: DenyWriteGate{}})
	if err != nil {
		t.Fatal(err)
	}

	var toolErr bool
	_, err = ag.HandleInput(context.Background(), "写入文件", func(evt Event) {
		if evt.Type == EventToolComplete && evt.ToolError {
			toolErr = true
			if !strings.Contains(evt.ToolOutput, "denied") {
				t.Errorf("tool output = %q", evt.ToolOutput)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !toolErr {
		t.Error("expected denied tool result")
	}
	if _, err := os.Stat(filepath.Join(root, "x.txt")); !os.IsNotExist(err) {
		t.Error("write_file should not have created file under plan mode")
	}
}

// TestUserDeniedToolExecution 验证用户拒绝确认时 LLM 收到 denied tool_result。
func TestUserDeniedToolExecution(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: toolUseEvents("tu_write", "write_file", `{"path":"nope.txt","content":"x"}`)},
		{Events: textEvents("明白，未写入文件。")},
	}

	ag, err := New(root, mock, Options{
		Gate: ConfirmAllGate{},
		Confirm: func(_ string, _ map[string]any) (bool, error) {
			return false, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ag.HandleInput(context.Background(), "写个文件", nil)
	if err != nil {
		t.Fatal(err)
	}

	msgs := ag.Messages()
	var foundDenied bool
	for _, msg := range msgs {
		for _, block := range msg.Content {
			if block.Type == "tool_result" && strings.Contains(block.Content, "user denied") {
				foundDenied = true
			}
		}
	}
	if !foundDenied {
		t.Error("expected tool_result with user denied message")
	}
}

// TestBuildBaseSystemPromptIncludesProfile 验证 user_profile.md 在会话 init 时注入 system prompt。
func TestBuildBaseSystemPromptIncludesProfile(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	profile := "# 用户画像\n- 偏好 Go\n"
	if err := os.WriteFile(filepath.Join(root, ".golem", "user_profile.md"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{{Events: textEvents("hi")}}

	ag, err := New(root, mock, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ag.SystemPrompt(), "用户画像") {
		t.Errorf("system prompt = %q", ag.SystemPrompt())
	}
	if strings.Contains(ag.SystemPrompt(), "相关记忆") {
		t.Error("BM25 block should not be present at init")
	}
}

type stubMemory struct {
	block string
}

func (s stubMemory) InjectOnce(_ string) (string, error) {
	return s.block, nil
}
