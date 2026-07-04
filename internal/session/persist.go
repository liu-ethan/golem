package session

import (
	"github.com/tencent-docs/golem/internal/llm"
)

// MessageSource 供持久化层读取当前会话 ID 与消息快照。
type MessageSource interface {
	SessionID() string
	Messages() []llm.Message
}

// PersistOnEnd 在会话正常结束时将消息同步到 SQLite；无 user 消息时不写入。
type PersistOnEnd struct {
	Store  *Store
	Source MessageSource
}

// OnSessionEnd 实现 agent.SessionEndHandler，在 /exit、Ctrl+D、SIGINT 等路径触发同步。
func (p PersistOnEnd) OnSessionEnd(sessionID string, hadUserMessages bool) {
	if !hadUserMessages || p.Store == nil || p.Source == nil {
		return
	}
	_ = p.Store.SyncMessages(sessionID, p.Source.Messages())
}

// SyncFromSource 立即将 Source 当前消息快照写入 Store，供每轮对话后增量持久化。
func SyncFromSource(store *Store, source MessageSource) error {
	if store == nil || source == nil {
		return nil
	}
	return store.SyncMessages(source.SessionID(), source.Messages())
}
