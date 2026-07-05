package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/llm/prompts"
	"github.com/tencent-docs/golem/internal/skills"
)

// DenialRecorder 在工具被拒绝时回调，供 /permissions Recently denied 持久化。
type DenialRecorder func(toolName string, input map[string]any, reason string)

// SetDenialRecorder 注册拒绝记录回调。
func (a *Agent) SetDenialRecorder(fn DenialRecorder) {
	a.onDenial = fn
}

// RunPlanOnce 临时切换为 plan 模式执行单条 query，完成后恢复原 approval 模式。
func (a *Agent) RunPlanOnce(ctx context.Context, query string, handler EventHandler) error {
	prevPolicy := a.policy
	planPolicy, err := approval.New(approval.ModePlan)
	if err != nil {
		return err
	}
	a.SetApprovalPolicy(planPolicy)
	defer a.SetApprovalPolicy(prevPolicy)
	return a.handleUserMessage(ctx, query, handler)
}

// RunSkillOnce 以指定 Skill 执行单条 query，仅本轮生效，不持久切换 Skill。
func (a *Agent) RunSkillOnce(ctx context.Context, skill skills.Skill, query string, handler EventHandler) error {
	overlay := skill.PromptOverlay()
	overlayStart := len(a.systemPrompt)
	if overlay != "" {
		a.systemPrompt += overlay
	}
	a.tools.SetToolFilter(skill.ToolAllowed)
	defer func() {
		if overlay != "" && len(a.systemPrompt) >= overlayStart+len(overlay) {
			a.systemPrompt = a.systemPrompt[:overlayStart] + a.systemPrompt[overlayStart+len(overlay):]
		}
		a.tools.ClearToolFilter()
	}()
	return a.handleUserMessage(ctx, query, handler)
}

// ConfigureProvider 运行时更新 LLM 接入端点、密钥与模型名。
func (a *Agent) ConfigureProvider(baseURL, apiKey, model string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("api_key is required")
	}
	if setter, ok := a.llm.(interface {
		Configure(baseURL, apiKey, model string)
	}); ok {
		setter.Configure(baseURL, apiKey, model)
		return nil
	}
	return fmt.Errorf("llm client does not support provider reconfiguration")
}

// SetModel 切换 LLM 模型名（AnthropicClient 实现）。
func (a *Agent) SetModel(model string) error {
	if model == "" {
		return fmt.Errorf("model is required")
	}
	if setter, ok := a.llm.(interface{ SetModel(string) }); ok {
		setter.SetModel(model)
		return nil
	}
	return fmt.Errorf("llm client does not support runtime model switch")
}

// ModelName 返回当前 LLM 模型名。
func (a *Agent) ModelName() string {
	if getter, ok := a.llm.(interface{ Model() string }); ok {
		return getter.Model()
	}
	return ""
}

// ClearContext 立即清空当前对话上下文并开新 session，返回新 ID 与旧会话快照。
// 旧会话收尾（持久化、Layer 1）须由调用方通过 OnSessionEndSnapshot 异步执行，避免阻塞 UI。
func (a *Agent) ClearContext() (newSessionID string, snapshot SessionEndSnapshot) {
	snapshot = SessionEndSnapshot{
		SessionID:       a.sessionID,
		HadUserMessages: a.hadUserMessages,
		Messages:        a.Messages(),
	}
	a.sessionID = uuid.NewString()
	a.messages = nil
	a.memoryInjected = false
	a.skillsInjected = false
	a.sessionInputTokens = 0
	a.sessionOutputTokens = 0
	a.hadUserMessages = false
	base, err := prompts.BuildBaseSystemPrompt(a.projectRoot)
	if err != nil {
		base = prompts.BaseSystemPrompt()
	}
	a.systemPrompt = base
	return a.sessionID, snapshot
}

// RunReview 对 git diff / working tree 执行 code review，返回 review 文本。
func (a *Agent) RunReview(ctx context.Context, target string) (string, error) {
	diff, err := a.collectReviewTarget(ctx, target)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(diff) == "" {
		return "（无变更可 review）", nil
	}
	text, usage, err := a.llm.Complete(ctx, llm.CompleteRequest{
		System: prompts.ReviewSystemPrompt(),
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{{
				Type: "text",
				Text: "请 review 以下变更：\n\n" + diff,
			}},
		}},
		MaxTokens: 4096,
	})
	if err != nil {
		return "", err
	}
	a.AddTokenUsage(usage)
	return strings.TrimSpace(text), nil
}

func (a *Agent) collectReviewTarget(ctx context.Context, target string) (string, error) {
	target = strings.TrimSpace(strings.ToLower(target))
	var cmd string
	switch target {
	case "", "working-tree", "working_tree", "wt", "diff":
		cmd = "git diff HEAD 2>/dev/null; git diff --cached 2>/dev/null; git status --porcelain 2>/dev/null"
	case "staged":
		cmd = "git diff --cached 2>/dev/null"
	default:
		if strings.HasPrefix(target, "commit:") {
			cmd = "git show --patch " + shellQuote(strings.TrimPrefix(target, "commit:"))
		} else if len(target) >= 7 {
			cmd = "git show --patch " + shellQuote(target)
		} else {
			cmd = "git diff HEAD 2>/dev/null; git status --porcelain 2>/dev/null"
		}
	}
	return a.runGit(ctx, cmd)
}

