package memory

import (
	"context"
	"math"
	"sort"
	"strings"
	"unicode"
)

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// BM25Retriever 基于 BM25 的关键词检索，零外部依赖，为 P1 默认实现。
type BM25Retriever struct{}

// NewBM25Retriever 返回 BM25 检索器实例。
func NewBM25Retriever() *BM25Retriever {
	return &BM25Retriever{}
}

// Search 对 facts 按 query 做 BM25 打分，返回得分最高的 topK 条；facts 为空或 topK <= 0 时返回 nil。
func (r *BM25Retriever) Search(_ context.Context, query string, facts []MemoryFact, topK int) ([]MemoryFact, error) {
	if len(facts) == 0 || topK <= 0 {
		return nil, nil
	}

	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return copyFacts(facts, topK), nil
	}

	docTokens := make([][]string, len(facts))
	docLens := make([]float64, len(facts))
	var totalLen float64
	for i, f := range facts {
		docTokens[i] = tokenize(f.Content)
		docLens[i] = float64(len(docTokens[i]))
		totalLen += docLens[i]
	}
	avgDL := totalLen / float64(len(facts))
	if avgDL == 0 {
		return copyFacts(facts, topK), nil
	}

	n := float64(len(facts))
	df := termDocumentFrequency(docTokens)

	type scored struct {
		idx   int
		score float64
	}
	scores := make([]scored, len(facts))
	for i, tokens := range docTokens {
		tf := termFrequency(tokens)
		var score float64
		for _, qt := range queryTokens {
			freq := tf[qt]
			if freq == 0 {
				continue
			}
			idf := math.Log((n-df[qt]+0.5)/(df[qt]+0.5) + 1)
			denom := freq + bm25K1*(1-bm25B+bm25B*docLens[i]/avgDL)
			score += idf * (freq * (bm25K1 + 1)) / denom
		}
		scores[i] = scored{idx: i, score: score}
	}

	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].score != scores[j].score {
			return scores[i].score > scores[j].score
		}
		return scores[i].idx < scores[j].idx
	})

	limit := topK
	if limit > len(facts) {
		limit = len(facts)
	}
	out := make([]MemoryFact, limit)
	for i := 0; i < limit; i++ {
		out[i] = facts[scores[i].idx]
	}
	return out, nil
}

// tokenize 将文本拆分为 BM25 词项：英文/数字连续串小写化，中文按单字切分。
func tokenize(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}

	var tokens []string
	var buf strings.Builder
	flushWord := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}

	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			flushWord()
			tokens = append(tokens, string(r))
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			buf.WriteRune(r)
		default:
			flushWord()
		}
	}
	flushWord()
	return tokens
}

func termFrequency(tokens []string) map[string]float64 {
	tf := make(map[string]float64, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}
	return tf
}

func termDocumentFrequency(docs [][]string) map[string]float64 {
	df := make(map[string]float64)
	for _, tokens := range docs {
		seen := make(map[string]struct{})
		for _, t := range tokens {
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			df[t]++
		}
	}
	return df
}

func copyFacts(facts []MemoryFact, topK int) []MemoryFact {
	limit := topK
	if limit > len(facts) {
		limit = len(facts)
	}
	out := make([]MemoryFact, limit)
	copy(out, facts[:limit])
	return out
}
