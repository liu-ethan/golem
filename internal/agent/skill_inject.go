package agent

import (
	"context"

	"github.com/tencent-docs/golem/internal/memory"
	"github.com/tencent-docs/golem/internal/skills"
)

// BM25SkillProvider 在首条用户消息前按 query 语义注入 Skill 渐进式披露块。
type BM25SkillProvider struct {
	Loader    *skills.Loader
	Retriever memory.MemoryRetriever
	TopK      int
}

// InjectOnce 调用 skills.InjectSkillsOnce 执行一次性 Skill 目录与语义匹配注入。
func (p BM25SkillProvider) InjectOnce(ctx context.Context, query string) (string, error) {
	return skills.InjectSkillsOnce(ctx, query, p.Loader, p.Retriever, p.TopK)
}
