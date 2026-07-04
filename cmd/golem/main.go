package main

import (
	"flag"
	"fmt"
)

// version 在构建时可通过 -ldflags 注入，如：
//   go build -ldflags "-X main.version=v0.1.0" ./cmd/golem
// 默认值用于开发构建。
var version = "v0.1.0-dev"

func main() {
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	// Step 7 起在此启动 TUI；当前仅打印版本，保持与 plan 验收一致。
	fmt.Println("golem", version)
}
