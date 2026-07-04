package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type rulesFile struct {
	Rules []struct {
		Action  string `yaml:"action"`
		Pattern string `yaml:"pattern"`
	} `yaml:"rules"`
	Priority string `yaml:"priority"`
}

// LoadRulesDisplay 合并加载项目级与全局 rules.yaml，格式化为 /permissions 页只读展示行。
func LoadRulesDisplay(projectRoot string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}

	paths := []string{
		filepath.Join(home, ".golem", "rules.yaml"),
		filepath.Join(projectRoot, ".golem", "rules.yaml"),
	}

	var merged rulesFile
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read rules %s: %w", path, err)
		}
		var rf rulesFile
		if err := yaml.Unmarshal(data, &rf); err != nil {
			return nil, fmt.Errorf("parse rules %s: %w", path, err)
		}
		merged.Rules = append(merged.Rules, rf.Rules...)
		if rf.Priority != "" {
			merged.Priority = rf.Priority
		}
	}

	if len(merged.Rules) == 0 {
		return []string{"（未找到 rules.yaml，P0 等同全部 allow）"}, nil
	}

	lines := make([]string, 0, len(merged.Rules)+2)
	if merged.Priority != "" {
		lines = append(lines, "priority: "+merged.Priority)
		lines = append(lines, "")
	}
	for _, r := range merged.Rules {
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
	return strings.Join(parts, "  ")
}
