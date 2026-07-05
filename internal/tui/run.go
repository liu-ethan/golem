package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Run 启动 Bubble Tea TUI，阻塞直到用户退出。
func Run(cfg Config) error {
	return runProgram(cfg, tea.WithAltScreen(), tea.WithReportFocus())
}

// finalizeSession 在 TUI 退出后执行会话收尾（持久化、Layer 1 记忆提取等）。
// 须在 alt screen 关闭后调用，避免 Layer 1 LLM 请求阻塞 Bubble Tea 主循环。
func finalizeSession(cfg Config) {
	if cfg.Agent != nil {
		cfg.Agent.OnSessionEnd()
	}
}

func runProgram(cfg Config, opts ...tea.ProgramOption) error {
	m := NewModel(cfg)
	model := &m
	p := tea.NewProgram(model, opts...)
	model.SetProgram(p)
	setupJobControl(p)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	finalizeSession(cfg)
	if cfg.Store != nil {
		_ = cfg.Store.Close()
	}
	return nil
}

// RunWithOptions 允许测试注入 tea.ProgramOption（如 WithoutRenderer）。
func RunWithOptions(cfg Config, opts ...tea.ProgramOption) error {
	allOpts := append([]tea.ProgramOption{tea.WithAltScreen(), tea.WithReportFocus()}, opts...)
	return runProgram(cfg, allOpts...)
}

// MustRun 启动 TUI，失败时写入 stderr 并以非零退出码终止进程。
func MustRun(cfg Config) {
	if err := Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "golem: %v\n", err)
		os.Exit(1)
	}
}
