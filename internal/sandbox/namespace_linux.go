//go:build linux

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// UserNamespaceSupported 检测当前 Linux 环境是否允许创建 unprivileged user namespace。
func UserNamespaceSupported() bool {
	data, err := os.ReadFile("/proc/sys/user/max_user_namespaces")
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return err == nil && n > 0
}

// runSandboxed 在 workspace-write 模式下于新 namespace 中执行 bash；不支持时降级并警告。
func runSandboxed(ctx context.Context, cmd, projectRoot string) (string, error) {
	if !UserNamespaceSupported() {
		out, err := execBash(ctx, cmd, projectRoot)
		return prependWarning(out), err
	}
	out, err := runInNamespace(ctx, cmd, projectRoot)
	if err != nil && isUserNamespaceSetupError(err) {
		fallback, fbErr := execBash(ctx, cmd, projectRoot)
		return prependWarning(fallback), fbErr
	}
	return out, err
}

// runInNamespace 使用 CLONE_NEWUSER|CLONE_NEWNS fork bash；P1 无 bind mount。
func runInNamespace(ctx context.Context, cmd, projectRoot string) (string, error) {
	c := exec.CommandContext(ctx, "bash", "-c", cmd)
	c.Dir = projectRoot
	c.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{{
			ContainerID: 0,
			HostID:      os.Getuid(),
			Size:        1,
		}},
		GidMappings: []syscall.SysProcIDMap{{
			ContainerID: 0,
			HostID:      os.Getgid(),
			Size:        1,
		}},
	}
	out, err := c.CombinedOutput()
	result := strings.TrimRight(string(out), "\n")
	if err != nil {
		if result == "" {
			return "", err
		}
		return result, err
	}
	return result, nil
}

// isUserNamespaceSetupError 判断 exec 失败是否由 user namespace 配置导致。
func isUserNamespaceSetupError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "invalid argument")
}
