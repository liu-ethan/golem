package tui

import (
	"fmt"
	"strings"

	"github.com/tencent-docs/golem/internal/approval"
)

const helpText = `P0 斜杠命令：
  /help                     列出命令与快捷键
  /permissions              权限页：切换 approval + 查看 rules
  /permissions <mode>       直接设定 plan | ask-before-edit | ask | edit-automatically
  /sessions                 最近会话列表，Enter 恢复
  /exit                     结束会话并退出

快捷键：
  Shift+Tab                 循环 approval 模式
  Ctrl+C（流式中）          取消当前 LLM 轮次
  Ctrl+C（空闲）            等同 /exit
  Ctrl+C（确认框）          等同 n（拒绝）
  Ctrl+D                    等同 /exit
  Y / Enter（确认框）       执行工具
  n / Esc（确认框）         拒绝工具`

// parseSlashCommand 解析斜杠命令字符串，返回命令名与小写参数列表。
func parseSlashCommand(raw string) (cmd string, args []string) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "/") {
		return "", nil
	}
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return "", nil
	}
	cmd = strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	for _, a := range fields[1:] {
		args = append(args, strings.ToLower(a))
	}
	return cmd, args
}

// dispatchSlash 处理 P0 斜杠命令，不送 LLM。
func dispatchSlash(input string) slashResult {
	raw := strings.TrimSpace(input)
	if !strings.HasPrefix(raw, "/") {
		return slashResult{handled: false}
	}
	cmd, args := parseSlashCommand(input)
	if cmd == "" {
		return slashResult{handled: true, message: "未知命令: /（输入 /help 查看）"}
	}
	switch cmd {
	case "help", "h", "?":
		return slashResult{handled: true, message: helpText}
	case "permissions", "permission", "perms":
		if len(args) >= 1 {
			mode := normalizeApprovalMode(args[0])
			if err := approvalValidateMode(mode); err != nil {
				return slashResult{handled: true, message: err.Error()}
			}
			return slashResult{handled: true, setMode: mode, message: "approval 模式已设为 " + mode}
		}
		return slashResult{handled: true, openPage: PagePermissions}
	case "sessions", "session":
		return slashResult{handled: true, openPage: PageSessions}
	case "exit", "quit":
		return slashResult{handled: true, quit: true}
	default:
		return slashResult{handled: true, message: fmt.Sprintf("未知命令: /%s（输入 /help 查看）", cmd)}
	}
}

// normalizeApprovalMode 将用户输入映射为合法 approval 模式名。
func normalizeApprovalMode(raw string) string {
	switch raw {
	case "plan":
		return approval.ModePlan
	case "ask-before-edit", "ask_before_edit", "askbeforeedit":
		return approval.ModeAskBeforeEdit
	case "ask":
		return approval.ModeAsk
	case "edit-automatically", "edit_automatically", "auto", "automatically":
		return approval.ModeEditAutomatically
	default:
		return raw
	}
}

// approvalValidateMode 校验模式名是否合法。
func approvalValidateMode(mode string) error {
	for _, m := range approval.Modes {
		if m == mode {
			return nil
		}
	}
	return fmt.Errorf("invalid approval mode %q (want: plan | ask-before-edit | ask | edit-automatically)", mode)
}

// formatTokens 将 token 计数格式化为状态栏展示字符串。
func formatTokens(input, limit int) string {
	if limit <= 0 {
		limit = 200000
	}
	return fmt.Sprintf("%s / %s", humanTokenCount(input), humanTokenCount(limit))
}

// humanTokenCount 将 token 数格式化为 k 单位可读字符串。
func humanTokenCount(n int) string {
	if n >= 1000 {
		f := float64(n) / 1000.0
		if f >= 100 {
			return fmt.Sprintf("%.0fk", f)
		}
		return fmt.Sprintf("%.1fk", f)
	}
	return fmt.Sprintf("%d", n)
}
