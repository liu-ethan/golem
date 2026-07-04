package sandbox

import "context"

// RunBash 按 sandbox 模式在 projectRoot 下执行 bash -c cmd。
// workspace-write 在支持时 fork CLONE_NEWUSER|CLONE_NEWNS；danger-full-access 直接 exec。
func RunBash(ctx context.Context, cmd, projectRoot string, mode SandboxMode) (string, error) {
	if mode == ModeDangerFullAccess {
		return execBash(ctx, cmd, projectRoot)
	}
	return runSandboxed(ctx, cmd, projectRoot)
}
