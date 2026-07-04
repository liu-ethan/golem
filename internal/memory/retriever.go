package memory

import "context"

// MemoryRetriever 记忆检索策略；P1 默认 BM25Retriever，P3 可扩展 Vector/Hybrid。
type MemoryRetriever interface {
	Search(ctx context.Context, query string, facts []MemoryFact, topK int) ([]MemoryFact, error)
}

// Embedder 向量生成接口，P3 可选；P1 仅定义，不实现、不调用。
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}
