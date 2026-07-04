package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/llm/prompts"
)

const (
	categoryPreference  = "preference"
	categoryProjectFact = "project_fact"
	categoryTaskProgress = "task_progress"
)

// FactStore 供 Layer 1 写入情节记忆并读取 Layer 2 触发计数。
type FactStore interface {
	ProjectIDValue() string
	InsertMemoryFacts(sessionID string, facts []MemoryFact) error
	IncrementSessionCount() (int, error)
}

// SessionEndParams 描述会话结束时 Layer 1 提取所需的上下文。
type SessionEndParams struct {
	SessionID   string
	ProjectID   string
	ProjectRoot string
	Messages    []llm.Message
	Config      config.MemoryConfig
	LLM         llm.LLMClient
	Store       FactStore
}

// extractedFact 对应 Layer 1 LLM 输出的单条 JSON 对象。
type extractedFact struct {
	Content  string `json:"content"`
	Category string `json:"category"`
}

// OnSessionEnd 在会话正常结束时同步提取情节记忆并写入 SQLite；达阈值时同步触发 Layer 2。
func OnSessionEnd(ctx context.Context, p SessionEndParams) error {
	if p.LLM == nil || p.Store == nil || p.SessionID == "" {
		return nil
	}
	if len(p.Messages) == 0 {
		return nil
	}

	facts, err := extractFacts(ctx, p.LLM, p.Messages)
	if err != nil {
		return fmt.Errorf("layer1 extract: %w", err)
	}
	if len(facts) > 0 {
		projectID := p.ProjectID
		if projectID == "" {
			projectID = p.Store.ProjectIDValue()
		}
		stamped := stampFacts(facts, p.SessionID, projectID)
		if err := p.Store.InsertMemoryFacts(p.SessionID, stamped); err != nil {
			return fmt.Errorf("insert memory facts: %w", err)
		}
	}

	count, err := p.Store.IncrementSessionCount()
	if err != nil {
		return fmt.Errorf("increment session count: %w", err)
	}

	threshold := p.Config.Layer2SessionThreshold
	if threshold <= 0 {
		threshold = 3
	}
	if count >= threshold {
		projectID := p.ProjectID
		if projectID == "" {
			projectID = p.Store.ProjectIDValue()
		}
		if profileStore, ok := p.Store.(ProfileStore); ok {
			if err := RunLayer2(ctx, projectID, p.ProjectRoot, profileStore, p.LLM); err != nil {
				return fmt.Errorf("layer2: %w", err)
			}
		}
	}
	return nil
}

// extractFacts 调用 Complete 从会话消息中提取情节记忆。
func extractFacts(ctx context.Context, client llm.LLMClient, messages []llm.Message) ([]MemoryFact, error) {
	conversation := formatConversation(messages)
	if strings.TrimSpace(conversation) == "" {
		return nil, nil
	}

	text, _, err := client.Complete(ctx, llm.CompleteRequest{
		System: prompts.Layer1SystemPrompt(),
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{{
				Type: "text",
				Text: conversation,
			}},
		}},
		MaxTokens: 2048,
	})
	if err != nil {
		return nil, err
	}
	return parseExtractedFacts(text)
}

// parseExtractedFacts 解析 LLM 返回的 JSON 数组，过滤无效条目。
func parseExtractedFacts(raw string) ([]MemoryFact, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	raw = stripJSONCodeFence(raw)

	var items []extractedFact
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("parse facts JSON: %w", err)
	}

	var facts []MemoryFact
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		category := normalizeCategory(item.Category)
		if content == "" || category == "" {
			continue
		}
		facts = append(facts, MemoryFact{
			Content:  content,
			Category: category,
		})
	}
	return facts, nil
}

// normalizeCategory 校验并规范化 category 字段。
func normalizeCategory(category string) string {
	switch strings.TrimSpace(category) {
	case categoryPreference, categoryProjectFact, categoryTaskProgress:
		return strings.TrimSpace(category)
	default:
		return ""
	}
}

// stampFacts 为提取结果填充 id、project_id 与时间戳。
func stampFacts(facts []MemoryFact, sessionID, projectID string) []MemoryFact {
	now := time.Now().UTC()
	out := make([]MemoryFact, len(facts))
	for i, f := range facts {
		out[i] = MemoryFact{
			ID:        uuid.NewString(),
			SessionID: sessionID,
			ProjectID: projectID,
			Content:   f.Content,
			Category:  f.Category,
			CreatedAt: now.Add(time.Duration(i) * time.Microsecond),
		}
	}
	return out
}

// formatConversation 将会话消息格式化为供 LLM 阅读的纯文本。
func formatConversation(messages []llm.Message) string {
	var b strings.Builder
	for _, msg := range messages {
		if IsSummaryMessage(msg) {
			fmt.Fprintf(&b, "user: %s\n", msg.Content[0].Text)
			continue
		}
		role := string(msg.Role)
		text := flattenMessageContent(msg)
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "%s: %s\n", role, text)
	}
	return strings.TrimSpace(b.String())
}

// flattenMessageContent 将单条消息的内容块合并为可读文本。
func flattenMessageContent(msg llm.Message) string {
	var parts []string
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			if t := strings.TrimSpace(block.Text); t != "" {
				parts = append(parts, t)
			}
		case "tool_use":
			parts = append(parts, fmt.Sprintf("[tool_use %s: %s]", block.Name, formatToolInput(block.Input)))
		case "tool_result":
			content := strings.TrimSpace(block.Content)
			if block.IsError {
				parts = append(parts, "[tool_result error: "+content+"]")
			} else {
				parts = append(parts, "[tool_result: "+truncateToolResult(content)+"]")
			}
		}
	}
	return strings.Join(parts, " ")
}

// formatToolInput 将 tool input map 格式化为简短字符串。
func formatToolInput(input map[string]any) string {
	if len(input) == 0 {
		return "{}"
	}
	data, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	s := string(data)
	const maxLen = 200
	if len(s) > maxLen {
		return s[:maxLen-1] + "…"
	}
	return s
}

// truncateToolResult 截断过长的 tool_result 文本。
func truncateToolResult(s string) string {
	const maxLen = 500
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// stripJSONCodeFence 去除 LLM 可能包裹的 Markdown 代码块。
func stripJSONCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	start := 1
	end := len(lines)
	if strings.TrimSpace(lines[len(lines)-1]) == "```" {
		end = len(lines) - 1
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}
