package sandbox

// SandboxMode 表示 bash 执行的沙箱策略；文件工具不走 namespace，仅做 project_root 路径校验。
type SandboxMode string

const (
	// ModeWorkspaceWrite 在支持时于新 user+mount namespace 中执行 bash；无 bind mount。
	ModeWorkspaceWrite SandboxMode = "workspace-write"
	// ModeDangerFullAccess bash 直接 exec，不 fork namespace。
	ModeDangerFullAccess SandboxMode = "danger-full-access"
)

// Modes 列出全部合法 sandbox 模式，供 TUI / CLI 校验与循环切换。
var Modes = []SandboxMode{
	ModeWorkspaceWrite,
	ModeDangerFullAccess,
}

// ParseMode 将配置或 CLI 字符串解析为 SandboxMode；空或未知值回退 workspace-write。
func ParseMode(raw string) SandboxMode {
	switch SandboxMode(raw) {
	case ModeDangerFullAccess:
		return ModeDangerFullAccess
	default:
		return ModeWorkspaceWrite
	}
}
