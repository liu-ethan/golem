package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProviderConfig 描述 LLM 接入端点与模型参数。
type ProviderConfig struct {
	BaseURL      string `yaml:"base_url"`
	APIKey       string `yaml:"api_key"`
	Model        string `yaml:"model"`
	ContextLimit int    `yaml:"context_limit"`
}

// DefaultsConfig 描述启动时的默认审批与沙箱模式。
type DefaultsConfig struct {
	Approval string `yaml:"approval"`
	Sandbox  string `yaml:"sandbox"`
}

// MemoryConfig 描述记忆系统相关阈值，P1 起由 agent / memory 包消费。
type MemoryConfig struct {
	Layer2SessionThreshold int     `yaml:"layer2_session_threshold"`
	BM25TopK               int     `yaml:"bm25_top_k"`
	CompactBatchSize       int     `yaml:"compact_batch_size"`
	CompactThreshold       float64 `yaml:"compact_threshold"`
}

// Config 为 golem 运行时完整配置快照。
type Config struct {
	Provider ProviderConfig `yaml:"provider"`
	Defaults DefaultsConfig `yaml:"defaults"`
	Memory   MemoryConfig   `yaml:"memory"`
}

// Overrides 承载 CLI flag 对 defaults 的覆盖；空字符串表示不覆盖。
type Overrides struct {
	Approval string
	Sandbox  string
}

var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// LoadConfig 加载并合并 LLM 与默认行为配置。
// 读取顺序：内置默认值 → ~/.golem/config.yaml（若存在）→ <projectRoot>/.golem/config.yaml（若存在）；
// overrides 中非空的 Approval / Sandbox 覆盖 defaults 同名字段；加载时将 ${ENV_VAR} 占位符展开为环境变量值。
func LoadConfig(projectRoot string, overrides Overrides) (Config, error) {
	cfg := defaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve home dir: %w", err)
	}

	globalPath := filepath.Join(home, ".golem", "config.yaml")
	projectPath := filepath.Join(projectRoot, ".golem", "config.yaml")

	merged, err := loadYAMLMap(globalPath)
	if err != nil {
		return Config{}, err
	}
	projectMap, err := loadYAMLMap(projectPath)
	if err != nil {
		return Config{}, err
	}
	merged = deepMergeMaps(merged, projectMap)

	raw, err := yaml.Marshal(merged)
	if err != nil {
		return Config{}, fmt.Errorf("marshal merged config: %w", err)
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode merged config: %w", err)
	}

	applyDefaults(&cfg)
	applyOverrides(&cfg, overrides)
	if err := expandEnvVars(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// defaultConfig 返回内置默认配置，在无 YAML 文件时使用。
func defaultConfig() Config {
	return Config{
		Provider: ProviderConfig{
			BaseURL:      "https://api.anthropic.com",
			Model:        "claude-sonnet-4-5",
			ContextLimit: 200000,
		},
		Defaults: DefaultsConfig{
			Approval: "ask-before-edit",
			Sandbox:  "workspace-write",
		},
		Memory: MemoryConfig{
			Layer2SessionThreshold: 3,
			BM25TopK:               5,
			CompactBatchSize:       10,
			CompactThreshold:       0.8,
		},
	}
}

// loadYAMLMap 读取 YAML 文件为 map；文件不存在时返回空 map。
func loadYAMLMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if m == nil {
		return map[string]any{}, nil
	}
	return m, nil
}

// deepMergeMaps 将 overlay 递归合并进 base，同 key 的 map 继续递归，标量/数组由 overlay 覆盖。
func deepMergeMaps(base, overlay map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for k, ov := range overlay {
		bv, ok := base[k]
		if !ok {
			base[k] = ov
			continue
		}
		bm, bOK := bv.(map[string]any)
		om, oOK := ov.(map[string]any)
		if bOK && oOK {
			base[k] = deepMergeMaps(bm, om)
			continue
		}
		base[k] = ov
	}
	return base
}

// applyDefaults 为零值字段回填内置默认值（YAML 未显式设置时）。
func applyDefaults(cfg *Config) {
	def := defaultConfig()
	if cfg.Provider.BaseURL == "" {
		cfg.Provider.BaseURL = def.Provider.BaseURL
	}
	if cfg.Provider.Model == "" {
		cfg.Provider.Model = def.Provider.Model
	}
	if cfg.Provider.ContextLimit == 0 {
		cfg.Provider.ContextLimit = def.Provider.ContextLimit
	}
	if cfg.Defaults.Approval == "" {
		cfg.Defaults.Approval = def.Defaults.Approval
	}
	if cfg.Defaults.Sandbox == "" {
		cfg.Defaults.Sandbox = def.Defaults.Sandbox
	}
	if cfg.Memory.Layer2SessionThreshold == 0 {
		cfg.Memory.Layer2SessionThreshold = def.Memory.Layer2SessionThreshold
	}
	if cfg.Memory.BM25TopK == 0 {
		cfg.Memory.BM25TopK = def.Memory.BM25TopK
	}
	if cfg.Memory.CompactBatchSize == 0 {
		cfg.Memory.CompactBatchSize = def.Memory.CompactBatchSize
	}
	if cfg.Memory.CompactThreshold == 0 {
		cfg.Memory.CompactThreshold = def.Memory.CompactThreshold
	}
}

// applyOverrides 将 CLI flag 覆盖写入 defaults。
func applyOverrides(cfg *Config, overrides Overrides) {
	if overrides.Approval != "" {
		cfg.Defaults.Approval = overrides.Approval
	}
	if overrides.Sandbox != "" {
		cfg.Defaults.Sandbox = overrides.Sandbox
	}
}

// expandEnvVars 展开配置中所有字符串字段的 ${ENV_VAR} 占位符。
func expandEnvVars(cfg *Config) error {
	var err error
	cfg.Provider.BaseURL, err = expandString(cfg.Provider.BaseURL)
	if err != nil {
		return err
	}
	cfg.Provider.APIKey, err = expandString(cfg.Provider.APIKey)
	if err != nil {
		return err
	}
	cfg.Provider.Model, err = expandString(cfg.Provider.Model)
	if err != nil {
		return err
	}
	cfg.Defaults.Approval, err = expandString(cfg.Defaults.Approval)
	if err != nil {
		return err
	}
	cfg.Defaults.Sandbox, err = expandString(cfg.Defaults.Sandbox)
	if err != nil {
		return err
	}
	return nil
}

// expandString 将 s 中所有 ${VAR} 替换为 os.Getenv(VAR)；未设置时返回错误。
func expandString(s string) (string, error) {
	var missing []string
	out := envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-1]
		val, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
			return match
		}
		return val
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("environment variable(s) not set: %s", strings.Join(missing, ", "))
	}
	return out, nil
}
