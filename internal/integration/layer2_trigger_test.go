package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/session"
	"github.com/tencent-docs/golem/internal/testutil"
)

// TestLayer2TriggerAfterThreeSessions 验证连续 3 次会话结束后触发 Layer 2，写入 profile 并清空 facts。
func TestLayer2TriggerAfterThreeSessions(t *testing.T) {
	root := testutil.TempProjectRoot(t)

	st, err := session.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	layer1Fact := layer1FactsJSON(map[string]string{
		"content":  "用户偏好 tabs 缩进",
		"category": "preference",
	})
	layer2Profile := "# 用户画像（2026-07-04 更新，基于 3 次会话）\n\n## 技术偏好\n- tabs 缩进\n"

	inputs := []string{"我用 tabs 缩进", "项目用 PostgreSQL", "golem 用 SQLite"}

	for i, input := range inputs {
		mock := testutil.NewMockLLM()
		mock.StreamResponses = []testutil.MockResponse{textStreamResponse("ok")}
		if i < 2 {
			mock.CompleteText = layer1Fact
		} else {
			mock.CompleteResponses = []string{layer1Fact, layer2Profile}
		}

		sessionID := newSessionID()
		ag, _ := newProductionAgent(t, root, st, mock, sessionID)
		if _, err := ag.HandleInput(context.Background(), input, nil); err != nil {
			t.Fatal(err)
		}
		ag.OnSessionEnd()
	}

	profilePath := filepath.Join(root, ".golem", "user_profile.md")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGolden(t, testdataDir, "user_profile.golden", data)

	facts, err := st.ListMemoryFacts()
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 0 {
		t.Fatalf("facts after layer2 = %d, want 0", len(facts))
	}

	count, err := st.SessionCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("session count = %d, want 0", count)
	}

	if !strings.Contains(string(data), "tabs") {
		t.Errorf("profile = %q", data)
	}
}
