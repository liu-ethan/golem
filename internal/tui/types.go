package tui

import (
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/memory"
	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/skills"
)

// PageKind 标识 TUI 当前激活的子页面。
type PageKind int

const (
	PageWelcome PageKind = iota
	PageSetup
	PageChat
	PagePermissions
	PageSessions
	PageMemories
	PageSkills
)

const (
	PermTabModes  = 0
	PermTabDenied = 1
	PermTabRules  = 2
)

// LineKind 标识聊天区单行内容的类型。
type LineKind int

const (
	LineUser LineKind = iota
	LineAssistant
	LineThinking
	LineTool
	LineSystem
)

// ToolState 描述工具卡片在 UI 中的执行状态。
type ToolState int

const (
	ToolPending ToolState = iota
	ToolRunning
	ToolDone
	ToolDenied
	ToolConfirm
)

// ChatLine 表示聊天区已渲染的一行或一张工具卡片。
type ChatLine struct {
	Kind       LineKind
	Text       string
	ToolName   string
	ToolInput  map[string]any
	ToolOutput string
	ToolError  bool
	ToolState  ToolState
}

// StatusBar 汇总状态栏展示字段。
type StatusBar struct {
	ProjectRoot  string
	Approval     string
	Sandbox      string
	SessionID    string
	Model        string
	InputTokens  int
	ContextLimit int
}

// ConfirmState 表示工具确认框等待用户 Y/n/Esc 响应。
type ConfirmState struct {
	ToolName string
	Input    map[string]any
	RespCh   chan bool
}

// PermissionsPage 保存 /permissions 子页状态。
type PermissionsPage struct {
	Tab    int
	Cursor int
	Denied []session.DenialEntry
}

// SessionsPage 保存 /sessions 子页状态。
type SessionsPage struct {
	Entries []session.Entry
	Cursor  int
}

// MemoriesPage 保存 /memories 子页状态。
type MemoriesPage struct {
	Facts         []memory.MemoryFact
	InjectEnabled bool
	Cursor        int
}

// SkillsPage 保存 /skills 子页状态。
type SkillsPage struct {
	Skills []skills.Skill
	Cursor int
}

// slashResult 为斜杠命令本地处理结果。
type slashResult struct {
	handled             bool
	quit                bool
	message             string
	openPage            PageKind
	setMode             string
	setSandbox          string
	setModel            string
	compact             bool
	compactInstructions string
	reviewTarget        string
	runReview           bool
	runInit             bool
	initWrite           bool
	runPlan             string
	runAgent            string
	clearScreen         bool
	clearContext        bool
	exportPath          string
	doExport            bool
	renameName          string
	fork                bool
	showUsage           bool
	runSkill            string
	skillQuery          string
}

// approvalModeIndex 返回 mode 在 approval.Modes 中的索引，未知模式返回 0。
func approvalModeIndex(mode string) int {
	for i, m := range approval.Modes {
		if m == mode {
			return i
		}
	}
	return 1
}
