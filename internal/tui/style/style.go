// Package style 定义 golem TUI 暖色调语义配色，对齐 Claude Code CLI 视觉层级。
package style

import "github.com/charmbracelet/lipgloss"

// 256 色调色板：暖色底 + 橙/金强调，16 色终端下仍可区分层级。
const (
	ColorFGDefault   = "252" // 正文
	ColorFGMuted     = "244" // 次要信息
	ColorFGEmphasis  = "254" // 标题、选中项
	ColorBGBase      = "237" // 主背景
	ColorBGSurface   = "235" // 状态栏/页脚
	ColorBGOverlay   = "236" // 选中背景、分隔
	ColorBorder      = "238" // 边框、分隔线
	ColorAccent      = "215" // 主强调（暖橙，对标 Claude）
	ColorAccentAlt   = "217" // 次强调（暖金）
	ColorUser        = "223" // 用户输入/消息
	ColorAssistant   = "252" // 助手正文
	ColorPath        = "214" // 文件路径
	ColorCode        = "221" // 行内/块代码
	ColorThinking    = "244" // 思考过程
	ColorThinkTitle  = "217" // Thinking 标题
	ColorSuccess     = "150" // 成功
	ColorError       = "203" // 错误
	ColorWarning     = "214" // 警告
	ColorInfo        = "117" // 信息
	ColorCursor      = "215" // 输入光标
)

var (
	StatusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorFGDefault)).
			Background(lipgloss.Color(ColorBGSurface)).
			Padding(0, 1)
	Footer = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorFGMuted)).
		Background(lipgloss.Color(ColorBGSurface)).
		Padding(0, 1)
	Prompt = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	// UserLabel 用 accent.primary，UserText 用暖沙色正文，与助手白字形成层级。
	UserLabel = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	UserText  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorUser))
	UserSlash = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccentAlt))
	AsstLabel = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccentAlt))
	AsstText  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAssistant))
	ThinkBody = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorThinking)).Italic(true)
	ThinkTitle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorThinkTitle)).Bold(true)
	SysText   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFGMuted)).Italic(true)
	ErrText   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	PathText  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPath))
	CodeText  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCode))
	Cursor    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCursor)).Bold(true)
	Border    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder))
	Accent    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent))
	AccentAlt = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccentAlt))
	Muted     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFGMuted))
	Emphasis  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFGEmphasis)).Bold(true)
	SlashSel  = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorFGEmphasis)).
			Background(lipgloss.Color(ColorBGOverlay)).
			Bold(true)
	SlashItem = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFGDefault))
	SlashDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFGMuted))
	Title     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorFGEmphasis))
	Active    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	Dim       = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFGMuted))
	Rule      = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFGDefault))
	Success   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	Warning   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning))
	WelcomeBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorAccent))
)
