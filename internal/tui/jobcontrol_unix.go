//go:build !windows

package tui

import (
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
)

// setupJobControl 在 fg（SIGCONT）后重建终端 raw/alt-screen 状态。
// Ctrl+Z 挂起后再 fg 时，若不恢复终端，输入常会卡住。
func setupJobControl(p *tea.Program) {
	if p == nil {
		return
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGCONT)
	go func() {
		for range sig {
			if err := p.RestoreTerminal(); err != nil {
				continue
			}
			p.Send(tea.ResumeMsg{})
		}
	}()
}
