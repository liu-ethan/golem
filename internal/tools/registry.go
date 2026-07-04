package tools

import (
	"context"
	"fmt"

	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/sandbox"
)

// Tool 描述一个内置工具的名称、schema 与执行函数。
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Execute     func(ctx context.Context, input map[string]any) (string, error)
}

// Registry 持有 project_root 下的全部内置工具，供 Agent 查询 schema 与分发执行。
type Registry struct {
	projectRoot string
	sandboxMode sandbox.SandboxMode
	tools       map[string]Tool
	toolFilter  func(string) bool
}

// NewRegistry 创建绑定 projectRoot 的内置工具注册表；mode 为空时默认 workspace-write。
func NewRegistry(projectRoot string, mode sandbox.SandboxMode) *Registry {
	if mode == "" {
		mode = sandbox.ModeWorkspaceWrite
	}
	r := &Registry{
		projectRoot: projectRoot,
		sandboxMode: mode,
		tools:       make(map[string]Tool),
	}
	r.register(bashTool(projectRoot, mode))
	r.register(readFileTool(projectRoot))
	r.register(writeFileTool(projectRoot))
	r.register(editFileTool(projectRoot))
	r.register(listDirTool(projectRoot))
	r.register(grepTool(projectRoot))
	r.register(webSearchTool())
	return r
}

func (r *Registry) register(tool Tool) {
	r.tools[tool.Name] = tool
}

// Definitions 返回全部工具的 Anthropic ToolDefinition，供 StreamChat 注册。
func (r *Registry) Definitions() []llm.ToolDefinition {
	names := []string{"bash", "read_file", "write_file", "edit_file", "list_dir", "grep", "web_search"}
	defs := make([]llm.ToolDefinition, 0, len(names))
	for _, name := range names {
		tool, ok := r.tools[name]
		if !ok {
			continue
		}
		if r.toolFilter != nil && !r.toolFilter(name) {
			continue
		}
		defs = append(defs, llm.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return defs
}

// Execute 按名称调用工具；未知工具或参数错误时返回 error。
func (r *Registry) Execute(ctx context.Context, name string, input map[string]any) (string, error) {
	if r.toolFilter != nil && !r.toolFilter(name) {
		return "", fmt.Errorf("tool denied by active skill: %s", name)
	}
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Execute(ctx, input)
}

// SetToolFilter 设置 Skill 工具过滤函数。
func (r *Registry) SetToolFilter(fn func(string) bool) {
	r.toolFilter = fn
}

// ClearToolFilter 清除 Skill 工具过滤。
func (r *Registry) ClearToolFilter() {
	r.toolFilter = nil
}

// ProjectRoot 返回注册表绑定的项目根目录。
func (r *Registry) ProjectRoot() string {
	return r.projectRoot
}

// SandboxMode 返回当前 bash 沙箱模式。
func (r *Registry) SandboxMode() sandbox.SandboxMode {
	return r.sandboxMode
}

// SetSandboxMode 运行时切换 bash 沙箱模式（供 /sandbox 与 TUI 状态栏同步）。
func (r *Registry) SetSandboxMode(mode sandbox.SandboxMode) {
	if mode == "" {
		mode = sandbox.ModeWorkspaceWrite
	}
	r.sandboxMode = mode
	r.register(bashTool(r.projectRoot, mode))
}
