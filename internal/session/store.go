package session

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tencent-docs/golem/internal/llm"
	"github.com/tencent-docs/golem/internal/memory"
	_ "modernc.org/sqlite"
)

const (
	defaultListLimit = 20
)

// Entry 表示 /sessions 列表中的单条会话摘要。
type Entry struct {
	ID        string
	CreatedAt time.Time
	Summary   string
	Preview   string
}

// Store 提供 SQLite 会话与消息持久化，数据库位于 <project_root>/.golem/data/golem.db。
type Store struct {
	db        *sql.DB
	projectID string
}

// ProjectID 根据 project_root 计算 sha256 哈希的前 16 个十六进制字符。
func ProjectID(projectRoot string) string {
	sum := sha256.Sum256([]byte(projectRoot))
	return hex.EncodeToString(sum[:])[:16]
}

// DBPath 返回项目级 SQLite 数据库文件路径。
func DBPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".golem", "data", "golem.db")
}

// Open 打开或创建项目级 SQLite 数据库并初始化 sessions / messages 表。
func Open(projectRoot string) (*Store, error) {
	dataDir := filepath.Join(projectRoot, ".golem", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", DBPath(projectRoot))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	st := &Store{
		db:        db,
		projectID: ProjectID(projectRoot),
	}
	if err := st.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

// Close 关闭底层数据库连接。
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// ProjectIDValue 返回当前 Store 绑定的 project_id。
func (s *Store) ProjectIDValue() string {
	return s.projectID
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    summary TEXT
);
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_project_id ON sessions(project_id);
CREATE TABLE IF NOT EXISTS memory_facts (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL,
    project_id  TEXT NOT NULL,
    content     TEXT NOT NULL,
    category    TEXT NOT NULL,
    created_at  DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_memory_facts_project_id ON memory_facts(project_id);
CREATE TABLE IF NOT EXISTS profile_jobs (
    project_id       TEXT PRIMARY KEY,
    last_run_at      DATETIME,
    session_count    INTEGER DEFAULT 0,
    status           TEXT DEFAULT 'idle'
);
`

// migrate 执行建表 DDL；幂等，可重复调用。
func (s *Store) migrate() error {
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	return nil
}

// EnsureSession 确保 sessions 表中存在指定 ID 的记录；不存在则插入。
func (s *Store) EnsureSession(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO sessions (id, project_id, created_at, summary) VALUES (?, ?, ?, NULL)`,
		sessionID, s.projectID, now,
	)
	if err != nil {
		return fmt.Errorf("ensure session: %w", err)
	}
	return nil
}

