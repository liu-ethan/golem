package agent

import (
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/rules"
)

// ToolGate 决定工具调用是否被拒绝或需要用户确认。
type ToolGate interface {
	IsDenied(toolName string, input map[string]any) bool
	ShouldConfirm(toolName string, input map[string]any) bool
}

// RuleDenier 可选接口，报告拒绝是否来自权限规则层（bash pattern deny）。
type RuleDenier interface {
	DeniedByRule(toolName string, input map[string]any) bool
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

// GateWithRules 在 inner 门控外包裹 bash 权限规则层；deny 先于审批层，ask 强制确认。
func GateWithRules(ruleList []rules.Rule, inner ToolGate) ToolGate {
	if inner == nil {
		inner = GateFromPolicy(mustPolicy(approval.ModeEditAutomatically))
	}
	return rulesGate{rules: ruleList, inner: inner}
}

// mustPolicy 创建策略，失败时 panic（仅用于已知合法模式的内部默认）。
func mustPolicy(mode string) approval.ApprovalPolicy {
	p, err := approval.New(mode)
	if err != nil {
		panic(err)
	}
	return p
}

type rulesGate struct {
	rules []rules.Rule
	inner ToolGate
}

// IsDenied 先检查 bash 规则 deny，再委托 inner 审批层。
func (g rulesGate) IsDenied(toolName string, input map[string]any) bool {
	if g.DeniedByRule(toolName, input) {
		return true
	}
	return g.inner.IsDenied(toolName, input)
}

// DeniedByRule 报告 bash 命令是否被 rules.deny 命中。
func (g rulesGate) DeniedByRule(toolName string, input map[string]any) bool {
	if toolName != "bash" || len(g.rules) == 0 {
		return false
	}
	return rules.MatchBash(bashCommandFromInput(input), g.rules) == rules.ActionDeny
}

// ShouldConfirm bash 规则 ask 强制确认；否则委托 inner 审批层。
func (g rulesGate) ShouldConfirm(toolName string, input map[string]any) bool {
	if toolName == "bash" && len(g.rules) > 0 {
		switch rules.MatchBash(bashCommandFromInput(input), g.rules) {
		case rules.ActionDeny:
			return false
		case rules.ActionAsk:
			return true
		}
	}
	return g.inner.ShouldConfirm(toolName, input)
}

// IsDenied 委托 ApprovalPolicy.IsDenied。
func (g policyGate) IsDenied(toolName string, input map[string]any) bool {
	return g.policy.IsDenied(toolName, input)
}

// ShouldConfirm 委托 ApprovalPolicy.ShouldConfirm。
func (g policyGate) ShouldConfirm(toolName string, input map[string]any) bool {
	return g.policy.ShouldConfirm(toolName, input)
}

// bashCommandFromInput 从 bash 工具 input 提取 command 字符串。
func bashCommandFromInput(input map[string]any) string {
	if input == nil {
		return ""
	}
	v, ok := input["command"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
