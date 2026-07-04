package integration

import (
	"context"
	"testing"

	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/testutil"
)

// TestSessionResumeRoundTrip 模拟 --resume：首轮对话持久化后，新 Agent 还原历史并继续对话。
func TestSessionResumeRoundTrip(t *testing.T) {
	root := testutil.TempProjectRoot(t)

	st, err := session.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	mock := testutil.NewMockLLM()
	mock.StreamResponses = []testutil.MockResponse{
		textStreamResponse("已记录偏好。"),
	}

	sessionID := newSessionID()
	ag, _ := newProductionAgent(t, root, st, mock, sessionID)

	if _, err := ag.HandleInput(context.Background(), "我用 tabs 缩进", nil); err != nil {
		t.Fatal(err)
	}
	ag.OnSessionEnd()

	summary, loaded, err := st.LoadSession(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) < 2 {
		t.Fatalf("loaded messages = %d", len(loaded))
	}

	mock2 := testutil.NewMockLLM()
	mock2.StreamResponses = []testutil.MockResponse{
		textStreamResponse("继续。"),
	}

	policy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := agent.New(root, mock2, agent.Options{
		SessionID:   sessionID,
		Policy:      policy,
		InitialMsgs: loaded,
	})
	if err != nil {
		t.Fatal(err)
	}
	resumed.RestoreState(loaded, false, summary)
	if resumed.MemoryInjected() {
		t.Error("memoryInjected should be false after resume")
	}

	if _, err := resumed.HandleInput(context.Background(), "继续对话", nil); err != nil {
		t.Fatal(err)
	}
	if len(mock2.StreamCalls) != 1 {
		t.Fatalf("StreamChat calls = %d, want 1", len(mock2.StreamCalls))
	}
	if len(mock2.StreamCalls[0].Messages) <= len(loaded) {
		t.Errorf("resume should include prior messages, got %d msgs", len(mock2.StreamCalls[0].Messages))
	}
}
