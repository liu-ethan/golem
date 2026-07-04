package skills

import (
	"context"
	"fmt"
	"strings"

	"github.com/tencent-docs/golem/internal/memory"
)

// InjectSkillsOnce 对可用 Skill 做渐进式披露：始终写入 Skill 目录摘要，并按 query 语义 BM25 命中 top-K 后注入完整指令。
// loader 或 retriever 为 nil、无 Skill 或无命中时仍可能返回仅含目录的块；topK <= 0 时使用 2。
func InjectSkillsOnce(ctx context.Context, query string, loader *Loader, retriever memory.MemoryRetriever, topK int) (string, error) {
	if loader == nil {
		return "", nil
	}
	if topK <= 0 {
		topK = 2
	}

	list, err := loader.List()
	if err != nil {
		return "", fmt.Errorf("list skills: %w", err)
	}
	if len(list) == 0 {
		return "", nil
	}

	block := injectSkillCatalogBlock(list)
	if retriever == nil {
		return block, nil
	}

	facts := make([]memory.MemoryFact, len(list))
	for i, s := range list {
		facts[i] = memory.MemoryFact{Content: s.SearchText()}
	}
	matchedFacts, err := retriever.Search(ctx, query, facts, topK)
	if err != nil {
		return "", fmt.Errorf("search skills: %w", err)
	}
	if len(matchedFacts) == 0 {
		return block, nil
	}

	byContent := make(map[string]Skill, len(list))
	for _, s := range list {
		byContent[s.SearchText()] = s
	}
	matched := make([]Skill, 0, len(matchedFacts))
	seen := map[string]bool{}
	for _, f := range matchedFacts {
		s, ok := byContent[f.Content]
		if !ok || seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		matched = append(matched, s)
	}
	return block + injectMatchedSkillsBlock(matched), nil
}

func injectSkillCatalogBlock(list []Skill) string {
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## 可用 Skills\n")
	b.WriteString("输入 /<skill-name> <提问> 可显式选用某一 Skill；未显式选用时，下方「自动匹配」块会按语义注入相关 Skill 指令。\n")
	for _, s := range list {
		fmt.Fprintf(&b, "- **%s**: %s\n", s.Name, s.Summary())
	}
	return b.String()
}

func injectMatchedSkillsBlock(matched []Skill) string {
	if len(matched) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## 自动匹配的相关 Skill\n")
	b.WriteString("以下 Skill 与当前用户问题语义相关，请按其中指令行事。\n")
	for _, s := range matched {
		b.WriteString(s.PromptOverlay())
	}
	return b.String()
}
