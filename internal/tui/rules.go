package tui

import (
	"fmt"
	"strings"

	"github.com/tencent-docs/golem/internal/rules"
)

// LoadRulesDisplay 合并加载项目级与全局 rules.yaml，格式化为 /permissions 页只读展示行。
func LoadRulesDisplay(projectRoot string) ([]string, error) {
	merged, err := rules.Load(projectRoot)
	if err != nil {
		return nil, err
	}
	priority, err := rules.LoadPriority(projectRoot)
	if err != nil {
		return nil, err
	}

	if len(merged) == 0 {
		return []string{"（未找到 rules.yaml，P0 等同全部 allow）"}, nil
	}

	lines := make([]string, 0, len(merged)+2)
	if priority != "" {
		lines = append(lines, "priority: "+priority)
		lines = append(lines, "")
	}
	for _, r := range merged {
		lines = append(lines, fmt.Sprintf("%-5s %s", r.Action+":", r.Pattern))
	}
	return lines, nil
}

// formatToolInput 将工具 input map 格式化为单行摘要。
func formatToolInput(input map[string]any) string {
	if len(input) == 0 {
		return ""
	}
	parts := make([]string, 0, len(input))
	for k, v := range input {
		parts = append(parts, fmt.Sprintf("%s: %v", k, v))
	}
	return strings.Join(parts, " ")
}
