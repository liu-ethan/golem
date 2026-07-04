package rules

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type rulesFile struct {
	Rules    []Rule `yaml:"rules"`
	Priority string `yaml:"priority"`
}

// Load 合并加载项目级与全局 rules.yaml，项目规则在前、全局在后；文件缺失时跳过。
func Load(projectRoot string) ([]Rule, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}

	paths := []string{
		filepath.Join(projectRoot, ".golem", "rules.yaml"),
		filepath.Join(home, ".golem", "rules.yaml"),
	}

	var merged []Rule
	for _, path := range paths {
		rules, err := loadFile(path)
		if err != nil {
			return nil, err
		}
		merged = append(merged, rules...)
	}
	return merged, nil
}

// loadFile 读取单个 rules.yaml；文件不存在时返回 nil 切片。
func loadFile(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read rules %s: %w", path, err)
	}
	var rf rulesFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parse rules %s: %w", path, err)
	}
	return rf.Rules, nil
}

// LoadPriority 返回合并后最后一份非空 priority 字段，供 /permissions 展示。
func LoadPriority(projectRoot string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	paths := []string{
		filepath.Join(projectRoot, ".golem", "rules.yaml"),
		filepath.Join(home, ".golem", "rules.yaml"),
	}
	var priority string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("read rules %s: %w", path, err)
		}
		var rf rulesFile
		if err := yaml.Unmarshal(data, &rf); err != nil {
			return "", fmt.Errorf("parse rules %s: %w", path, err)
		}
		if rf.Priority != "" {
			priority = rf.Priority
		}
	}
	return priority, nil
}
