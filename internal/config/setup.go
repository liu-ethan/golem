package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// NeedsProviderSetup 判断 LLM provider 是否尚未配置可用 api_key。
func NeedsProviderSetup(cfg Config) bool {
	return strings.TrimSpace(cfg.Provider.APIKey) == ""
}

// EnsureProjectConfig 在 <projectRoot>/.golem/config.yaml 不存在时写入内置默认配置。
// 返回 created 表示本次是否新建了文件。
func EnsureProjectConfig(projectRoot string) (created bool, err error) {
	dir := filepath.Join(projectRoot, ".golem")
	path := filepath.Join(dir, "config.yaml")
	if _, statErr := os.Stat(path); statErr == nil {
		return false, nil
	} else if !os.IsNotExist(statErr) {
		return false, fmt.Errorf("stat config %s: %w", path, statErr)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	cfg := defaultConfig()
	cfg.Provider.APIKey = ""
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return false, fmt.Errorf("marshal default config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, fmt.Errorf("write config %s: %w", path, err)
	}
	return true, nil
}

// SaveProviderConfig 将 provider 字段写入 <projectRoot>/.golem/config.yaml，保留 defaults / memory 等其它段。
func SaveProviderConfig(projectRoot string, provider ProviderConfig) error {
	if strings.TrimSpace(provider.APIKey) == "" {
		return fmt.Errorf("api_key is required")
	}
	dir := filepath.Join(projectRoot, ".golem")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, "config.yaml")

	cfg := defaultConfig()
	existing, err := loadYAMLMap(path)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		raw, err := yaml.Marshal(existing)
		if err != nil {
			return fmt.Errorf("marshal existing config: %w", err)
		}
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("decode existing config: %w", err)
		}
	}

	cfg.Provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	cfg.Provider.APIKey = strings.TrimSpace(provider.APIKey)
	cfg.Provider.Model = strings.TrimSpace(provider.Model)
	if provider.ContextLimit > 0 {
		cfg.Provider.ContextLimit = provider.ContextLimit
	}
	applyDefaults(&cfg)

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}
