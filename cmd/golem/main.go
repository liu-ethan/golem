package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/config"
)

// version 在构建时可通过 -ldflags 注入，如：
//   go build -ldflags "-X main.version=v0.1.0" ./cmd/golem
// 默认值用于开发构建。
var version = "v0.1.0-dev"

func main() {
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	approvalFlag := flag.String("approval", "", "审批模式：plan | ask-before-edit | ask | edit-automatically")
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

	// Step 7 起在此启动 TUI 并将 policy 注入 Agent；当前仅打印版本与审批模式。
	fmt.Printf("golem %s (approval: %s)\n", version, policy.Mode())
}
