//go:build windows

package tui

import tea "github.com/charmbracelet/bubbletea"

// setupJobControl 在 Windows 上为 no-op（无 SIGTSTP）。
func setupJobControl(_ *tea.Program) {}
