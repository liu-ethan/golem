package memory

import (
	"context"
	"strings"
	"testing"
)

type stubFactReader struct {
	facts []MemoryFact
	err   error
}

func (s stubFactReader) ListMemoryFacts() ([]MemoryFact, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.facts, nil
}

func TestInjectMemoryOnceReturnsEmptyWithoutFacts(t *testing.T) {
	got, err := InjectMemoryOnce(context.Background(), "Go 错误处理", stubFactReader{}, NewBM25Retriever(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("got = %q, want empty", got)
	}
}

func TestInjectMemoryOnceFormatsTopMatches(t *testing.T) {
	facts := loadSampleFacts(t)
	reader := stubFactReader{facts: facts}
	got, err := InjectMemoryOnce(context.Background(), "Go 错误处理", reader, NewBM25Retriever(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "## 相关记忆") {
		t.Errorf("got = %q, want memory block heading", got)
	}
	if !strings.Contains(got, "Go") {
		t.Errorf("got = %q, want Go-related fact", got)
	}
}

func TestInjectMemoryOnceNilDeps(t *testing.T) {
	got, err := InjectMemoryOnce(context.Background(), "query", nil, nil, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("got = %q, want empty", got)
	}
}
