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
	Messages       []llm.Message
	Summary        string
	Compacted      bool
	CompactedCount int
	Usage          llm.Usage
}

// MaybeCompact 在 force 为 false 时，累计 input tokens 达阈值或非 system 消息数大于 batch size（满足其一）即触发压缩；
// force 为 true 时（/compact）强制压缩：消息数大于 batch size 时压最新 batch size 条，否则压全部。
// 自动压缩在消息数大于 batch size 时压最旧 batch size 条；仅因 token 达阈值且消息数不足一批时压全部。
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

	nonSystem := filterNonSystem(messages)
	if len(nonSystem) == 0 {
		return out, nil
	}

	tokenThresholdMet := float64(sessionInputTokens) >= float64(contextLimit)*threshold
	messageCountMet := len(nonSystem) > batchSize

	if !force && !tokenThresholdMet && !messageCountMet {
		return out, nil
	}

	var batch []llm.Message
	var applySummary func(string) []llm.Message

	switch {
	case force && len(nonSystem) > batchSize:
		batch = append([]llm.Message(nil), nonSystem[len(nonSystem)-batchSize:]...)
		applySummary = func(summary string) []llm.Message {
			return replaceNewestBatch(messages, batchSize, summary)
		}
	case force:
		batch = append([]llm.Message(nil), nonSystem...)
		applySummary = replaceAllWithSummary
	case len(nonSystem) > batchSize:
		batch = append([]llm.Message(nil), nonSystem[:batchSize]...)
		applySummary = func(summary string) []llm.Message {
			return replaceOldestBatch(messages, batchSize, summary)
		}
	default:
		batch = append([]llm.Message(nil), nonSystem...)
		applySummary = replaceAllWithSummary
	}

	summary, usage, err := summarizeBatch(ctx, client, batch, extraInstructions)
	if err != nil {
		return out, err
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return out, fmt.Errorf("layer0: empty summary from LLM")
	}

	newMessages := applySummary(summary)

	if store != nil && sessionID != "" {
		if err := store.UpdateSummary(sessionID, summary); err != nil {
			return out, fmt.Errorf("update session summary: %w", err)
		}
	}

	out.Messages = newMessages
	out.Summary = summary
	out.Compacted = true
	out.CompactedCount = len(batch)
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

// replaceOldestBatch 将最旧 batchSize 条消息替换为一条 summary 用户消息置于开头。
func replaceOldestBatch(messages []llm.Message, batchSize int, summary string) []llm.Message {
	if batchSize <= 0 || batchSize >= len(messages) {
		if summary == "" {
			return messages
		}
		return replaceAllWithSummary(summary)
	}
	remaining := append([]llm.Message(nil), messages[batchSize:]...)
	return append([]llm.Message{SummaryMessage(summary)}, remaining...)
}

// replaceNewestBatch 将最新 batchSize 条消息替换为一条 summary 用户消息置于末尾。
func replaceNewestBatch(messages []llm.Message, batchSize int, summary string) []llm.Message {
	if batchSize <= 0 || len(messages) <= batchSize {
		if summary == "" {
			return messages
		}
		return replaceAllWithSummary(summary)
	}
	prefix := append([]llm.Message(nil), messages[:len(messages)-batchSize]...)
	return append(prefix, SummaryMessage(summary))
}

// replaceAllWithSummary 将全部消息替换为一条 summary 用户消息。
func replaceAllWithSummary(summary string) []llm.Message {
	return []llm.Message{SummaryMessage(summary)}
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
