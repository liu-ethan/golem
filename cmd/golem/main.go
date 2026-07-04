package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/tui"
)

// version 在构建时可通过 -ldflags 注入，如：
//   go build -ldflags "-X main.version=v0.1.0" ./cmd/golem
// 默认值用于开发构建。
var version = "v0.1.0-dev"

// agentRef 供 PersistOnEnd 在 Agent 创建后读取消息快照。
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

func main() {
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	approvalFlag := flag.String("approval", "", "审批模式：plan | ask-before-edit | ask | edit-automatically")
	sandboxFlag := flag.String("sandbox", "", "沙箱模式：workspace-write | danger-full-access")
	resumeFlag := flag.String("resume", "", "恢复指定 session id 的历史会话")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "golem: resolve working directory: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(projectRoot, config.Overrides{
		Approval: *approvalFlag,
		Sandbox:  *sandboxFlag,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "golem: load config: %v\n", err)
		os.Exit(1)
	}

	policy, err := approval.New(cfg.Defaults.Approval)
	if err != nil {
		fmt.Fprintf(os.Stderr, "golem: approval policy: %v\n", err)
		os.Exit(1)
	}

	store, err := session.Open(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "golem: open session store: %v\n", err)
		os.Exit(1)
	}

	sessionID := uuid.NewString()
	var resumeSummary string
	var resumeMsgs []llm.Message
	if *resumeFlag != "" {
		sessionID = *resumeFlag
		resumeSummary, resumeMsgs, err = store.LoadSession(*resumeFlag)
		if err != nil {
			_ = store.Close()
			fmt.Fprintf(os.Stderr, "golem: resume session: %v\n", err)
			os.Exit(1)
		}
	}

	var agPtr *agent.Agent
	ref := agentRef{ag: &agPtr}

	llmClient := llm.NewAnthropicClient(
		cfg.Provider.BaseURL,
		cfg.Provider.APIKey,
		cfg.Provider.Model,
		llm.WithSessionID(sessionID),
	)

	ag, err := agent.New(projectRoot, llmClient, agent.Options{
		SessionID:    sessionID,
		Policy:       policy,
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
	})
	if err != nil {
		_ = store.Close()
		fmt.Fprintf(os.Stderr, "golem: create agent: %v\n", err)
		os.Exit(1)
	}
	agPtr = ag

	if *resumeFlag != "" {
		ag.RestoreState(resumeMsgs, false, resumeSummary)
	}

	rulesLines, err := tui.LoadRulesDisplay(projectRoot)
	if err != nil {
		_ = store.Close()
		fmt.Fprintf(os.Stderr, "golem: load rules: %v\n", err)
		os.Exit(1)
	}

	tui.MustRun(tui.Config{
		ProjectRoot:  projectRoot,
		Agent:        ag,
		Store:        store,
		Policy:       policy,
		Sandbox:      cfg.Defaults.Sandbox,
		ModelName:    cfg.Provider.Model,
		ContextLimit: cfg.Provider.ContextLimit,
		RulesLines:   rulesLines,
	})
}
