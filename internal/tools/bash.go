package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func bashTool(projectRoot string) Tool {
	return Tool{
		Name:        "bash",
		Description: "Execute a shell command in bash. Working directory is project_root.",
		InputSchema: objectSchema(map[string]any{
			"command": stringProperty("Shell command to execute"),
		}, "command"),
		Execute: func(ctx context.Context, input map[string]any) (string, error) {
			command, err := requiredString(input, "command")
			if err != nil {
				return "", err
			}
			return runBash(ctx, projectRoot, command)
		},
	}
}

// runBash 在 projectRoot 下执行 bash -c command，合并 stdout 与 stderr 返回。
// P1 将接入 sandbox；P0 直接 exec。
func runBash(ctx context.Context, projectRoot, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	result := strings.TrimRight(string(out), "\n")
	if err != nil {
		if result == "" {
			return "", fmt.Errorf("bash: %w", err)
		}
		return result, fmt.Errorf("bash: %w", err)
	}
	return result, nil
}
