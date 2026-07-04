package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tencent-docs/golem/internal/skills"
)

// SlashSuggestion 表示一条斜杠命令或 Skill 补全候选项。
type SlashSuggestion struct {
	Name string
	Desc string
}

// slashCommandCatalog 列出所有可补全斜杠命令及其说明。
var slashCommandCatalog = []SlashSuggestion{
	{Name: "help", Desc: "列出命令与快捷键"},
	{Name: "permissions", Desc: "权限页：approval + rules"},
	{Name: "sessions", Desc: "最近会话列表，Enter 恢复"},
	{Name: "status", Desc: "显示 model / approval / sandbox / session"},
	{Name: "model", Desc: "运行时切换 LLM 模型"},
	{Name: "clear", Desc: "清空上下文开新会话"},
	{Name: "compact", Desc: "手动触发 Layer 0 压缩"},
	{Name: "context", Desc: "可视化 context 占用"},
	{Name: "diff", Desc: "显示 working tree git diff"},
	{Name: "sandbox", Desc: "切换或设定 sandbox 模式"},
	{Name: "review", Desc: "对 working tree / commit 跑 code review"},
	{Name: "memories", Desc: "查看/管理 memory_facts"},
	{Name: "usage", Desc: "会话 token 统计"},
	{Name: "fork", Desc: "分叉当前会话到新 session"},
	{Name: "export", Desc: "导出当前会话为 markdown"},
	{Name: "rename", Desc: "重命名当前 session"},
	{Name: "plan", Desc: "单条 plan 模式 query"},
	{Name: "skills", Desc: "Skill 列表页"},
	{Name: "init", Desc: "生成 AGENTS.md 模板"},
	{Name: "exit", Desc: "结束会话并退出"},
}

// matchSlashSuggestions 根据当前输入前缀返回斜杠命令与 Skill 补全列表。
// 仅当 input 以 / 开头且尚未输入参数时返回候选项；命令在前，Skill 在后。
func matchSlashSuggestions(input string, loader *skills.Loader) []SlashSuggestion {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return nil
	}
	rest := strings.TrimPrefix(input, "/")
	if strings.Contains(rest, " ") {
		return nil
	}
	prefix := strings.ToLower(rest)

	var cmdMatches []SlashSuggestion
	if prefix == "" {
		out := make([]SlashSuggestion, len(slashCommandCatalog))
		copy(out, slashCommandCatalog)
		cmdMatches = out
	} else {
		for _, cmd := range slashCommandCatalog {
			if strings.HasPrefix(cmd.Name, prefix) {
				cmdMatches = append(cmdMatches, cmd)
			}
		}
		sort.Slice(cmdMatches, func(i, j int) bool {
			return len(cmdMatches[i].Name) < len(cmdMatches[j].Name)
		})
	}

	skillMatches := matchSkillSuggestions(loader, prefix)
	if len(skillMatches) == 0 {
		return cmdMatches
	}
	out := make([]SlashSuggestion, 0, len(cmdMatches)+len(skillMatches))
	out = append(out, cmdMatches...)
	out = append(out, skillMatches...)
	return out
}

// matchSkillSuggestions 返回与 prefix 匹配的 Skill 补全项。
func matchSkillSuggestions(loader *skills.Loader, prefix string) []SlashSuggestion {
	if loader == nil {
		return nil
	}
	list, err := loader.List()
	if err != nil {
		return nil
	}
	var matches []SlashSuggestion
	for _, s := range list {
		name := strings.ToLower(s.Name)
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		matches = append(matches, SlashSuggestion{
			Name: s.Name,
			Desc: skillSuggestionDesc(s),
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Name < matches[j].Name
	})
	return matches
}

// skillSuggestionDesc 生成 Skill 补全行的说明文字。
func skillSuggestionDesc(s skills.Skill) string {
	return fmt.Sprintf("选中并提问 (%s)", s.Source)
}

// resolveSlashInput 在提交前将部分输入解析为完整斜杠命令或 Skill 名。
func resolveSlashInput(input string, sel int, suggestions []SlashSuggestion) string {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") || len(suggestions) == 0 {
		return input
	}
	if strings.Contains(strings.TrimPrefix(input, "/"), " ") {
		return input
	}
	idx := sel
	if idx < 0 || idx >= len(suggestions) {
		idx = 0
	}
	return "/" + suggestions[idx].Name
}

// completeSlashInput 将当前输入补全为选中的斜杠命令或 Skill 并追加空格。
func completeSlashInput(input string, sel int, suggestions []SlashSuggestion) string {
	resolved := resolveSlashInput(input, sel, suggestions)
	if !strings.HasSuffix(resolved, " ") {
		return resolved + " "
	}
	return resolved
}

// isSlashCommandName 判断 name 是否为内置斜杠命令（优先于 Skill 同名解析）。
func isSlashCommandName(name string) bool {
	lower := strings.ToLower(name)
	for _, cmd := range slashCommandCatalog {
		if cmd.Name == lower {
			return true
		}
	}
	// parseSlashCommand 中的别名
	switch lower {
	case "h", "?", "permission", "perms", "memory", "cost", "quit", "skill":
		return true
	}
	return false
}
