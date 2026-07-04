package tools

import (
	"context"

	"github.com/tencent-docs/golem/internal/sandbox"
)

func bashTool(projectRoot string, mode sandbox.SandboxMode) Tool {
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
			return sandbox.RunBash(ctx, command, projectRoot, mode)
		},
	}
}
