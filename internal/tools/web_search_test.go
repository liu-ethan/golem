package tools

import (
	"context"
	"os"
	"testing"
)

func TestWebSearchRequiresConfig(t *testing.T) {
	_ = os.Unsetenv("WEB_SEARCH_API_URL")
	tool := webSearchTool()
	_, err := tool.Execute(context.Background(), map[string]any{"query": "golem"})
	if err == nil {
		t.Fatal("expected error when WEB_SEARCH_API_URL unset")
	}
}
