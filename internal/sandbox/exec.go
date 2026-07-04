package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// FallbackWarning 在 user namespace 不可用时 prepend 到 bash 输出，提示已降级为直接 exec。
const FallbackWarning = "[golem: sandbox 不可用，bash 未在 namespace 中执行]"

// execBash 在 projectRoot 下直接执行 bash -c cmd，合并 stdout 与 stderr。
func execBash(ctx context.Context, cmd, projectRoot string) (string, error) {
	c := exec.CommandContext(ctx, "bash", "-c", cmd)
	c.Dir = projectRoot
	out, err := c.CombinedOutput()
	result := strings.TrimRight(string(out), "\n")
	if err != nil {
		if result == "" {
			return "", fmt.Errorf("bash: %w", err)
		}
		return result, fmt.Errorf("bash: %w", err)
	}
	return result, nil
}

// prependWarning 在输出前追加降级警告；空输出时仅返回警告文本。
func prependWarning(out string) string {
	if out == "" {
		return FallbackWarning
	}
	return FallbackWarning + "\n" + out
}
