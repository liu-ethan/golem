package agent

import (
	"context"

	"github.com/tencent-docs/golem/internal/memory"
)

// runCompact 尝试 Layer 0 滑动窗口压缩；force 为 true 时（/compact）强制压缩。
func (a *Agent) runCompact(ctx context.Context, force bool, instructions string) (memory.CompactResult, error) {
	if a.llm == nil {
		return memory.CompactResult{Messages: a.messages}, nil
	}
	if ensurer, ok := a.summaryStore.(interface{ EnsureSession(string) error }); ok && a.sessionID != "" {
		if err := ensurer.EnsureSession(a.sessionID); err != nil {
			return memory.CompactResult{}, err
		}
	}
	result, err := memory.MaybeCompact(
		ctx,
		a.sessionID,
		a.messages,
		a.sessionInputTokens,
		a.contextLimit,
		a.memoryCfg,
		a.llm,
		a.summaryStore,
		force,
		instructions,
	)
	if err != nil {
		return memory.CompactResult{}, err
	}
	if result.Compacted {
		a.messages = result.Messages
		a.AddTokenUsage(result.Usage)
	}
	return result, nil
}

// runCompactBeforeTurn 在主循环每轮 StreamChat 前尝试自动压缩。
func (a *Agent) runCompactBeforeTurn(ctx context.Context, handler EventHandler) error {
	_, err := a.runCompact(ctx, false, "")
	if err != nil && handler != nil {
		handler(Event{Type: EventError, Err: err})
	}
	return err
}
