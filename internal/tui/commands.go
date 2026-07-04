package tui

import (
	"fmt"
	"strings"

	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/sandbox"
	"github.com/tencent-docs/golem/internal/skills"
)

const helpText = `斜杠命令：
  /help                     列出命令与快捷键
  /permissions              权限页：approval + rules + Recently denied
  /permissions <mode>       直接设定 plan | ask-before-edit | ask | edit-automatically
  /sessions                 最近会话列表，Enter 恢复
  /status                   显示 model / approval / sandbox / session / tokens
  /model [model]            运行时切换 LLM 模型
  /clear                    清空上下文开新会话（保留 user_profile）
  /compact [instructions]   手动触发 Layer 0 压缩
  /context                  可视化 context 占用
  /diff                     显示 working tree git diff
  /sandbox [mode]           切换或设定 sandbox 模式
  /review [target]          对 working tree / commit 跑 code review
  /memories                 查看/管理 memory_facts
  /usage (/cost)            会话 token 统计
  /fork                     分叉当前会话到新 session
  /export [file]            导出当前会话为 markdown
  /rename [name]            重命名当前 session
  /plan <query>             单条 plan 模式 query
  /skills                   Skill 列表页
  /<skill-name> <提问>       选中 Skill 并提问（单次，/ 补全）
  /init                     生成 AGENTS.md 模板
  /exit                     结束会话并退出

Skill 扫描路径（优先级：项目 > 全局 > builtin）：
  · builtin（编译进 golem）
  · ~/.golem/skills/（golem skill install 安装目录）
  · <project>/.golem/skills/（项目级 Skill）

未显式选用 Skill 时，首条消息会按语义 BM25 匹配并将相关 Skill 渐进式写入 system prompt。

快捷键：
  Tab（/ 命令前缀）         补全斜杠命令与 Skill
  Shift+Tab                 循环 approval 模式
  Ctrl+L                    清屏（保留对话）
  Esc×2（空输入）           编辑上一条 user 消息
  Tab（流式中，非 / 前缀）   排队下一条输入
  Ctrl+G                    外部编辑器撰写输入
  Ctrl+C（流式中）          取消当前 LLM 轮次
  Ctrl+C（空闲）            等同 /exit
  Ctrl+D                    等同 /exit
  Y / Enter（确认框）       执行工具
  n / Esc（确认框）         拒绝工具`

// parseSlashCommand 解析斜杠命令字符串，返回命令名与原始参数列表。
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
		args = append(args, a)
	}
	return cmd, args
}

// dispatchSlash 处理斜杠命令，不送 LLM；loader 用于解析 /<skill-name> <提问>。
func dispatchSlash(input string, loader *skills.Loader) slashResult {
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
	case "status":
		return slashResult{handled: true, message: "__status__"}
	case "model":
		if len(args) >= 1 {
			return slashResult{handled: true, setModel: strings.Join(args, " ")}
		}
		return slashResult{handled: true, message: "__model__"}
	case "clear":
		return slashResult{handled: true, clearContext: true}
	case "compact":
		return slashResult{handled: true, compact: true, compactInstructions: strings.Join(args, " ")}
	case "context":
		return slashResult{handled: true, message: "__context__"}
	case "diff":
		return slashResult{handled: true, runAgent: "__diff__"}
	case "sandbox":
		if len(args) >= 1 {
			mode := normalizeSandboxMode(args[0])
			if err := sandboxValidateMode(mode); err != nil {
				return slashResult{handled: true, message: err.Error()}
			}
			return slashResult{handled: true, setSandbox: mode, message: "sandbox 已设为 " + mode}
		}
		return slashResult{handled: true, message: "__sandbox_cycle__"}
	case "review":
		target := strings.Join(args, " ")
		return slashResult{handled: true, runReview: true, reviewTarget: target}
	case "memories", "memory":
		return slashResult{handled: true, openPage: PageMemories}
	case "usage", "cost":
		return slashResult{handled: true, showUsage: true}
	case "fork":
		return slashResult{handled: true, fork: true}
	case "export":
		return slashResult{handled: true, doExport: true, exportPath: strings.Join(args, " ")}
	case "rename":
		if len(args) == 0 {
			return slashResult{handled: true, message: "用法: /rename <name>"}
		}
		return slashResult{handled: true, renameName: strings.Join(args, " ")}
	case "plan":
		query := strings.TrimSpace(strings.TrimPrefix(raw, "/plan"))
		if query == "" {
			return slashResult{handled: true, message: "用法: /plan <query>"}
		}
		return slashResult{handled: true, runPlan: query}
	case "skills", "skill":
		return slashResult{handled: true, openPage: PageSkills}
	case "init":
		return slashResult{handled: true, runInit: true, initWrite: true}
	case "exit", "quit":
		return slashResult{handled: true, quit: true}
	default:
		if loader != nil && !isSlashCommandName(cmd) {
			if skill, err := loader.LoadByName(cmd); err == nil {
				query := strings.TrimSpace(strings.Join(args, " "))
				if query == "" {
					return slashResult{
						handled: true,
						message: fmt.Sprintf("用法: /%s <提问>", skill.Name),
					}
				}
				return slashResult{handled: true, runSkill: skill.Name, skillQuery: query}
			}
		}
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

// normalizeSandboxMode 将用户输入映射为合法 sandbox 模式名。
func normalizeSandboxMode(raw string) string {
	switch raw {
	case "workspace-write", "workspace_write", "workspace":
		return string(sandbox.ModeWorkspaceWrite)
	case "danger-full-access", "danger_full_access", "danger":
		return string(sandbox.ModeDangerFullAccess)
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

// sandboxValidateMode 校验 sandbox 模式名是否合法。
func sandboxValidateMode(mode string) error {
	switch sandbox.SandboxMode(mode) {
	case sandbox.ModeWorkspaceWrite, sandbox.ModeDangerFullAccess:
		return nil
	default:
		return fmt.Errorf("invalid sandbox mode %q (want: workspace-write | danger-full-access)", mode)
	}
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

// cycleSandboxMode 循环 sandbox 模式。
func cycleSandboxMode(current string) string {
	if current == string(sandbox.ModeDangerFullAccess) {
		return string(sandbox.ModeWorkspaceWrite)
	}
	return string(sandbox.ModeDangerFullAccess)
}
