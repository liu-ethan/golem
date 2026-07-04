package integration

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/memory"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/testutil"
)

const testdataDir = "testdata"

// agentSource 供 PersistOnEnd / MemoryOnEnd 延迟读取 Agent 快照。
type agentSource struct {
	ag *agent.Agent
}

func (s *agentSource) SessionID() string {
	if s.ag == nil {
		return ""
	}
	return s.ag.SessionID()
}

func (s *agentSource) Messages() []llm.Message {
	if s.ag == nil {
		return nil
	}
	return s.ag.Messages()
}

// newProductionAgent 按 main.go 方式装配 Agent：真实 SQLite、BM25 注入、会话持久化与记忆提取。
func newProductionAgent(t *testing.T, root string, store *session.Store, mock *testutil.MockLLM, sessionID string) (*agent.Agent, *agentSource) {
	t.Helper()

	policy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}

	var src agentSource
	ag, err := agent.New(root, mock, agent.Options{
		SessionID: sessionID,
		Policy:    policy,
		Memory: agent.BM25MemoryProvider{
			Store:     store,
			Retriever: memory.NewBM25Retriever(),
			TopK:      5,
		},
		OnSession: agent.ChainEndHandler{
			session.PersistOnEnd{Store: store, Source: &src},
			agent.MemoryOnEnd{
				Store:       store,
				Source:      &src,
				ProjectRoot: root,
				MemoryCfg:   config.MemoryConfig{Layer2SessionThreshold: 3, BM25TopK: 5},
				LLM:         mock,
			},
		},
		MemoryCfg:    config.MemoryConfig{Layer2SessionThreshold: 3, BM25TopK: 5},
		ContextLimit: 200000,
		SummaryStore: store,
	})
	if err != nil {
		t.Fatal(err)
	}
	src.ag = ag
	return ag, &src
}

func textStreamResponse(text string) testutil.MockResponse {
	return testutil.MockResponse{Events: []llm.StreamEvent{
		{Type: llm.StreamEventTextDelta, Text: text},
		{Type: llm.StreamEventMessageEnd, Usage: llm.Usage{InputTokens: 5, OutputTokens: 2}},
	}}
}

func layer1FactsJSON(facts ...map[string]string) string {
	payload, err := json.Marshal(facts)
	if err != nil {
		panic(err)
	}
	return string(payload)
}

func newSessionID() string {
	return uuid.NewString()
}
