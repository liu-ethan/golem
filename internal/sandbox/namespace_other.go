//go:build !linux

package sandbox

import "context"

// UserNamespaceSupported 在非 Linux 平台恒为 false。
func UserNamespaceSupported() bool {
	return false
}

// runSandboxed 在非 Linux 平台降级为直接 exec 并附带警告。
func runSandboxed(ctx context.Context, cmd, projectRoot string) (string, error) {
	out, err := execBash(ctx, cmd, projectRoot)
	return prependWarning(out), err
}
