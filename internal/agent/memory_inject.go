package agent

import (
	"context"

	"github.com/tencent-docs/golem/internal/memory"
)

// BM25MemoryProvider 在首条用户消息前从 SQLite 检索 top-K 情节记忆并格式化为 system prompt 块。
type BM25MemoryProvider struct {
	Store     memory.MemoryFactReader
	Retriever memory.MemoryRetriever
	TopK      int
}

// InjectOnce 调用 memory.InjectMemoryOnce 执行一次性 BM25 检索。
func (p BM25MemoryProvider) InjectOnce(ctx context.Context, query string) (string, error) {
	return memory.InjectMemoryOnce(ctx, query, p.Store, p.Retriever, p.TopK)
}
