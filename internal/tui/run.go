package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Run 启动 Bubble Tea TUI，阻塞直到用户退出。
func Run(cfg Config) error {
	m := NewModel(cfg)
	model := &m
	p := tea.NewProgram(model, tea.WithAltScreen())
	model.SetProgram(p)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	if cfg.Store != nil {
		_ = cfg.Store.Close()
	}
	return nil
}

// RunWithOptions 允许测试注入 tea.ProgramOption（如 WithoutRenderer）。
func RunWithOptions(cfg Config, opts ...tea.ProgramOption) error {
	m := NewModel(cfg)
	model := &m
	allOpts := append([]tea.ProgramOption{tea.WithAltScreen()}, opts...)
	p := tea.NewProgram(model, allOpts...)
	model.SetProgram(p)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	if cfg.Store != nil {
		_ = cfg.Store.Close()
	}
	return nil
}

// MustRun 启动 TUI，失败时写入 stderr 并以非零退出码终止进程。
func MustRun(cfg Config) {
	if err := Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "golem: %v\n", err)
		os.Exit(1)
	}
}
