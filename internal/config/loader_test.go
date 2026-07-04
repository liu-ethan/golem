package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFieldLevelMerge(t *testing.T) {
	t.Setenv("GOLEM_TEST_KEY", "from-env")

	root := t.TempDir()
	home := filepath.Join(root, "home")
	projectRoot := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(home, ".golem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, ".golem"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)

	globalYAML := `
provider:
  base_url: "https://global.example.com"
  api_key: "${GOLEM_TEST_KEY}"
  model: "global-model"
defaults:
  approval: plan
  sandbox: danger-full-access
memory:
  bm25_top_k: 10
`
	projectYAML := `
provider:
  model: "project-model"
defaults:
  approval: ask
memory:
  compact_batch_size: 20
`
	if err := os.WriteFile(filepath.Join(home, ".golem", "config.yaml"), []byte(globalYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".golem", "config.yaml"), []byte(projectYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(projectRoot, Overrides{Sandbox: "workspace-write"})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Provider.BaseURL != "https://global.example.com" {
		t.Errorf("BaseURL = %q, want global base", cfg.Provider.BaseURL)
	}
	if cfg.Provider.APIKey != "from-env" {
		t.Errorf("APIKey = %q, want expanded env", cfg.Provider.APIKey)
	}
	if cfg.Provider.Model != "project-model" {
		t.Errorf("Model = %q, want project override", cfg.Provider.Model)
	}
	if cfg.Defaults.Approval != "ask" {
		t.Errorf("Approval = %q, want project override before CLI", cfg.Defaults.Approval)
	}
	if cfg.Defaults.Sandbox != "workspace-write" {
		t.Errorf("Sandbox = %q, want CLI override", cfg.Defaults.Sandbox)
	}
	if cfg.Memory.BM25TopK != 10 {
		t.Errorf("BM25TopK = %d, want global value", cfg.Memory.BM25TopK)
	}
	if cfg.Memory.CompactBatchSize != 20 {
		t.Errorf("CompactBatchSize = %d, want project value", cfg.Memory.CompactBatchSize)
	}
	if cfg.Memory.Layer2SessionThreshold != 3 {
		t.Errorf("Layer2SessionThreshold = %d, want builtin default", cfg.Memory.Layer2SessionThreshold)
	}
}

func TestLoadConfigDefaultsWithoutFiles(t *testing.T) {
	root := t.TempDir()
	cfg, err := LoadConfig(root, Overrides{})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Provider.BaseURL != "https://api.anthropic.com" {
		t.Errorf("BaseURL = %q", cfg.Provider.BaseURL)
	}
	if cfg.Defaults.Approval != "ask-before-edit" {
		t.Errorf("Approval = %q", cfg.Defaults.Approval)
	}
}

func TestLoadConfigMissingEnvVar(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".golem"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := `provider:
  api_key: "${GOLEM_MISSING_VAR_XYZ}"
`
	if err := os.WriteFile(filepath.Join(home, ".golem", "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(root, Overrides{})
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
}

func TestLoadConfigCLIOverridesDefaults(t *testing.T) {
	root := t.TempDir()
	cfg, err := LoadConfig(root, Overrides{
		Approval: "edit-automatically",
		Sandbox:  "danger-full-access",
	})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Defaults.Approval != "edit-automatically" {
		t.Errorf("Approval = %q", cfg.Defaults.Approval)
	}
	if cfg.Defaults.Sandbox != "danger-full-access" {
		t.Errorf("Sandbox = %q", cfg.Defaults.Sandbox)
	}
}

func TestExpandString(t *testing.T) {
	t.Setenv("FOO", "bar")
	got, err := expandString("prefix-${FOO}-suffix")
	if err != nil {
		t.Fatal(err)
	}
	if got != "prefix-bar-suffix" {
		t.Errorf("got %q", got)
	}
}

func TestDeepMergeMaps(t *testing.T) {
	base := map[string]any{
		"provider": map[string]any{
			"base_url": "https://a.com",
			"model":    "m1",
		},
		"defaults": map[string]any{
			"approval": "plan",
		},
	}
	overlay := map[string]any{
		"provider": map[string]any{
			"model": "m2",
		},
	}
	merged := deepMergeMaps(base, overlay)
	provider := merged["provider"].(map[string]any)
	if provider["base_url"] != "https://a.com" {
		t.Errorf("base_url lost during merge")
	}
	if provider["model"] != "m2" {
		t.Errorf("model = %v, want m2", provider["model"])
	}
}
