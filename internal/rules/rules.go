package rules

import (
	"regexp"
	"strings"
)

// Action 表示 bash 命令匹配规则后的处置结果。
type Action string

const (
	ActionAllow Action = "allow"
	ActionAsk   Action = "ask"
	ActionDeny  Action = "deny"
)

// Rule 描述一条 bash 权限规则；Pattern 为 shell 通配符，* 匹配任意后缀。
type Rule struct {
	Action  string `yaml:"action"`
	Pattern string `yaml:"pattern"`
}

// MatchBash 对 command 收集所有命中规则，按 deny > ask > allow 返回最终 Action；无命中等同 allow。
// 匹配对象为 bash 工具的 command 参数字符串，不含 "bash -c" 前缀。
func MatchBash(command string, rules []Rule) Action {
	var hasAllow, hasAsk, hasDeny bool
	for _, r := range rules {
		if !matchPattern(r.Pattern, command) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(r.Action)) {
		case string(ActionDeny):
			hasDeny = true
		case string(ActionAsk):
			hasAsk = true
		case string(ActionAllow):
			hasAllow = true
		}
	}
	if hasDeny {
		return ActionDeny
	}
	if hasAsk {
		return ActionAsk
	}
	if hasAllow {
		return ActionAllow
	}
	return ActionAllow
}

// matchPattern 判断 s 是否匹配 shell 风格通配符 pattern（仅支持 *）。
func matchPattern(pattern, s string) bool {
	if pattern == "" {
		return false
	}
	var re strings.Builder
	re.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			re.WriteString(".*")
		case '.', '+', '?', '^', '$', '(', ')', '[', ']', '{', '}', '|', '\\':
			re.WriteByte('\\')
			re.WriteByte(pattern[i])
		default:
			re.WriteByte(pattern[i])
		}
	}
	re.WriteString("$")
	ok, err := regexp.MatchString(re.String(), s)
	return err == nil && ok
}
