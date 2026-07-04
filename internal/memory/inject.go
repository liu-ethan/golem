package memory

import (
	"context"
	"fmt"

	"github.com/tencent-docs/golem/internal/llm/prompts"
)

// MemoryFactReader 供会话启动时 BM25 检索读取当前项目的全部情节记忆。
type MemoryFactReader interface {
	ListMemoryFacts() ([]MemoryFact, error)
}

// InjectMemoryOnce 以 query 对 store 中的情节记忆做 BM25 检索，格式化为可追加到 system prompt 的文本块。
// store 或 retriever 为 nil、无 facts 或无命中时返回空字符串；topK <= 0 时使用 5。
func InjectMemoryOnce(ctx context.Context, query string, store MemoryFactReader, retriever MemoryRetriever, topK int) (string, error) {
	if store == nil || retriever == nil {
		return "", nil
	}
	if topK <= 0 {
		topK = 5
	}

	facts, err := store.ListMemoryFacts()
	if err != nil {
		return "", fmt.Errorf("list memory facts: %w", err)
	}
	if len(facts) == 0 {
		return "", nil
	}

	matched, err := retriever.Search(ctx, query, facts, topK)
	if err != nil {
		return "", fmt.Errorf("search memory facts: %w", err)
	}
	if len(matched) == 0 {
		return "", nil
	}

	contents := make([]string, len(matched))
	for i, f := range matched {
		contents[i] = f.Content
	}
	return prompts.InjectMemoryBlock(contents), nil
}
