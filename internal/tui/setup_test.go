package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tencent-docs/golem/internal/agent"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/testutil"
)

func testSetupModel(t *testing.T) Model {
	t.Helper()
	root := testutil.TempProjectRoot(t)
	mock := testutil.NewMockLLM()
	policy, err := approval.New(approval.ModePlan)
	if err != nil {
		t.Fatal(err)
	}
	ag, err := agent.New(root, mock, agent.Options{Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	m := NewModel(Config{
		ProjectRoot:      root,
		Agent:            ag,
		Policy:           policy,
		Sandbox:          "workspace-write",
		ContextLimit:     200000,
		LLMClient:        mock,
		NeedsSetup:       true,
		DefaultBaseURL:   "https://api.anthropic.com",
		DefaultModel:     "claude-sonnet-4-5",
	})
	m.activePage = PageSetup
	m.setupStep = setupStepBaseURL
	return m
}

func TestWelcomeEnterOpensSetupWhenNeeded(t *testing.T) {
	m := testSetupModel(t)
	m.activePage = PageWelcome
	m.needsSetup = true

	next, _ := m.handleWelcomeKey("enter")
	if next.activePage != PageSetup {
		t.Fatalf("page = %v, want PageSetup", next.activePage)
	}
	if next.setupStep != setupStepBaseURL {
		t.Fatalf("setupStep = %d", next.setupStep)
	}
}

func TestSetupWizardCompletesAndWritesConfig(t *testing.T) {
	m := testSetupModel(t)

	next, _ := m.handleSetupKey(tea.KeyMsg{Type: tea.KeyEnter}, "enter")
	m = next
	if m.setupStep != setupStepAPIKey {
		t.Fatalf("step = %d, want api key step", m.setupStep)
	}
	if m.setupBaseURL != "https://api.anthropic.com" {
		t.Fatalf("baseURL = %q", m.setupBaseURL)
	}

	m.input = "sk-test-key"
	next, _ = m.handleSetupKey(tea.KeyMsg{Type: tea.KeyEnter}, "enter")
	m = next
	if m.setupStep != setupStepModel {
		t.Fatalf("step = %d, want model step", m.setupStep)
	}

	next, _ = m.handleSetupKey(tea.KeyMsg{Type: tea.KeyEnter}, "enter")
	m = next
	if m.activePage != PageChat {
		t.Fatalf("page = %v, want PageChat", m.activePage)
	}
	if m.needsSetup {
		t.Fatal("needsSetup should be cleared")
	}

	cfg, err := config.LoadConfig(m.projectRoot, config.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider.APIKey != "sk-test-key" {
		t.Errorf("APIKey = %q", cfg.Provider.APIKey)
	}
	if cfg.Provider.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q", cfg.Provider.Model)
	}
}

func TestSetupRejectsEmptyAPIKey(t *testing.T) {
	m := testSetupModel(t)
	m.setupStep = setupStepAPIKey

	next, _ := m.handleSetupKey(tea.KeyMsg{Type: tea.KeyEnter}, "enter")
	if next.setupErrMsg == "" {
		t.Fatal("expected error for empty api key")
	}
	if next.setupStep != setupStepAPIKey {
		t.Fatalf("step advanced unexpectedly to %d", next.setupStep)
	}
}

func TestRenderSetupPanelShowsPrompt(t *testing.T) {
	m := testSetupModel(t)
	out := renderSetupPanel(m, 80, 24)
	if !strings.Contains(out, "Base URL") {
		t.Fatalf("missing prompt: %s", out)
	}
	if !strings.Contains(out, "1 / 3") {
		t.Fatal("missing step indicator")
	}
}

func TestEnsureProjectConfigFromCLIPath(t *testing.T) {
	root := t.TempDir()
	golemDir := filepath.Join(root, ".golem")
	if err := os.MkdirAll(golemDir, 0o755); err != nil {
		t.Fatal(err)
	}
	created, err := config.EnsureProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected config file creation")
	}
	if _, err := os.Stat(filepath.Join(golemDir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestConfigureProviderUpdatesAnthropicClient(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	client := llm.NewAnthropicClient("https://old.example.com", "", "old-model")
	policy, _ := approval.New(approval.ModePlan)
	ag, err := agent.New(root, client, agent.Options{Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	if err := ag.ConfigureProvider("https://new.example.com", "sk-new", "new-model"); err != nil {
		t.Fatal(err)
	}
	if client.Model() != "new-model" {
		t.Fatalf("model = %q", client.Model())
	}
}