// SyncMessages 用 messages 全量替换指定会话在库中的消息行，保持顺序与 Agent 内存一致。
func (s *Store) SyncMessages(sessionID string, msgs []llm.Message) error {
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}
	if err := s.EnsureSession(sessionID); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}

	baseTime := time.Now().UTC()
	for i, msg := range msgs {
		contentJSON, err := json.Marshal(msg.Content)
		if err != nil {
			return fmt.Errorf("marshal message content: %w", err)
		}
		createdAt := baseTime.Add(time.Duration(i) * time.Microsecond).Format(time.RFC3339Nano)
		_, err = tx.Exec(
			`INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
			uuid.NewString(), sessionID, string(msg.Role), string(contentJSON), createdAt,
		)
		if err != nil {
			return fmt.Errorf("insert message: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sync messages: %w", err)
	}
	return nil
}

// UpdateSummary 写入 Layer 0 压缩摘要到 sessions.summary。
func (s *Store) UpdateSummary(sessionID, summary string) error {
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}
	res, err := s.db.Exec(
		`UPDATE sessions SET summary = ? WHERE id = ? AND project_id = ?`,
		summary, sessionID, s.projectID,
	)
	if err != nil {
		return fmt.Errorf("update summary: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	return nil
}

// LoadSession 加载指定会话的 summary 与按时间排序的消息列表，供 --resume 还原上下文。
func (s *Store) LoadSession(sessionID string) (summary string, messages []llm.Message, err error) {
	if sessionID == "" {
		return "", nil, fmt.Errorf("session id is required")
	}

	var summaryNull sql.NullString
	err = s.db.QueryRow(
		`SELECT summary FROM sessions WHERE id = ? AND project_id = ?`,
		sessionID, s.projectID,
	).Scan(&summaryNull)
	if err == sql.ErrNoRows {
		return "", nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return "", nil, fmt.Errorf("load session: %w", err)
	}
	if summaryNull.Valid {
		summary = summaryNull.String
	}

	rows, err := s.db.Query(
		`SELECT role, content FROM messages WHERE session_id = ? ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return "", nil, fmt.Errorf("load messages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var role, contentJSON string
		if err := rows.Scan(&role, &contentJSON); err != nil {
			return "", nil, fmt.Errorf("scan message: %w", err)
		}
		var blocks []llm.ContentBlock
		if err := json.Unmarshal([]byte(contentJSON), &blocks); err != nil {
			return "", nil, fmt.Errorf("unmarshal message content: %w", err)
		}
		messages = append(messages, llm.Message{
			Role:    llm.Role(role),
			Content: blocks,
		})
	}
	if err := rows.Err(); err != nil {
		return "", nil, fmt.Errorf("iterate messages: %w", err)
	}
	return summary, messages, nil
}

// ListSessions 列出当前 project 最近 limit 条会话，按 created_at 降序；limit ≤ 0 时使用 20。
func (s *Store) ListSessions(limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = defaultListLimit
	}

	rows, err := s.db.Query(
		`SELECT id, created_at, COALESCE(summary, '') FROM sessions
		 WHERE project_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		s.projectID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	var ids []string
	for rows.Next() {
		var e Entry
		var createdRaw string
		if err := rows.Scan(&e.ID, &createdRaw, &e.Summary); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		e.CreatedAt, err = parseSQLiteTime(createdRaw)
		if err != nil {
			return nil, err
		}
		ids = append(ids, e.ID)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}
	rows.Close()

	for i, id := range ids {
		preview, err := s.firstUserPreview(id)
		if err != nil {
			return nil, err
		}
		entries[i].Preview = preview
	}
	return entries, nil
}

// firstUserPreview 返回会话首条 user 消息的文本摘要，供 /sessions 列表展示。
func (s *Store) firstUserPreview(sessionID string) (string, error) {
	rows, err := s.db.Query(
		`SELECT content FROM messages WHERE session_id = ? AND role = ? ORDER BY created_at ASC`,
		sessionID, string(llm.RoleUser),
	)
	if err != nil {
		return "", fmt.Errorf("query user messages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var contentJSON string
		if err := rows.Scan(&contentJSON); err != nil {
			return "", fmt.Errorf("scan user message: %w", err)
		}
		preview := extractFirstText(contentJSON)
		if preview != "" {
			return truncatePreview(preview), nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate user messages: %w", err)
	}
	return "", nil
}

// extractFirstText 从 content JSON 数组中提取首个 text 块内容。
func extractFirstText(contentJSON string) string {
	var blocks []llm.ContentBlock
	if err := json.Unmarshal([]byte(contentJSON), &blocks); err != nil {
		return ""
	}
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			return strings.TrimSpace(b.Text)
		}
	}
	return ""
}

// truncatePreview 将预览文本截断为单行、最多 80 字符。
func truncatePreview(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	const maxLen = 80
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// parseSQLiteTime 解析 SQLite 存储的时间字符串，兼容 RFC3339Nano 与空格分隔格式。
func parseSQLiteTime(raw string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse time %q", raw)
}

// InsertMemoryFacts 将 Layer 1 提取的情节记忆写入 memory_facts 表。
func (s *Store) InsertMemoryFacts(sessionID string, facts []memory.MemoryFact) error {
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}
	if len(facts) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	for i, fact := range facts {
		id := fact.ID
		if id == "" {
			id = uuid.NewString()
		}
		projectID := fact.ProjectID
		if projectID == "" {
			projectID = s.projectID
		}
		createdAt := fact.CreatedAt
		if createdAt.IsZero() {
			createdAt = now.Add(time.Duration(i) * time.Microsecond)
		}
		_, err := tx.Exec(
			`INSERT INTO memory_facts (id, session_id, project_id, content, category, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			id, sessionID, projectID, fact.Content, fact.Category, createdAt.UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("insert memory fact: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit memory facts: %w", err)
	}
	return nil
}

// ListMemoryFacts 返回当前项目的全部情节记忆，供 BM25 检索使用。
func (s *Store) ListMemoryFacts() ([]memory.MemoryFact, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, project_id, content, category, created_at FROM memory_facts
		 WHERE project_id = ?
		 ORDER BY created_at ASC`,
		s.projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list memory facts: %w", err)
	}
	defer rows.Close()

	var facts []memory.MemoryFact
	for rows.Next() {
		var f memory.MemoryFact
		var createdRaw string
		if err := rows.Scan(&f.ID, &f.SessionID, &f.ProjectID, &f.Content, &f.Category, &createdRaw); err != nil {
			return nil, fmt.Errorf("scan memory fact: %w", err)
		}
		f.CreatedAt, err = parseSQLiteTime(createdRaw)
		if err != nil {
			return nil, err
		}
		facts = append(facts, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory facts: %w", err)
	}
	return facts, nil
}

// IncrementSessionCount 将会话结束计数加一并返回新值，用于 Layer 2 触发判断。
func (s *Store) IncrementSessionCount() (int, error) {
	_, err := s.db.Exec(
		`INSERT INTO profile_jobs (project_id, session_count, status) VALUES (?, 1, 'idle')
		 ON CONFLICT(project_id) DO UPDATE SET session_count = session_count + 1`,
		s.projectID,
	)
	if err != nil {
		return 0, fmt.Errorf("increment session count: %w", err)
	}

	var count int
	err = s.db.QueryRow(
		`SELECT session_count FROM profile_jobs WHERE project_id = ?`,
		s.projectID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("read session count: %w", err)
	}
	return count, nil
}

// ResetSessionCount 将 profile_jobs.session_count 归零，Layer 2 合并完成后调用。
func (s *Store) ResetSessionCount() error {
	_, err := s.db.Exec(
		`INSERT INTO profile_jobs (project_id, session_count, status) VALUES (?, 0, 'idle')
		 ON CONFLICT(project_id) DO UPDATE SET session_count = 0`,
		s.projectID,
	)
	if err != nil {
		return fmt.Errorf("reset session count: %w", err)
	}
	return nil
}

// DeleteAllFacts 删除当前项目的全部 memory_facts，Layer 2 合并完成后调用。
func (s *Store) DeleteAllFacts() error {
	_, err := s.db.Exec(`DELETE FROM memory_facts WHERE project_id = ?`, s.projectID)
	if err != nil {
		return fmt.Errorf("delete memory facts: %w", err)
	}
	return nil
}
