package main

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/memory"
	"github.com/tencent-docs/golem/internal/rules"
	"github.com/tencent-docs/golem/internal/sandbox"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/skills"
)

type agentRef struct {
	ag **agent.Agent
}

func (r agentRef) SessionID() string {
	if r.ag == nil || *r.ag == nil {
		return ""
	}
	return (*r.ag).SessionID()
}

func (r agentRef) Messages() []llm.Message {
	if r.ag == nil || *r.ag == nil {
		return nil
	}
	return (*r.ag).Messages()
}

type bootstrapResult struct {
	projectRoot string
	cfg         config.Config
	policy      *approval.Policy
	store       *session.Store
	ag          *agent.Agent
	llmClient   llm.LLMClient
	rulesLines  []string
	skillName   string
}

func (b *bootstrapResult) agLLM() llm.LLMClient {
	return b.llmClient
}

// bootstrapAgent 加载配置并创建 Agent，供 TUI 与 headless 共用。
func bootstrapAgent(projectRoot string, overrides config.Overrides, resumeID, skillName string) (*bootstrapResult, error) {
	cfg, err := config.LoadConfig(projectRoot, overrides)
	if err != nil {
		return nil, err
	}

	policy, err := approval.New(cfg.Defaults.Approval)
	if err != nil {
		return nil, fmt.Errorf("approval policy: %w", err)
	}

	store, err := session.Open(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("open session store: %w", err)
	}

	sessionID := uuid.NewString()
	var resumeSummary string
	var resumeMsgs []llm.Message
	if resumeID != "" {
		sessionID = resumeID
		resumeSummary, resumeMsgs, err = store.LoadSession(resumeID)
		if err != nil {
			store.Close()
			return nil, fmt.Errorf("resume session: %w", err)
		}
	}

	var agPtr *agent.Agent
	ref := agentRef{ag: &agPtr}

	llmClient := llm.NewAnthropicClient(
		cfg.Provider.BaseURL,
		cfg.Provider.APIKey,
		cfg.Provider.Model,
		llm.WithSessionID(sessionID),
		llm.WithTokenUsageHook(func(_ string, u llm.Usage, _ string) {
			if agPtr != nil {
				(*agPtr).AddTokenUsage(u)
			}
		}),
	)

	loadedRules, err := rules.Load(projectRoot)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("load rules: %w", err)
	}

	ag, err := agent.New(projectRoot, llmClient, agent.Options{
		SessionID: sessionID,
		Policy:    policy,
		Rules:     loadedRules,
		Memory: agent.BM25MemoryProvider{
			Store:     store,
			Retriever: memory.NewBM25Retriever(),
			TopK:      cfg.Memory.BM25TopK,
		},
		OnSession: agent.ChainEndHandler{
			session.PersistOnEnd{Store: store, Source: ref},
			agent.MemoryOnEnd{
				Store:       store,
				Source:      ref,
				ProjectRoot: projectRoot,
				MemoryCfg:   cfg.Memory,
				LLM:         llmClient,
			},
		},
		InitialMsgs:  resumeMsgs,
		MemoryCfg:    cfg.Memory,
		ContextLimit: cfg.Provider.ContextLimit,
		SummaryStore: store,
		SandboxMode:  sandbox.ParseMode(cfg.Defaults.Sandbox),
	})
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("create agent: %w", err)
	}
	agPtr = ag

	if resumeID != "" {
		ag.RestoreState(resumeMsgs, false, resumeSummary)
	}

	ag.SetDenialRecorder(func(tool string, input map[string]any, reason string) {
		raw, _ := json.Marshal(input)
		_ = store.InsertDenial(tool, string(raw), reason)
	})

	if skillName != "" {
		loader := skills.NewLoader(projectRoot)
		skill, err := loader.Load(skillName)
		if err != nil {
			store.Close()
			return nil, err
		}
		ag.SetSkill(skill)
	}

	return &bootstrapResult{
		projectRoot: projectRoot,
		cfg:         cfg,
		policy:      policy,
		store:       store,
		ag:          ag,
		llmClient:   llmClient,
		skillName:   skillName,
	}, nil
}
