package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/memory"
	"github.com/tencent-docs/golem/internal/rules"
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

	policy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	ag, err := New(root, mock, Options{Policy: policy})
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

// TestPlanModeDeniesWrite 验证 plan 模式直接拒绝写操作，不执行工具、不弹确认框。
func TestPlanModeDeniesWrite(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: toolUseEvents("tu_write", "write_file", `{"path":"x.txt","content":"bad"}`)},
		{Events: textEvents("写操作在 plan 模式下被拒绝。")},
	}

	planPolicy, err := approval.New(approval.ModePlan)
	if err != nil {
		t.Fatal(err)
	}
	confirmCalled := false
	ag, err := New(root, mock, Options{
		Policy: planPolicy,
		Confirm: func(_ string, _ map[string]any) (bool, error) {
			confirmCalled = true
			return true, nil
		},
	})
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
	if confirmCalled {
		t.Error("plan mode should deny without confirmation dialog")
	}
	if _, err := os.Stat(filepath.Join(root, "x.txt")); !os.IsNotExist(err) {
		t.Error("write_file should not have created file under plan mode")
	}
}

// TestAskBeforeEditRequiresConfirm 验证 ask-before-edit 模式下写操作需用户确认。
func TestAskBeforeEditRequiresConfirm(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: toolUseEvents("tu_write", "write_file", `{"path":"ok.txt","content":"yes"}`)},
		{Events: textEvents("已写入。")},
	}

	abPolicy, err := approval.New(approval.ModeAskBeforeEdit)
	if err != nil {
		t.Fatal(err)
	}
	confirmCalled := false
	ag, err := New(root, mock, Options{
		Policy: abPolicy,
		Confirm: func(_ string, _ map[string]any) (bool, error) {
			confirmCalled = true
			return true, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ag.HandleInput(context.Background(), "写个文件", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !confirmCalled {
		t.Error("ask-before-edit should invoke confirm for write_file")
	}
	if _, err := os.Stat(filepath.Join(root, "ok.txt")); err != nil {
		t.Errorf("write_file should succeed after confirm: %v", err)
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

	askPolicy, err := approval.New(approval.ModeAsk)
	if err != nil {
		t.Fatal(err)
	}
	ag, err := New(root, mock, Options{
		Policy: askPolicy,
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

// TestRulesDenyBashCommand 验证 rules.deny 直接拒绝 bash，不执行、不弹确认框。
func TestRulesDenyBashCommand(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: toolUseEvents("tu_bash", "bash", `{"command":"rm -rf /tmp/x"}`)},
		{Events: textEvents("命令被拒绝。")},
	}

	autoPolicy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	confirmCalled := false
	ag, err := New(root, mock, Options{
		Policy: autoPolicy,
		Rules: []rules.Rule{
			{Action: "deny", Pattern: "rm -rf *"},
		},
		Confirm: func(_ string, _ map[string]any) (bool, error) {
			confirmCalled = true
			return true, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var toolOutput string
	_, err = ag.HandleInput(context.Background(), "删除临时目录", func(evt Event) {
		if evt.Type == EventToolComplete && evt.ToolName == "bash" {
			toolOutput = evt.ToolOutput
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(toolOutput, "permission rule") {
		t.Errorf("tool output = %q", toolOutput)
	}
	if confirmCalled {
		t.Error("rules deny should not invoke confirm")
	}
}

// TestRulesAskRequiresConfirmInEditAutomatically 验证 edit-automatically 下 rules.ask 仍弹确认。
func TestRulesAskRequiresConfirmInEditAutomatically(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: toolUseEvents("tu_bash", "bash", `{"command":"echo ok"}`)},
		{Events: textEvents("已执行。")},
	}

	autoPolicy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	confirmCalled := false
	ag, err := New(root, mock, Options{
		Policy: autoPolicy,
		Rules: []rules.Rule{
			{Action: "ask", Pattern: "echo *"},
		},
		Confirm: func(_ string, _ map[string]any) (bool, error) {
			confirmCalled = true
			return true, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ag.HandleInput(context.Background(), "跑 echo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !confirmCalled {
		t.Error("rules ask should invoke confirm under edit-automatically")
	}
}


type stubMemory struct {
	block string
}

func (s stubMemory) InjectOnce(_ context.Context, _ string) (string, error) {
	return s.block, nil
}

// TestMemoryNotInjectedOnSecondMessage 验证同会话第二条消息不再追加 BM25 记忆块。
func TestMemoryNotInjectedOnSecondMessage(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	calls := 0
	mem := countingMemory{
		block: "\n\n## 相关记忆\n1. fact\n",
		onCall: func() {
			calls++
		},
	}
	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: textEvents("first")},
		{Events: textEvents("second")},
	}

	ag, err := New(root, mock, Options{Memory: mem})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ag.HandleInput(context.Background(), "第一条", nil); err != nil {
		t.Fatal(err)
	}
	promptAfterFirst := ag.SystemPrompt()
	if _, err := ag.HandleInput(context.Background(), "第二条", nil); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("InjectOnce calls = %d, want 1", calls)
	}
	if ag.SystemPrompt() != promptAfterFirst {
		t.Error("system prompt should not change on second message")
	}
}

// TestBM25MemoryProviderInjectsFromStore 验证首条 user 消息前从 SQLite 检索并注入记忆块。
func TestBM25MemoryProviderInjectsFromStore(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := session.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	sessionID := "bm25-inject"
	if err := st.InsertMemoryFacts(sessionID, []memory.MemoryFact{
		{Content: "用户偏好 tabs 缩进", Category: "preference"},
		{Content: "项目使用 Go 与 SQLite", Category: "project_fact"},
	}); err != nil {
		t.Fatal(err)
	}

	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{{Events: textEvents("ok")}}

	ag, err := New(root, mock, Options{
		Memory: BM25MemoryProvider{
			Store:     st,
			Retriever: memory.NewBM25Retriever(),
			TopK:      5,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ag.SystemPrompt(), "相关记忆") {
		t.Error("BM25 block should not be present before first user message")
	}

	_, err = ag.HandleInput(context.Background(), "帮我写 Go 代码", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ag.SystemPrompt(), "相关记忆") {
		t.Errorf("system prompt = %q", ag.SystemPrompt())
	}
	if !strings.Contains(ag.SystemPrompt(), "tabs") {
		t.Error("expected relevant fact in injected block")
	}
	if len(mock.StreamCalls) != 1 || !strings.Contains(mock.StreamCalls[0].System, "相关记忆") {
		t.Errorf("first StreamChat system = %q", mock.StreamCalls[0].System)
	}
}

type countingMemory struct {
	block  string
	onCall func()
}

func (c countingMemory) InjectOnce(_ context.Context, _ string) (string, error) {
	if c.onCall != nil {
		c.onCall()
	}
	return c.block, nil
}

func makeAgentMessages(n int) []llm.Message {
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
				Text: strings.Repeat("m", i+1),
			}},
		}
	}
	return msgs
}

// TestAgentCompactManual 验证 /compact 路径压缩最旧一批消息并写入 summary store。
func TestAgentCompactManual(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := session.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	mock := testutil.NewMockLLM()
	mock.CompleteText = "压缩摘要内容"
	policy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	sessionID := "compact-test"
	ag, err := New(root, mock, Options{
		SessionID: sessionID,
		Policy:    policy,
		MemoryCfg: config.MemoryConfig{CompactBatchSize: 10, CompactThreshold: 0.8},
		ContextLimit: 100,
		SummaryStore: st,
		InitialMsgs:  makeAgentMessages(15),
	})
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ag.Compact(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "已压缩") {
		t.Errorf("compact message = %q", msg)
	}
	if len(ag.Messages()) != 6 {
		t.Fatalf("messages = %d, want 6", len(ag.Messages()))
	}
	if !memory.IsSummaryMessage(ag.Messages()[0]) {
		t.Error("expected summary message at front")
	}
	summary, _, err := st.LoadSession(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if summary != "压缩摘要内容" {
		t.Errorf("summary = %q", summary)
	}
}

// TestRestoreStateSkipsDuplicateSummary 验证 resume 时不在已有 summary 消息前重复注入。
func TestRestoreStateSkipsDuplicateSummary(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	ag, err := New(root, mock, Options{})
	if err != nil {
		t.Fatal(err)
	}
	withSummary := []llm.Message{memory.SummaryMessage("已有摘要")}
	ag.RestoreState(withSummary, false, "已有摘要")
	if len(ag.Messages()) != 1 {
		t.Fatalf("messages = %d, want 1 without duplicate", len(ag.Messages()))
	}
}
