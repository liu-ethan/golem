package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/llm/prompts"
)

// SummaryMessagePrefix 标识 Layer 0 压缩后注入对话的摘要消息前缀，与 resume 还原格式一致。
const SummaryMessagePrefix = "[Previous conversation summary]"

// SummaryStore 持久化 Layer 0 压缩摘要到 sessions.summary。
type SummaryStore interface {
	UpdateSummary(sessionID, summary string) error
}

// CompactResult 描述一次压缩尝试的结果。
type CompactResult struct {
	Messages  []llm.Message
	Summary   string
	Compacted bool
	Usage     llm.Usage
}

// MaybeCompact 在累计 input tokens 达阈值时压缩最旧一批非 system 消息；force 为 true 时跳过阈值（/compact）。
func MaybeCompact(
	ctx context.Context,
	sessionID string,
	messages []llm.Message,
	sessionInputTokens, contextLimit int,
	cfg config.MemoryConfig,
	client llm.LLMClient,
	store SummaryStore,
	force bool,
	extraInstructions string,
) (CompactResult, error) {
	out := CompactResult{Messages: messages}
	if client == nil {
		return out, nil
	}
	if contextLimit <= 0 {
		contextLimit = 200000
	}
	batchSize := cfg.CompactBatchSize
	if batchSize <= 0 {
		batchSize = 10
	}
	threshold := cfg.CompactThreshold
	if threshold <= 0 {
		threshold = 0.8
	}

	if !force && float64(sessionInputTokens) < float64(contextLimit)*threshold {
		return out, nil
	}

	nonSystem := filterNonSystem(messages)
	if len(nonSystem) <= batchSize {
		return out, nil
	}

	old := append([]llm.Message(nil), nonSystem[:batchSize]...)
	summary, usage, err := summarizeBatch(ctx, client, old, extraInstructions)
	if err != nil {
		return out, err
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return out, fmt.Errorf("layer0: empty summary from LLM")
	}

	newMessages := replaceOldestBatch(messages, batchSize, summary)
	if store != nil && sessionID != "" {
		if err := store.UpdateSummary(sessionID, summary); err != nil {
			return out, fmt.Errorf("update session summary: %w", err)
		}
	}

	out.Messages = newMessages
	out.Summary = summary
	out.Compacted = true
	out.Usage = usage
	return out, nil
}

// filterNonSystem 返回非 system 角色消息；当前 Message 仅含 user/assistant，原样返回副本。
func filterNonSystem(messages []llm.Message) []llm.Message {
	out := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		out = append(out, msg)
	}
	return out
}

// replaceOldestBatch 将最旧 batchSize 条消息替换为一条 summary 用户消息。
func replaceOldestBatch(messages []llm.Message, batchSize int, summary string) []llm.Message {
	if batchSize <= 0 || batchSize >= len(messages) {
		return messages
	}
	remaining := append([]llm.Message(nil), messages[batchSize:]...)
	return append([]llm.Message{SummaryMessage(summary)}, remaining...)
}

// SummaryMessage 构造 Layer 0 摘要用户消息。
func SummaryMessage(summary string) llm.Message {
	return llm.Message{
		Role: llm.RoleUser,
		Content: []llm.ContentBlock{{
			Type: "text",
			Text: SummaryMessagePrefix + "\n" + summary,
		}},
	}
}

// IsSummaryMessage 判断消息是否为 Layer 0 注入的摘要行。
func IsSummaryMessage(msg llm.Message) bool {
	if msg.Role != llm.RoleUser || len(msg.Content) == 0 {
		return false
	}
	block := msg.Content[0]
	return block.Type == "text" && strings.HasPrefix(block.Text, SummaryMessagePrefix)
}

// summarizeBatch 调用 Complete 将一批消息压缩为摘要文本。
func summarizeBatch(ctx context.Context, client llm.LLMClient, batch []llm.Message, extraInstructions string) (string, llm.Usage, error) {
	text, usage, err := client.Complete(ctx, llm.CompleteRequest{
		System:    prompts.Layer0SystemPrompt(extraInstructions),
		Messages:  batch,
		MaxTokens: 2048,
	})
	if err != nil {
		return "", usage, err
	}
	return text, usage, nil
}
