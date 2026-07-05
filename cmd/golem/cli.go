package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/headless"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/skills"
	"github.com/tencent-docs/golem/internal/tui"
)

func runCLI(args []string) int {
	if len(args) == 0 {
		return -1
	}
	switch args[0] {
	case "sessions":
		return runSessionsSubcommand(args[1:])
	case "skill":
		return runSkillSubcommand(args[1:])
	default:
		return runHeadless(strings.Join(args, " "))
	}
}

func runSessionsSubcommand(args []string) int {
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "golem: resolve working directory: %v\n", err)
		return 1
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: golem sessions list|delete <id>")
		return 1
	}
	switch args[0] {
	case "list":
		if err := session.RunListCLI(projectRoot); err != nil {
			fmt.Fprintf(os.Stderr, "golem: %v\n", err)
			return 1
		}
		return 0
	case "delete":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: golem sessions delete <session-id>")
			return 1
		}
		if err := session.RunDeleteCLI(projectRoot, args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "golem: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintln(os.Stderr, "usage: golem sessions list|delete <id>")
		return 1
	}
}

func runSkillSubcommand(args []string) int {
	if len(args) < 2 || args[0] != "install" {
		fmt.Fprintln(os.Stderr, "usage: golem skill install github:user/repo/skill-name")
		return 1
	}
	dest, err := skills.InstallGitHub(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "golem: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "installed skill to %s\n", dest)
	return 0
}

func runHeadless(query string) int {
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "golem: resolve working directory: %v\n", err)
		return 1
	}
	boot, err := bootstrapAgent(projectRoot, config.Overrides{}, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "golem: %v\n", err)
		return 1
	}
	defer boot.store.Close()
	if err := headless.Run(context.Background(), boot.ag, query); err != nil {
		fmt.Fprintf(os.Stderr, "golem: %v\n", err)
		return 1
	}
	return 0
}

func runTUI(showVersion bool, approvalFlag, sandboxFlag, resumeFlag string) int {
	if showVersion {
		fmt.Println(version)
		return 0
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "golem: resolve working directory: %v\n", err)
		return 1
	}

	if _, err := config.EnsureProjectConfig(projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "golem: ensure config: %v\n", err)
		return 1
	}

	cfgPreview, err := config.LoadConfig(projectRoot, config.Overrides{
		Approval: approvalFlag,
		Sandbox:  sandboxFlag,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "golem: load config: %v\n", err)
		return 1
	}
	needsSetup := config.NeedsProviderSetup(cfgPreview)

	boot, err := bootstrapAgent(projectRoot, config.Overrides{
		Approval: approvalFlag,
		Sandbox:  sandboxFlag,
	}, resumeFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "golem: %v\n", err)
		return 1
	}

	rulesLines, err := tui.LoadRulesDisplay(projectRoot)
	if err != nil {
		boot.store.Close()
		fmt.Fprintf(os.Stderr, "golem: format rules: %v\n", err)
		return 1
	}

	if err := tui.Run(tui.Config{
		ProjectRoot:      projectRoot,
		Version:          version,
		Agent:            boot.ag,
		Store:            boot.store,
		Policy:           boot.policy,
		Sandbox:          boot.cfg.Defaults.Sandbox,
		ModelName:        boot.cfg.Provider.Model,
		ContextLimit:     boot.cfg.Provider.ContextLimit,
		RulesLines:       rulesLines,
		SkillLoader:      skills.NewLoader(projectRoot),
		LLMClient:        boot.agLLM(),
		NeedsSetup:       needsSetup,
		DefaultBaseURL:   cfgPreview.Provider.BaseURL,
		DefaultModel:     cfgPreview.Provider.Model,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "golem: %v\n", err)
		return 1
	}
	return 0
}

func mainEntry() int {
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	approvalFlag := flag.String("approval", "", "审批模式：plan | ask-before-edit | ask | edit-automatically")
	sandboxFlag := flag.String("sandbox", "", "沙箱模式：workspace-write | danger-full-access")
	resumeFlag := flag.String("resume", "", "恢复指定 session id 的历史会话")
	flag.Parse()

	args := flag.Args()
	if len(args) > 0 && !*showVersion {
		return runCLI(args)
	}
	return runTUI(*showVersion, *approvalFlag, *sandboxFlag, *resumeFlag)
}
