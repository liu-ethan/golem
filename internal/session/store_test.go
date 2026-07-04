package session

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/memory"
	"github.com/tencent-docs/golem/internal/testutil"
)

func TestProjectIDDeterministic(t *testing.T) {
	root := "/tmp/golem-test-project"
	id1 := ProjectID(root)
	id2 := ProjectID(root)
	if id1 != id2 {
		t.Fatalf("ProjectID not stable: %q vs %q", id1, id2)
	}
	if len(id1) != 16 {
		t.Fatalf("ProjectID length = %d, want 16", len(id1))
	}
}

func TestOpenCreatesDatabase(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	path := DBPath(root)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file missing: %v", err)
	}
}

func TestSyncAndLoadMessagesRoundTrip(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	sessionID := uuid.NewString()
	msgs := []llm.Message{
		{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{{
				Type: "text",
				Text: "读 hello.txt",
			}},
		},
		{
			Role: llm.RoleAssistant,
			Content: []llm.ContentBlock{
				{Type: "text", Text: "好的"},
				{
					Type:  "tool_use",
					ID:    "tu_1",
					Name:  "read_file",
					Input: map[string]any{"path": "hello.txt"},
				},
			},
		},
		{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{{
				Type:      "tool_result",
				ToolUseID: "tu_1",
				Content:   "hello world",
			}},
		},
	}

	if err := st.SyncMessages(sessionID, msgs); err != nil {
		t.Fatal(err)
	}

	summary, loaded, err := st.LoadSession(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if summary != "" {
		t.Errorf("summary = %q, want empty", summary)
	}
	if len(loaded) != len(msgs) {
		t.Fatalf("loaded count = %d, want %d", len(loaded), len(msgs))
	}
	for i := range msgs {
		if loaded[i].Role != msgs[i].Role {
			t.Errorf("msg[%d] role = %s, want %s", i, loaded[i].Role, msgs[i].Role)
		}
		if len(loaded[i].Content) != len(msgs[i].Content) {
			t.Fatalf("msg[%d] content blocks = %d, want %d", i, len(loaded[i].Content), len(msgs[i].Content))
		}
		if loaded[i].Content[0].Type == "text" && loaded[i].Content[0].Text != msgs[i].Content[0].Text {
			t.Errorf("msg[%d] text = %q", i, loaded[i].Content[0].Text)
		}
	}
}

func TestUpdateSummaryAndResume(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	sessionID := uuid.NewString()
	if err := st.SyncMessages(sessionID, []llm.Message{{
		Role:    llm.RoleUser,
		Content: []llm.ContentBlock{{Type: "text", Text: "first question"}},
	}}); err != nil {
		t.Fatal(err)
	}
	wantSummary := "用户讨论了 session 持久化"
	if err := st.UpdateSummary(sessionID, wantSummary); err != nil {
		t.Fatal(err)
	}

	summary, msgs, err := st.LoadSession(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if summary != wantSummary {
		t.Errorf("summary = %q, want %q", summary, wantSummary)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgs))
	}
}

