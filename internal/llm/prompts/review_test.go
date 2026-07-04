package prompts

import "testing"

func TestReviewSystemPromptNonEmpty(t *testing.T) {
	if ReviewSystemPrompt() == "" {
		t.Fatal("review prompt should not be empty")
	}
}
