package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNeedsProviderSetupEmptyAPIKey(t *testing.T) {
	cfg := defaultConfig()
	cfg.Provider.APIKey = ""
	if !NeedsProviderSetup(cfg) {
		t.Fatal("expected setup when api_key empty")
	}
	cfg.Provider.APIKey = "sk-test"
	if NeedsProviderSetup(cfg) {
		t.Fatal("expected no setup when api_key set")
	}
}

func TestEnsureProjectConfigCreatesDefaultFile(t *testing.T) {
	root := t.TempDir()
	created, err := EnsureProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected created=true on first call")
	}
	path := filepath.Join(root, ".golem", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "base_url:") {
		t.Fatalf("missing base_url: %s", content)
	}
	if !strings.Contains(content, "ask-before-edit") {
		t.Fatalf("missing defaults: %s", content)
	}

	createdAgain, err := EnsureProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if createdAgain {
		t.Fatal("expected created=false when file exists")
	}
}

func TestSaveProviderConfigWritesProjectYAML(t *testing.T) {
	root := t.TempDir()
	if _, err := EnsureProjectConfig(root); err != nil {
		t.Fatal(err)
	}
	err := SaveProviderConfig(root, ProviderConfig{
		BaseURL: "https://api.example.com",
		APIKey:  "sk-secret",
		Model:   "claude-sonnet-4-5",
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(root, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %q", cfg.Provider.BaseURL)
	}
	if cfg.Provider.APIKey != "sk-secret" {
		t.Errorf("APIKey = %q", cfg.Provider.APIKey)
	}
	if cfg.Provider.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q", cfg.Provider.Model)
	}
	if cfg.Defaults.Approval != "ask-before-edit" {
		t.Errorf("Approval = %q", cfg.Defaults.Approval)
	}
}

func TestSaveProviderConfigRejectsEmptyAPIKey(t *testing.T) {
	root := t.TempDir()
	err := SaveProviderConfig(root, ProviderConfig{BaseURL: "https://a.com", Model: "m"})
	if err == nil {
		t.Fatal("expected error for empty api_key")
	}
}
