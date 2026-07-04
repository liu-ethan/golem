package agent

// ToolGate 决定工具调用是否被拒绝或需要用户确认。
// Step 5 的 ApprovalPolicy 将实现此接口；P0 默认使用 AllowAllGate。
type ToolGate interface {
	IsDenied(toolName string, input map[string]any) bool
	ShouldConfirm(toolName string, input map[string]any) bool
}

// AllowAllGate 等同 edit-automatically：不拒绝、不弹确认框（P0 rules/sandbox 占位期使用）。
type AllowAllGate struct{}

// IsDenied 恒为 false。
func (AllowAllGate) IsDenied(_ string, _ map[string]any) bool {
	return false
}

// ShouldConfirm 恒为 false。
func (AllowAllGate) ShouldConfirm(_ string, _ map[string]any) bool {
	return false
}

// DenyWriteGate 模拟 plan 模式：拒绝写操作与 bash，读操作自动通过。
type DenyWriteGate struct{}

// IsDenied 对 write_file、edit_file、bash 返回 true。
func (DenyWriteGate) IsDenied(toolName string, _ map[string]any) bool {
	switch toolName {
	case "write_file", "edit_file", "bash":
		return true
	default:
		return false
	}
}

// ShouldConfirm 恒为 false；plan 模式直接拒绝，不弹框。
func (DenyWriteGate) ShouldConfirm(_ string, _ map[string]any) bool {
	return false
}

// ConfirmAllGate 对任意工具均要求确认，用于测试确认拒绝路径。
type ConfirmAllGate struct{}

// IsDenied 恒为 false。
func (ConfirmAllGate) IsDenied(_ string, _ map[string]any) bool {
	return false
}

// ShouldConfirm 恒为 true。
func (ConfirmAllGate) ShouldConfirm(_ string, _ map[string]any) bool {
	return true
}
