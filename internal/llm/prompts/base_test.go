package prompts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/llm/prompts"
	"github.com/tencent-docs/golem/internal/testutil"
)

func TestBuildBaseSystemPromptWithoutProfile(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	got, err := prompts.BuildBaseSystemPrompt(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "golem") {
		t.Error("expected base prompt content")
	}
	if strings.Contains(got, "用户画像") {
		t.Error("should not contain profile section")
	}
}

func TestBuildBaseSystemPromptWithProfile(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	profile := "# 用户画像\n- 使用 tabs\n"
	path := filepath.Join(root, ".golem", "user_profile.md")
	if err := os.WriteFile(path, []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := prompts.BuildBaseSystemPrompt(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "用户画像") || !strings.Contains(got, "tabs") {
		t.Errorf("got = %q", got)
	}
}

func TestInjectMemoryBlockEmpty(t *testing.T) {
	if got := prompts.InjectMemoryBlock(nil); got != "" {
		t.Errorf("got = %q", got)
	}
}

func TestInjectMemoryBlockFormatsFacts(t *testing.T) {
	got := prompts.InjectMemoryBlock([]string{"偏好 Go", "  ", "项目使用 SQLite"})
	if !strings.Contains(got, "## 相关记忆") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "1. 偏好 Go") || !strings.Contains(got, "2. 项目使用 SQLite") {
		t.Errorf("got = %q", got)
	}
}
