package tui

import (
	"fmt"
	"strings"

	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/tui/pages"
)

func renderPermissionsPage(m Model, width int) string {
	tabs := []string{"Modes", "Denied", "Rules"}
	var header strings.Builder
	header.WriteString("  ")
	for i, tab := range tabs {
		label := tab
		if i == m.permissions.Tab {
			label = "[" + tab + "]"
		}
		header.WriteString(label)
		if i < len(tabs)-1 {
			header.WriteString("  ")
		}
	}
	out := header.String() + "\n\n"
	switch m.permissions.Tab {
	case PermTabDenied:
		out += pages.DeniedList(width, deniedPageEntries(m.permissions.Denied), m.permissions.Cursor)
	case PermTabRules:
		out += pages.Permissions(width, m.height, m.status.Approval, 0, m.rulesLines)
	default:
		out += pages.Permissions(width, m.height, m.status.Approval, m.permissions.Cursor, nil)
	}
	return out
}

func permissionsTabLabel(tab int) string {
	switch tab {
	case PermTabDenied:
		return "Recently denied"
	case PermTabRules:
		return "Rules"
	default:
		return fmt.Sprintf("Modes (%s)", approval.Modes[0])
	}
}