func (a *Agent) runGit(ctx context.Context, cmd string) (string, error) {
	return a.tools.Execute(ctx, "bash", map[string]any{"command": cmd})
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// RunInit 为当前项目生成 AGENTS.md 内容；write 为 true 时写入 project_root/AGENTS.md。
func (a *Agent) RunInit(ctx context.Context, write bool) (string, error) {
	tree, err := a.runGit(ctx, "find . -maxdepth 3 -type f 2>/dev/null | head -n 80")
	if err != nil {
		tree = ""
	}
	userText := "project_root: " + a.projectRoot + "\n\n目录采样：\n" + tree

	text, usage, err := a.llm.Complete(ctx, llm.CompleteRequest{
		System: prompts.InitSystemPrompt(),
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{{
				Type: "text",
				Text: userText,
			}},
		}},
		MaxTokens: 4096,
	})
	if err != nil {
		text = prompts.InitTemplate()
	} else {
		a.AddTokenUsage(usage)
		text = strings.TrimSpace(text)
	}
	if write {
		path := filepath.Join(a.projectRoot, "AGENTS.md")
		if err := os.WriteFile(path, []byte(text+"\n"), 0o644); err != nil {
			return "", fmt.Errorf("write AGENTS.md: %w", err)
		}
	}
	return text, nil
}

// ExportSessionMarkdown 导出当前会话为 markdown。
func (a *Agent) ExportSessionMarkdown() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# golem session %s\n\n", a.sessionID))
	for _, msg := range a.messages {
		switch msg.Role {
		case llm.RoleUser:
			for _, block := range msg.Content {
				if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
					b.WriteString("## User\n\n")
					b.WriteString(block.Text)
					b.WriteString("\n\n")
				}
			}
		case llm.RoleAssistant:
			for _, block := range msg.Content {
				switch block.Type {
				case "text":
					if strings.TrimSpace(block.Text) != "" {
						b.WriteString("## Assistant\n\n")
						b.WriteString(block.Text)
						b.WriteString("\n\n")
					}
				case "tool_use":
					b.WriteString("### Tool: ")
					b.WriteString(block.Name)
					b.WriteString("\n\n")
				}
			}
		}
	}
	return b.String()
}

// FormatUsageSummary 格式化本会话 token 统计。
func (a *Agent) FormatUsageSummary() string {
	return fmt.Sprintf("input: %d tokens\noutput: %d tokens\ntotal: %d tokens",
		a.sessionInputTokens, a.sessionOutputTokens, a.sessionInputTokens+a.sessionOutputTokens)
}

// FormatContextBreakdown 返回 /context 可视化摘要。
func (a *Agent) FormatContextBreakdown() string {
	limit := a.contextLimit
	if limit <= 0 {
		limit = 200000
	}
	return fmt.Sprintf(`Context 占用概览：
  context_limit: %d
  session_input_tokens: %d (%.1f%%)
  session_output_tokens: %d
  system_prompt_chars: %d
  message_count: %d
  memory_injected: %v`,
		limit,
		a.sessionInputTokens,
		float64(a.sessionInputTokens)/float64(limit)*100,
		a.sessionOutputTokens,
		len(a.systemPrompt),
		len(a.messages),
		a.memoryInjected,
	)
}

// FormatStatusSummary 返回 /status 概览文本。
func (a *Agent) FormatStatusSummary(sandbox, model string) string {
	if model == "" {
		model = a.ModelName()
	}
	mode := ""
	if a.policy != nil {
		mode = a.policy.Mode()
	}
	limit := a.contextLimit
	if limit <= 0 {
		limit = 200000
	}
	return fmt.Sprintf(`model: %s
approval: %s
sandbox: %s
session: %s
tokens: %d / %d`,
		model,
		mode,
		sandbox,
		a.sessionID,
		a.sessionInputTokens,
		limit,
	)
}

// RetryDeniedTool 重试 Recently denied 记录中的工具调用（跳过审批门控）。
func (a *Agent) RetryDeniedTool(ctx context.Context, toolName, inputJSON string, handler EventHandler) (string, error) {
	var input map[string]any
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return "", fmt.Errorf("parse denial input: %w", err)
	}
	out, execErr := a.tools.Execute(ctx, toolName, input)
	if execErr != nil {
		return "", execErr
	}
	if handler != nil {
		handler(Event{
			Type:       EventToolComplete,
			ToolName:   toolName,
			ToolInput:  input,
			ToolOutput: out,
		})
	}
	return out, nil
}

// OpenExternalEditor 调用 $EDITOR 撰写多行输入；EDITOR 未设置时使用 vi。
func OpenExternalEditor(initial string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	tmp, err := os.CreateTemp("", "golem-edit-*.md")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(initial); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
