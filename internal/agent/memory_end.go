package agent

import (
	"context"

	"github.com/tencent-docs/golem/internal/config"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/memory"
)

// ChainEndHandler 按顺序调用多个 SessionEndHandler。
type ChainEndHandler []SessionEndHandler

// OnSessionEnd 依次触发链中每个 handler。
func (c ChainEndHandler) OnSessionEnd(sessionID string, hadUserMessages bool) {
	for _, h := range c {
		if h != nil {
			h.OnSessionEnd(sessionID, hadUserMessages)
		}
	}
}

// OnSessionEndSnapshot 使用会话快照收尾；handler 未实现快照接口时回退为 OnSessionEnd。
func (c ChainEndHandler) OnSessionEndSnapshot(snap SessionEndSnapshot) {
	for _, h := range c {
		if h == nil {
			continue
		}
		switch sh := h.(type) {
		case interface {
			OnSessionEndSnapshot(snap SessionEndSnapshot)
		}:
			sh.OnSessionEndSnapshot(snap)
		case interface {
			OnSessionEndWithMessages(sessionID string, hadUserMessages bool, messages []llm.Message)
		}:
			sh.OnSessionEndWithMessages(snap.SessionID, snap.HadUserMessages, snap.Messages)
		default:
			h.OnSessionEnd(snap.SessionID, snap.HadUserMessages)
		}
	}
}

// SessionMessageSource 供会话结束时读取当前消息快照。
type SessionMessageSource interface {
	Messages() []llm.Message
}

// MemoryFactStore 供 Layer 1/2 读写 SQLite 中的情节记忆与 profile_jobs。
type MemoryFactStore interface {
	memory.FactStore
	memory.ProfileStore
}

// MemoryOnEnd 在会话正常结束时同步执行 Layer 1 情节记忆提取。
type MemoryOnEnd struct {
	Store       MemoryFactStore
	Source      SessionMessageSource
	ProjectRoot string
	MemoryCfg   config.MemoryConfig
	LLM         llm.LLMClient
}

// OnSessionEnd 实现 SessionEndHandler，无 user 消息或依赖缺失时跳过。
func (m MemoryOnEnd) OnSessionEnd(sessionID string, hadUserMessages bool) {
	if !hadUserMessages || m.LLM == nil || m.Store == nil || m.Source == nil {
		return
	}
	m.runSessionEnd(sessionID, m.Source.Messages())
}

// OnSessionEndSnapshot 使用快照消息执行 Layer 1 情节记忆提取，供 /clear 异步收尾。
func (m MemoryOnEnd) OnSessionEndSnapshot(snap SessionEndSnapshot) {
	if !snap.HadUserMessages || m.LLM == nil || m.Store == nil || len(snap.Messages) == 0 {
		return
	}
	m.runSessionEnd(snap.SessionID, snap.Messages)
}

func (m MemoryOnEnd) runSessionEnd(sessionID string, messages []llm.Message) {
	ctx := context.Background()
	_ = memory.OnSessionEnd(ctx, memory.SessionEndParams{
		SessionID:   sessionID,
		ProjectID:   m.Store.ProjectIDValue(),
		ProjectRoot: m.ProjectRoot,
		Messages:    messages,
		Config:      m.MemoryCfg,
		LLM:         m.LLM,
		Store:       m.Store,
	})
}
