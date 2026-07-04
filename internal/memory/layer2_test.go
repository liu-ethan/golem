package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/llm/prompts"
	"github.com/tencent-docs/golem/internal/testutil"
)

func TestRunLayer2WritesProfileAndClearsFacts(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	store := &stubFactStore{
		projectID: "proj-1",
		facts: []MemoryFact{
			{Content: "用户偏好 tabs 缩进", Category: "preference"},
			{Content: "golem 使用 SQLite", Category: "project_fact"},
		},
		count: 3,
	}

	mock := testutil.NewMockLLM()
	mock.CompleteText = "# 用户画像（2026-07-04 更新，基于 3 次会话）\n\n## 技术偏好\n- tabs 缩进"

	if err := RunLayer2(context.Background(), store.projectID, root, store, mock); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".golem", "user_profile.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "用户画像") {
		t.Errorf("profile = %q", data)
	}

	if len(store.facts) != 0 {
		t.Fatalf("facts after layer2 = %d, want 0", len(store.facts))
	}
	if store.count != 0 {
		t.Errorf("session count = %d, want 0", store.count)
	}
	if len(mock.CompleteCalls) != 1 {
		t.Fatalf("Complete calls = %d, want 1", len(mock.CompleteCalls))
	}
	if mock.CompleteCalls[0].System != prompts.Layer2SystemPrompt() {
		t.Error("Complete should use Layer2 system prompt")
	}
}

func TestRunLayer2SkipsLLMWhenNoFacts(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	store := &stubFactStore{projectID: "proj-1", count: 3}

	mock := testutil.NewMockLLM()
	if err := RunLayer2(context.Background(), store.projectID, root, store, mock); err != nil {
		t.Fatal(err)
	}

	if len(mock.CompleteCalls) != 0 {
		t.Fatalf("Complete calls = %d, want 0", len(mock.CompleteCalls))
	}
	if _, err := os.Stat(filepath.Join(root, ".golem", "user_profile.md")); err == nil {
		t.Error("profile file should not be created when there are no facts")
	}
	if store.count != 0 {
		t.Errorf("session count = %d, want 0", store.count)
	}
}

func TestRunLayer2IncludesExistingProfileInMergeInput(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	existing := "# 用户画像（旧）\n\n## 技术偏好\n- Go"
	if err := os.WriteFile(filepath.Join(root, ".golem", "user_profile.md"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &stubFactStore{
		projectID: "proj-1",
		facts: []MemoryFact{
			{Content: "用户偏好 tabs", Category: "preference"},
		},
		count: 3,
	}

	mock := testutil.NewMockLLM()
	mock.CompleteText = "# 用户画像（新）"

	if err := RunLayer2(context.Background(), "proj-1", root, store, mock); err != nil {
		t.Fatal(err)
	}

	if len(mock.CompleteCalls) != 1 {
		t.Fatal("expected one Complete call")
	}
	userText := mock.CompleteCalls[0].Messages[0].Content[0].Text
	if !strings.Contains(userText, "用户画像（旧）") {
		t.Errorf("merge input missing existing profile: %q", userText)
	}
	if !strings.Contains(userText, "session_count: 3") {
		t.Errorf("merge input missing session_count: %q", userText)
	}
}

func TestFactsForMergeSkipsEmptyEntries(t *testing.T) {
	got := factsForMerge([]MemoryFact{
		{Content: "有效", Category: "preference"},
		{Content: "  ", Category: "preference"},
		{Content: "无类别", Category: ""},
	})
	if len(got) != 1 {
		t.Fatalf("facts = %d, want 1", len(got))
	}
}

func TestMergeProfileStripsMarkdownFence(t *testing.T) {
	mock := testutil.NewMockLLM()
	mock.CompleteText = "```markdown\n# 用户画像\n\n## 技术偏好\n- Go\n```"

	profile, err := mergeProfile(context.Background(), mock, "", []MemoryFact{
		{Content: "用户偏好 Go", Category: "preference"},
	}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(profile, "```") {
		t.Errorf("profile should not include fence: %q", profile)
	}
	if !strings.HasPrefix(profile, "# 用户画像") {
		t.Errorf("profile = %q", profile)
	}
}
