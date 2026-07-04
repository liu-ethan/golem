package memory

import "time"

// MemoryFact 表示 Layer 1 提取的一条跨会话情节记忆。
type MemoryFact struct {
	ID        string
	SessionID string
	ProjectID string
	Content   string
	Category  string // preference | project_fact | task_progress
	CreatedAt time.Time
}
