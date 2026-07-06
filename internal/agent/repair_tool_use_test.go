package agent

import (
	"testing"

	"github.com/tencent-docs/golem/internal/llm"
)

func TestRepairToolUsePairingInsertsMissingResults(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: "text", Text: "hi"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: "text", Text: "running"},
			{Type: "tool_use", ID: "call_1", Name: "bash", Input: map[string]any{"command": "ls"}},
		}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: "text", Text: "next question"}}},
	}
	fixed := RepairToolUsePairing(msgs)
	if len(fixed) != 3 {
		t.Fatalf("messages = %d, want 3", len(fixed))
	}
	if fixed[2].Role != llm.RoleUser {
		t.Fatalf("messages[2].role = %s, want user tool_result", fixed[2].Role)
	}
	found := false
	for _, block := range fixed[2].Content {
		if block.Type == "tool_result" && block.ToolUseID == "call_1" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected recovered tool_result for call_1")
	}
	if fixed[2].Content[0].Text != "next question" {
		t.Fatalf("original user text should be preserved, got %q", fixed[2].Content[0].Text)
	}
}

func TestRepairToolUsePairingPatchesPartialResults(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: "tool_use", ID: "a", Name: "bash"},
			{Type: "tool_use", ID: "b", Name: "bash"},
		}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{
			{Type: "tool_result", ToolUseID: "a", Content: "ok"},
		}},
	}
	fixed := RepairToolUsePairing(msgs)
	if len(fixed) != 2 {
		t.Fatalf("messages = %d, want 2", len(fixed))
	}
	count := 0
	for _, block := range fixed[1].Content {
		if block.Type == "tool_result" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("tool_result count = %d, want 2", count)
	}
}

func TestAppendUniqueToolUse(t *testing.T) {
	block := llm.ContentBlock{Type: "tool_use", ID: "x", Name: "bash"}
	uses := appendUniqueToolUse(nil, block)
	uses = appendUniqueToolUse(uses, block)
	if len(uses) != 1 {
		t.Fatalf("tool uses = %d, want 1", len(uses))
	}
}
