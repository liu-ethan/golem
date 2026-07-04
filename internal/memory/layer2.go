package memory

import (
	"context"

	"github.com/tencent-docs/golem/internal/llm"
)

// ProfileStore 供 Layer 2 合并用户画像时读写 SQLite 与 profile 文件。
type ProfileStore interface {
	FactStore
	DeleteAllFacts() error
	ResetSessionCount() error
}

// RunLayer2 将 Layer 1 碎片合并为 user_profile.md；Step 12 实现，当前为占位。
func RunLayer2(_ context.Context, _ string, _ string, _ ProfileStore, _ llm.LLMClient) error {
	return nil
}
