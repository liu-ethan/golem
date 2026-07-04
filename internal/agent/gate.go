package agent

import "github.com/tencent-docs/golem/internal/approval"

// ToolGate 决定工具调用是否被拒绝或需要用户确认。
type ToolGate interface {
	IsDenied(toolName string, input map[string]any) bool
	ShouldConfirm(toolName string, input map[string]any) bool
}

// policyGate 将 approval.ApprovalPolicy 适配为 ToolGate。
type policyGate struct {
	policy approval.ApprovalPolicy
}

// GateFromPolicy 将审批策略包装为 ToolGate；policy 为 nil 时等同 edit-automatically。
func GateFromPolicy(policy approval.ApprovalPolicy) ToolGate {
	if policy == nil {
		return GateFromPolicy(mustPolicy(approval.ModeEditAutomatically))
	}
	return policyGate{policy: policy}
}

// mustPolicy 创建策略，失败时 panic（仅用于已知合法模式的内部默认）。
func mustPolicy(mode string) approval.ApprovalPolicy {
	p, err := approval.New(mode)
	if err != nil {
		panic(err)
	}
	return p
}

// IsDenied 委托 ApprovalPolicy.IsDenied。
func (g policyGate) IsDenied(toolName string, input map[string]any) bool {
	return g.policy.IsDenied(toolName, input)
}

// ShouldConfirm 委托 ApprovalPolicy.ShouldConfirm。
func (g policyGate) ShouldConfirm(toolName string, input map[string]any) bool {
	return g.policy.ShouldConfirm(toolName, input)
}
