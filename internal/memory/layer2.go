package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/llm/prompts"
)

const userProfileFile = "user_profile.md"

// ProfileStore 供 Layer 2 合并用户画像时读写 SQLite 与 profile 文件。
type ProfileStore interface {
	FactStore
	ListMemoryFacts() ([]MemoryFact, error)
	SessionCount() (int, error)
	DeleteAllFacts() error
	ResetSessionCount() error
}

// RunLayer2 将 Layer 1 碎片合并为 user_profile.md，随后清空 memory_facts 并重置会话计数。
func RunLayer2(ctx context.Context, _ string, projectRoot string, store ProfileStore, client llm.LLMClient) error {
	if store == nil {
		return nil
	}

	facts, err := store.ListMemoryFacts()
	if err != nil {
		return fmt.Errorf("list memory facts: %w", err)
	}

	sessionCount, err := store.SessionCount()
	if err != nil {
		return fmt.Errorf("read session count: %w", err)
	}

	existing, err := readExistingProfile(projectRoot)
	if err != nil {
		return err
	}

	if len(facts) > 0 && client != nil {
		profile, err := mergeProfile(ctx, client, existing, facts, sessionCount)
		if err != nil {
			return fmt.Errorf("merge profile: %w", err)
		}
		if err := writeUserProfile(projectRoot, profile); err != nil {
			return err
		}
	}

	if err := store.DeleteAllFacts(); err != nil {
		return fmt.Errorf("delete memory facts: %w", err)
	}
	if err := store.ResetSessionCount(); err != nil {
		return fmt.Errorf("reset session count: %w", err)
	}
	return nil
}

// readExistingProfile 读取 projectRoot/.golem/user_profile.md，不存在时返回空字符串。
func readExistingProfile(projectRoot string) (string, error) {
	path := filepath.Join(projectRoot, ".golem", userProfileFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read user profile: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// writeUserProfile 将合并后的画像写入 projectRoot/.golem/user_profile.md。
func writeUserProfile(projectRoot, profile string) error {
	dir := filepath.Join(projectRoot, ".golem")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create .golem dir: %w", err)
	}
	path := filepath.Join(dir, userProfileFile)
	if err := os.WriteFile(path, []byte(strings.TrimSpace(profile)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write user profile: %w", err)
	}
	return nil
}

// mergeProfile 调用 Complete 将现有画像与情节记忆碎片合并为 user_profile.md 正文。
func mergeProfile(ctx context.Context, client llm.LLMClient, existing string, facts []MemoryFact, sessionCount int) (string, error) {
	payload, err := json.Marshal(factsForMerge(facts))
	if err != nil {
		return "", fmt.Errorf("marshal facts: %w", err)
	}

	existingLabel := existing
	if existingLabel == "" {
		existingLabel = "(无)"
	}

	userText := fmt.Sprintf(`session_count: %d
现有画像:
%s

待合并碎片:
%s`, sessionCount, existingLabel, string(payload))

	text, _, err := client.Complete(ctx, llm.CompleteRequest{
		System: prompts.Layer2SystemPrompt(),
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{{
				Type: "text",
				Text: userText,
			}},
		}},
		MaxTokens: 4096,
	})
	if err != nil {
		return "", err
	}
	return stripJSONCodeFence(strings.TrimSpace(text)), nil
}

// factsForMerge 将 MemoryFact 转为供 LLM 阅读的精简 JSON 对象列表。
func factsForMerge(facts []MemoryFact) []map[string]string {
	out := make([]map[string]string, 0, len(facts))
	for _, f := range facts {
		content := strings.TrimSpace(f.Content)
		category := strings.TrimSpace(f.Category)
		if content == "" || category == "" {
			continue
		}
		out = append(out, map[string]string{
			"content":  content,
			"category": category,
		})
	}
	return out
}
