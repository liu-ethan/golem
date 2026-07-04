package tui

import tea "github.com/charmbracelet/bubbletea"

// isShiftTab 判断按键是否为 Shift+Tab（循环 approval 模式）。
func isShiftTab(msg tea.KeyMsg) bool {
	return msg.String() == "shift+tab"
}

// confirmKeyAllowed 判断确认框下是否为有效响应键。
func confirmKeyAllowed(key string) (allow bool, deny bool) {
	switch key {
	case "y", "Y", "enter":
		return true, false
	case "n", "N", "esc", "ctrl+c":
		return false, true
	default:
		return false, false
	}
}