func TestListSessionsWithPreview(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	id1 := uuid.NewString()
	id2 := uuid.NewString()
	if err := st.SyncMessages(id1, []llm.Message{{
		Role:    llm.RoleUser,
		Content: []llm.ContentBlock{{Type: "text", Text: "第一条用户消息用于预览"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SyncMessages(id2, []llm.Message{{
		Role: llm.RoleUser,
		Content: []llm.ContentBlock{{Type: "text", Text: "第二条会话"}},
	}}); err != nil {
		t.Fatal(err)
	}

	entries, err := st.ListSessions(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	found := map[string]string{}
	for _, e := range entries {
		found[e.ID] = e.Preview
	}
	if got := found[id1]; got == "" || got != "第一条用户消息用于预览" {
		t.Errorf("id1 preview = %q", got)
	}
	if got := found[id2]; got != "第二条会话" {
		t.Errorf("id2 preview = %q", got)
	}
}

func TestLoadSessionNotFound(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	_, _, err = st.LoadSession("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestLoadSessionWrongProject(t *testing.T) {
	rootA := testutil.TempProjectRoot(t)
	rootB := testutil.TempProjectRoot(t)

	stA, err := Open(rootA)
	if err != nil {
		t.Fatal(err)
	}
	defer stA.Close()

	sessionID := uuid.NewString()
	if err := stA.SyncMessages(sessionID, []llm.Message{{
		Role:    llm.RoleUser,
		Content: []llm.ContentBlock{{Type: "text", Text: "secret"}},
	}}); err != nil {
		t.Fatal(err)
	}

	stB, err := Open(rootB)
	if err != nil {
		t.Fatal(err)
	}
	defer stB.Close()

	_, _, err = stB.LoadSession(sessionID)
	if err == nil {
		t.Fatal("expected error when loading session from different project")
	}
}

// lazyAgentSource 延迟绑定 Agent 指针，避免 New 时 Source 与 Agent 循环依赖。
type lazyAgentSource struct {
	ag *agent.Agent
}

func (l *lazyAgentSource) SessionID() string {
	if l.ag == nil {
		return ""
	}
	return l.ag.SessionID()
}

func (l *lazyAgentSource) Messages() []llm.Message {
	if l.ag == nil {
		return nil
	}
	return l.ag.Messages()
}

func TestAgentSessionPersistResume(t *testing.T) {
	root := testutil.TempProjectRoot(t)

	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		{Events: []llm.StreamEvent{
			{Type: llm.StreamEventTextDelta, Text: "已记录。"},
			{Type: llm.StreamEventMessageEnd, Usage: llm.Usage{InputTokens: 5, OutputTokens: 2}},
		}},
	}

	policy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	sessionID := uuid.NewString()
	var src lazyAgentSource
	ag, err := agent.New(root, mock, agent.Options{
		SessionID: sessionID,
		Policy:    policy,
		OnSession: PersistOnEnd{Store: st, Source: &src},
	})
	if err != nil {
		t.Fatal(err)
	}
	src.ag = ag

	if _, err := ag.HandleInput(context.Background(), "记住这个会话", nil); err != nil {
		t.Fatal(err)
	}
	ag.OnSessionEnd()

	summary, loaded, err := st.LoadSession(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if summary != "" {
		t.Errorf("unexpected summary %q", summary)
	}
	if len(loaded) < 2 {
		t.Fatalf("loaded messages = %d, want at least user+assistant", len(loaded))
	}
	if loaded[0].Role != llm.RoleUser || loaded[0].Content[0].Text != "记住这个会话" {
		t.Errorf("first message = %+v", loaded[0])
	}

	mock2 := testutil.NewMockLLM()
	mock2.StreamResponses = []testutil.MockResponse{
		{Events: []llm.StreamEvent{
			{Type: llm.StreamEventTextDelta, Text: "继续。"},
			{Type: llm.StreamEventMessageEnd, Usage: llm.Usage{InputTokens: 3, OutputTokens: 1}},
		}},
	}
	resumed, err := agent.New(root, mock2, agent.Options{
		SessionID: sessionID,
		Policy:    policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	resumed.RestoreState(loaded, false, summary)
	if resumed.MemoryInjected() {
		t.Error("memoryInjected should be false after resume")
	}
	if len(resumed.Messages()) != len(loaded) {
		t.Fatalf("resumed messages = %d, want %d", len(resumed.Messages()), len(loaded))
	}

	if _, err := resumed.HandleInput(context.Background(), "继续对话", nil); err != nil {
		t.Fatal(err)
	}
	if len(mock2.StreamCalls) != 1 {
		t.Fatalf("StreamChat calls = %d, want 1", len(mock2.StreamCalls))
	}
	if len(mock2.StreamCalls[0].Messages) <= len(loaded) {
		t.Errorf("resume should send prior messages + new user msg, got %d msgs", len(mock2.StreamCalls[0].Messages))
	}
}

func TestPersistOnEndSkipsEmptySession(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	mock := testutil.NewMockLLM()
	var src lazyAgentSource
	ag, err := agent.New(root, mock, agent.Options{
		OnSession: PersistOnEnd{Store: st, Source: &src},
	})
	if err != nil {
		t.Fatal(err)
	}
	src.ag = ag

	ag.OnSessionEnd()
	entries, err := st.ListSessions(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no sessions without user messages, got %d", len(entries))
	}
}

func TestTruncatePreview(t *testing.T) {
	long := strings.Repeat("a", 100)
	got := truncatePreview(long)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
	if len([]rune(got)) > 80 {
		t.Errorf("preview rune length = %d", len([]rune(got)))
	}
}

func TestInsertAndListMemoryFacts(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	sessionID := uuid.NewString()
	facts := []memory.MemoryFact{
		{Content: "用户偏好 tabs", Category: "preference"},
		{Content: "项目用 Go", Category: "project_fact"},
	}
	if err := st.InsertMemoryFacts(sessionID, facts); err != nil {
		t.Fatal(err)
	}

	loaded, err := st.ListMemoryFacts()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("facts = %d, want 2", len(loaded))
	}
	if loaded[0].ProjectID != st.ProjectIDValue() {
		t.Errorf("project_id = %q", loaded[0].ProjectID)
	}
	if loaded[0].SessionID != sessionID {
		t.Errorf("session_id = %q", loaded[0].SessionID)
	}
}

func TestIncrementSessionCount(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	c1, err := st.IncrementSessionCount()
	if err != nil {
		t.Fatal(err)
	}
	c2, err := st.IncrementSessionCount()
	if err != nil {
		t.Fatal(err)
	}
	if c1 != 1 || c2 != 2 {
		t.Fatalf("counts = %d, %d, want 1, 2", c1, c2)
	}
}

func TestDeleteAllFactsAndResetSessionCount(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	sessionID := uuid.NewString()
	if err := st.InsertMemoryFacts(sessionID, []memory.MemoryFact{
		{Content: "fact", Category: "preference"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.IncrementSessionCount(); err != nil {
		t.Fatal(err)
	}

	if err := st.DeleteAllFacts(); err != nil {
		t.Fatal(err)
	}
	facts, err := st.ListMemoryFacts()
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 0 {
		t.Fatalf("facts after delete = %d", len(facts))
	}

	if err := st.ResetSessionCount(); err != nil {
		t.Fatal(err)
	}
	count, err := st.IncrementSessionCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count after reset = %d, want 1", count)
	}
}

func TestRunLayer2WithStore(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	sessionID := uuid.NewString()
	if err := st.InsertMemoryFacts(sessionID, []memory.MemoryFact{
		{Content: "用户偏好 tabs 缩进", Category: "preference"},
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := st.IncrementSessionCount(); err != nil {
			t.Fatal(err)
		}
	}

	mock := testutil.NewMockLLM()
	mock.CompleteText = "# 用户画像（2026-07-04 更新，基于 3 次会话）\n\n## 技术偏好\n- tabs"

	if err := memory.RunLayer2(context.Background(), st.ProjectIDValue(), root, st, mock); err != nil {
		t.Fatal(err)
	}

	facts, err := st.ListMemoryFacts()
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 0 {
		t.Fatalf("facts = %d, want 0", len(facts))
	}
	count, err := st.SessionCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("session count = %d, want 0", count)
	}
}
