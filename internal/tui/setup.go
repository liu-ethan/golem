package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/tui/style"
)

const (
	setupStepBaseURL = iota
	setupStepAPIKey
	setupStepModel
	setupStepCount
)

type setupPrompt struct {
	title       string
	hint        string
	defaultText string
}

func setupPrompts(defaultBaseURL, defaultModel string) []setupPrompt {
	return []setupPrompt{
		{
			title:       "Base URL",
			hint:        "Anthropic 兼容 API 端点",
			defaultText: defaultBaseURL,
		},
		{
			title:       "API Key",
			hint:        "Anthropic 格式密钥（必填）",
			defaultText: "",
		},
		{
			title:       "Model",
			hint:        "Anthropic 模型名，如 claude-sonnet-4-5",
			defaultText: defaultModel,
		},
	}
}

func (m Model) currentSetupPrompt() setupPrompt {
	prompts := setupPrompts(m.setupDefaultBaseURL, m.setupDefaultModel)
	if m.setupStep < 0 || m.setupStep >= len(prompts) {
		return setupPrompt{}
	}
	return prompts[m.setupStep]
}

// renderSetupPanel 渲染首次 LLM 配置向导。
func renderSetupPanel(m Model, width, height int) string {
	if width < 40 {
		width = 40
	}
	prompt := m.currentSetupPrompt()
	stepLabel := fmt.Sprintf("  %d / %d", m.setupStep+1, setupStepCount)

	var body strings.Builder
	body.WriteString(style.Emphasis.Render("  首次配置 LLM"))
	body.WriteString("\n\n")
	body.WriteString(style.Accent.Render(stepLabel))
	body.WriteString("  ")
	body.WriteString(style.AccentAlt.Bold(true).Render(prompt.title))
	body.WriteString("\n\n")
	body.WriteString(style.Muted.Render("  " + prompt.hint))
	body.WriteString("\n")
	if prompt.defaultText != "" {
		body.WriteString(style.Muted.Render("  默认: "))
		body.WriteString(style.PathText.Render(prompt.defaultText))
		body.WriteString("\n")
	}
	if m.setupErrMsg != "" {
		body.WriteString("\n")
		body.WriteString(style.ErrText.Render("  " + m.setupErrMsg))
	}

	boxed := style.WelcomeBorder.
		Padding(1, 2).
		Width(width - 2).
		Render(body.String())

	footer := style.Accent.
		Bold(true).
		Render("\n  [Enter] 继续  ·  [Ctrl+C] 退出")

	inputLine := "\n" + renderInputLine(m.input, false, m.showCursor)

	padTop := (height - lipgloss.Height(boxed+footer+inputLine) - 2) / 2
	if padTop < 0 {
		padTop = 0
	}
	return strings.Repeat("\n", padTop) + boxed + footer + inputLine
}

func (m Model) handleSetupKey(msg tea.KeyMsg, key string) (Model, tea.Cmd) {
	switch key {
	case "enter":
		return m.advanceSetupStep()
	case "ctrl+c", "ctrl+d":
		return m.quit()
	case "backspace":
		if len(m.input) > 0 {
			r := []rune(m.input)
			m.input = string(r[:len(r)-1])
		}
	default:
		if len(msg.Runes) > 0 {
			m.input += string(msg.Runes)
		}
	}
	return m, nil
}

func (m Model) advanceSetupStep() (Model, tea.Cmd) {
	value := strings.TrimSpace(m.input)
	prompt := m.currentSetupPrompt()

	switch m.setupStep {
	case setupStepBaseURL:
		if value == "" {
			value = prompt.defaultText
		}
		m.setupBaseURL = value
	case setupStepAPIKey:
		if value == "" {
			m.setupErrMsg = "API Key 不能为空"
			return m, nil
		}
		m.setupAPIKey = value
	case setupStepModel:
		if value == "" {
			value = prompt.defaultText
		}
		m.setupModel = value
		return m.finishSetup()
	}

	m.setupStep++
	m.input = ""
	m.setupErrMsg = ""
	return m, nil
}

func (m Model) finishSetup() (Model, tea.Cmd) {
	if err := config.SaveProviderConfig(m.projectRoot, config.ProviderConfig{
		BaseURL:      m.setupBaseURL,
		APIKey:       m.setupAPIKey,
		Model:        m.setupModel,
		ContextLimit: m.status.ContextLimit,
	}); err != nil {
		m.setupErrMsg = err.Error()
		return m, nil
	}
	if err := m.agent.ConfigureProvider(m.setupBaseURL, m.setupAPIKey, m.setupModel); err != nil {
		m.setupErrMsg = err.Error()
		return m, nil
	}
	if ac, ok := m.llmClient.(interface {
		Configure(baseURL, apiKey, model string)
	}); ok {
		ac.Configure(m.setupBaseURL, m.setupAPIKey, m.setupModel)
	}
	m.status.Model = m.setupModel
	m.needsSetup = false
	m.setupErrMsg = ""
	m.input = ""
	m.activePage = PageChat
	return m, nil
}
