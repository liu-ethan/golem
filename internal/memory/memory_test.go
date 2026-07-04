package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadSampleFacts(t *testing.T) []MemoryFact {
	t.Helper()
	path := filepath.Join("testdata", "sample_facts.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var facts []MemoryFact
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var f MemoryFact
		if err := json.Unmarshal([]byte(line), &f); err != nil {
			t.Fatalf("parse fact: %v", err)
		}
		facts = append(facts, f)
	}
	return facts
}

func TestBM25SearchGoErrorHandling(t *testing.T) {
	facts := loadSampleFacts(t)
	if len(facts) < 5 {
		t.Fatalf("sample facts = %d, want at least 5", len(facts))
	}

	r := NewBM25Retriever()
	got, err := r.Search(context.Background(), "Go 错误处理", facts, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("results = %d, want 5", len(got))
	}

	// 最相关条目应排在前列：同时命中 Go 与错误处理语义。
	top := got[0].Content
	if !strings.Contains(top, "Go") || !strings.Contains(top, "error") {
		t.Errorf("top result = %q, want Go error handling related fact", top)
	}

	// 无关事实不应进入 top-5。
	for _, f := range got {
		if strings.Contains(f.Content, "tabs 缩进") {
			t.Errorf("irrelevant fact in top-5: %q", f.Content)
		}
	}
}

func TestBM25SearchEmptyInput(t *testing.T) {
	r := NewBM25Retriever()

	got, err := r.Search(context.Background(), "query", nil, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("facts=nil: got %v, want nil", got)
	}

	got, err = r.Search(context.Background(), "query", []MemoryFact{{Content: "x"}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("topK=0: got %v, want nil", got)
	}
}

func TestBM25SearchFewerFactsThanTopK(t *testing.T) {
	facts := []MemoryFact{
		{Content: "Go error return 显式处理"},
		{Content: "Python exception 风格"},
	}
	r := NewBM25Retriever()
	got, err := r.Search(context.Background(), "Go 错误", facts, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("results = %d, want 2", len(got))
	}
	if !strings.Contains(got[0].Content, "Go") {
		t.Errorf("first = %q, want Go-related fact first", got[0].Content)
	}
}

func TestTokenizeMixedChineseEnglish(t *testing.T) {
	tokens := tokenize("Go 错误处理 error-return")
	want := []string{"go", "错", "误", "处", "理", "error", "return"}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %v, want %v", tokens, want)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Errorf("tokens[%d] = %q, want %q", i, tokens[i], want[i])
		}
	}
}
