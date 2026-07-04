package tools

import (
	"context"
	"fmt"

	"github.com/tencent-docs/golem/internal/llm"
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
	tools       map[string]Tool
}

// NewRegistry 创建绑定 projectRoot 的内置工具注册表。
func NewRegistry(projectRoot string) *Registry {
	r := &Registry{
		projectRoot: projectRoot,
		tools:       make(map[string]Tool),
	}
	r.register(bashTool(projectRoot))
	r.register(readFileTool(projectRoot))
	r.register(writeFileTool(projectRoot))
	r.register(editFileTool(projectRoot))
	r.register(listDirTool(projectRoot))
	r.register(grepTool(projectRoot))
	return r
}

func (r *Registry) register(tool Tool) {
	r.tools[tool.Name] = tool
}

// Definitions 返回全部工具的 Anthropic ToolDefinition，供 StreamChat 注册。
func (r *Registry) Definitions() []llm.ToolDefinition {
	names := []string{"bash", "read_file", "write_file", "edit_file", "list_dir", "grep"}
	defs := make([]llm.ToolDefinition, 0, len(names))
	for _, name := range names {
		tool, ok := r.tools[name]
		if !ok {
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
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Execute(ctx, input)
}

// ProjectRoot 返回注册表绑定的项目根目录。
func (r *Registry) ProjectRoot() string {
	return r.projectRoot
}
