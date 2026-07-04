package approval

import (
	"fmt"
	"slices"
)

// 四种审批交互模式，对齐 Claude Code / Codex CLI。
const (
	ModePlan              = "plan"
	ModeAskBeforeEdit     = "ask-before-edit"
	ModeAsk               = "ask"
	ModeEditAutomatically = "edit-automatically"
)

// Modes 为 Shift+Tab 循环顺序与校验用的合法模式列表。
var Modes = []string{
	ModePlan,
	ModeAskBeforeEdit,
	ModeAsk,
	ModeEditAutomatically,
}

// ApprovalPolicy 决定工具调用是否被拒绝或需要用户确认。
type ApprovalPolicy interface {
	ShouldConfirm(toolName string, input map[string]any) bool
	IsDenied(toolName string, input map[string]any) bool
	Mode() string
	SetMode(mode string) error
}

// Policy 实现四种审批模式的判定逻辑。
type Policy struct {
	mode string
}

// New 按模式名创建审批策略；非法模式返回 error。
func New(mode string) (*Policy, error) {
	if err := validateMode(mode); err != nil {
		return nil, err
	}
	return &Policy{mode: mode}, nil
}

// ShouldConfirm 报告执行指定工具前是否需弹确认框。
// plan 模式下写/bash 由 IsDenied 直接拒绝，此处恒为 false。
func (p *Policy) ShouldConfirm(toolName string, _ map[string]any) bool {
	switch p.mode {
	case ModePlan:
		return false
	case ModeAskBeforeEdit:
		return isMutatingTool(toolName)
	case ModeAsk:
		return true
	case ModeEditAutomatically:
		return false
	default:
		return false
	}
}

// IsDenied 报告工具调用是否被审批层直接拒绝（不弹确认框）。
// plan 模式下 write_file、edit_file、bash 返回 true。
func (p *Policy) IsDenied(toolName string, _ map[string]any) bool {
	if p.mode != ModePlan {
		return false
	}
	return isMutatingTool(toolName)
}

// Mode 返回当前审批模式名。
func (p *Policy) Mode() string {
	return p.mode
}

// SetMode 运行时切换审批模式；非法模式返回 error。
func (p *Policy) SetMode(mode string) error {
	if err := validateMode(mode); err != nil {
		return err
	}
	p.mode = mode
	return nil
}

// CycleMode 按 Modes 顺序切换到下一模式，供 Shift+Tab 使用。
func (p *Policy) CycleMode() string {
	for i, m := range Modes {
		if p.mode == m {
			p.mode = Modes[(i+1)%len(Modes)]
			return p.mode
		}
	}
	p.mode = Modes[0]
	return p.mode
}

// isMutatingTool 判断是否为写操作或 bash（需确认或 plan 下拒绝）。
func isMutatingTool(toolName string) bool {
	switch toolName {
	case "write_file", "edit_file", "bash":
		return true
	default:
		return false
	}
}

// validateMode 校验模式名是否在 Modes 列表中。
func validateMode(mode string) error {
	if slices.Contains(Modes, mode) {
		return nil
	}
	return fmt.Errorf("invalid approval mode %q (want one of: %v)", mode, Modes)
}
