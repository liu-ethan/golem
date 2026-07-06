package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/testutil"
)

func testModel(t *testing.T) Model {
	t.Helper()
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	policy, err := approval.New(approval.ModePlan)
	if err != nil {
		t.Fatal(err)
	}
	ag, err := agent.New(root, mock, agent.Options{Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	return NewModel(Config{
		ProjectRoot:  root,
		Agent:        ag,
		Policy:       policy,
		Sandbox:      "workspace-write",
		ContextLimit: 200000,
	})
}

func TestHandleAgentEventThinkingThenAnswer(t *testing.T) {
	m := Model{width: 80, height: 24}
	m.handleAgentEvent(agent.Event{Type: agent.EventThinkingDelta, Text: "analyze"})
	m.handleAgentEvent(agent.Event{Type: agent.EventThinkingDelta, Text: " code"})
	if m.thinkingStreaming != "analyze code" {
		t.Fatalf("thinking = %q", m.thinkingStreaming)
	}

	m.handleAgentEvent(agent.Event{Type: agent.EventTextDelta, Text: "Done."})
	if m.thinkingStreaming != "" {
		t.Fatal("thinking should be flushed")
	}
	if len(m.lines) != 1 || m.lines[0].Kind != LineThinking {
		t.Fatalf("lines = %+v", m.lines)
	}
	if m.streaming != "Done." {
		t.Fatalf("streaming = %q", m.streaming)
	}

	m.handleAgentEvent(agent.Event{Type: agent.EventTurnComplete})
	if len(m.lines) != 2 {
		t.Fatalf("lines = %+v", m.lines)
	}
	if m.lines[1].Kind != LineAssistant || m.lines[1].Text != "Done." {
		t.Fatalf("assistant line = %+v", m.lines[1])
	}
}

func TestRenderThinkingBlockSeparateFromAnswer(t *testing.T) {
	m := Model{
		width:  80,
		height: 24,
		lines: []ChatLine{
			{Kind: LineThinking, Text: "planning"},
			{Kind: LineAssistant, Text: "final answer"},
		},
	}
	out := renderChatArea(m, 80, 20)
	if !strings.Contains(out, "Thinking") {
		t.Fatal("expected thinking block")
	}
	if !strings.Contains(out, "Golem") {
		t.Fatal("expected assistant label")
	}
	if !strings.Contains(out, "final answer") {
		t.Fatal("expected final answer")
	}
}

func TestWelcomePanelShowsVersion(t *testing.T) {
	m := testModel(t)
	m.version = "v0.2.0"
	out := renderWelcomePanel(m, 80, 24)
	if !strings.Contains(out, "v0.2.0") {
		t.Fatalf("welcome missing version: %s", out)
	}
	if !strings.Contains(out, "Welcome back") {
		t.Fatal("welcome missing greeting")
	}
	if !strings.Contains(out, "Tips for getting started") {
		t.Fatal("welcome missing tips column")
	}
}

func TestChatEmptyStateShowsHomeDashboard(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.width = 80
	m.height = 24
	out := renderChatArea(m, 80, 20)
	if !strings.Contains(out, "Welcome back") {
		t.Fatalf("empty chat missing dashboard: %s", out)
	}
	if !strings.Contains(out, "Tips for getting started") {
		t.Fatal("empty chat missing tips")
	}
	if !strings.Contains(out, "What's new") {
		t.Fatal("empty chat missing news")
	}
}

func TestChatWithMessagesHidesHomeDashboard(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.lines = []ChatLine{{Kind: LineUser, Text: "hello"}}
	out := renderChatArea(m, 80, 20)
	if strings.Contains(out, "Tips for getting started") {
		t.Fatal("dashboard should not show when messages exist")
	}
	if !strings.Contains(out, "hello") {
		t.Fatal("expected user message")
	}
}

func TestConfirmFlowApprove(t *testing.T) {
	resp := make(chan bool, 1)
	m := testModel(t)
	m.activePage = PageChat
	m.width = 80

	next, _ := m.Update(confirmRequestMsg{
		toolName: "bash",
		input:    map[string]any{"command": "echo ok"},
		resp:     resp,
	})
	m2 := next.(Model)
	if m2.confirm == nil {
		t.Fatal("expected confirm state")
	}
	out := renderView(m2)
	if !strings.Contains(out, "是否允许") {
		t.Fatalf("expected confirm prompt: %s", out)
	}
	if !strings.Contains(out, "[Y/Enter] 允许") {
		t.Fatal("expected confirm footer hints")
	}

	next, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m3 := next.(Model)
	if m3.confirm != nil {
		t.Fatal("confirm should be cleared after approve")
	}
	select {
	case ok := <-resp:
		if !ok {
			t.Fatal("expected confirm approved")
		}
	default:
		t.Fatal("expected confirm response")
	}
}

func TestNewModelStartsOnWelcome(t *testing.T) {
	m := testModel(t)
	if m.activePage != PageWelcome {
		t.Fatalf("page = %v", m.activePage)
	}
}

func TestSlashTabCompletes(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.input = "/mod"

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := next.(Model)
	if m2.input != "/model " {
		t.Fatalf("input = %q", m2.input)
	}
}

func TestSubmitInputResolvesPartialSlash(t *testing.T) {
	m := testModel(t)
	m.activePage = PageChat
	m.input = "/mod"

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := next.(Model)
	if len(m2.lines) == 0 {
		t.Fatal("expected system line for /model")
	}
	last := m2.lines[len(m2.lines)-1]
	if last.Kind != LineSystem || !strings.Contains(last.Text, "model") {
		t.Fatalf("line = %+v", last)
	}
}

func TestHandleAgentDoneClearsErrMsgOnSuccess(t *testing.T) {
	m := testModel(t)
	m.errMsg = "llm api 400: bad model"
	m.handleAgentDone(agentDoneMsg{})
	if m.errMsg != "" {
		t.Fatalf("errMsg = %q, want cleared after success", m.errMsg)
	}
}

func TestHandleAgentDoneSetsErrMsgOnFailure(t *testing.T) {
	m := testModel(t)
	m.handleAgentDone(agentDoneMsg{err: errTestAPI})
	if m.errMsg == "" {
		t.Fatal("expected errMsg on failure")
	}
}

func TestApplySlashSetModelClearsErrMsg(t *testing.T) {
	m := testModel(t)
	m.errMsg = "llm api 400: bad model"
	m2, _ := m.applySlash("/model deepseek-v4-pro", dispatchSlash("/model deepseek-v4-pro", nil))
	if m2.errMsg != "" {
		t.Fatalf("errMsg = %q, want cleared after model change", m2.errMsg)
	}
}

var errTestAPI = &testAPIError{msg: "llm api 400: bad model"}

type testAPIError struct{ msg string }

func (e *testAPIError) Error() string { return e.msg }
