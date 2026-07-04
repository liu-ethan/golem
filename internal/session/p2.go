package session

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tencent-docs/golem/internal/llm"
)

// DenialEntry 表示 /permissions Recently denied 页中的一条拒绝记录。
type DenialEntry struct {
	ID        string
	Tool      string
	Input     string
	Reason    string
	CreatedAt time.Time
}

const p2SchemaSQL = `
CREATE TABLE IF NOT EXISTS denial_log (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    tool        TEXT NOT NULL,
    input       TEXT NOT NULL,
    reason      TEXT NOT NULL,
    created_at  DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_denial_log_project_id ON denial_log(project_id);
CREATE TABLE IF NOT EXISTS project_settings (
    project_id TEXT PRIMARY KEY,
    memory_inject_enabled INTEGER NOT NULL DEFAULT 1
);
`

// migrateP2 执行 P2 schema 扩展；ALTER COLUMN 失败时忽略（列已存在）。
func (s *Store) migrateP2() error {
	if _, err := s.db.Exec(p2SchemaSQL); err != nil {
		return fmt.Errorf("migrate p2 tables: %w", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE sessions ADD COLUMN name TEXT`,
		`ALTER TABLE sessions ADD COLUMN forked_from TEXT`,
	} {
		_, _ = s.db.Exec(stmt)
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO project_settings (project_id, memory_inject_enabled) VALUES (?, 1)`,
		s.projectID,
	)
	if err != nil {
		return fmt.Errorf("ensure project settings: %w", err)
	}
	return nil
}

// DeleteSession 删除指定会话及其消息；session 不存在时返回 error。
func (s *Store) DeleteSession(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		`DELETE FROM sessions WHERE id = ? AND project_id = ?`,
		sessionID, s.projectID,
	)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	if _, err := tx.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete session: %w", err)
	}
	return nil
}

// RenameSession 为当前会话写入显示名称，供 /sessions 列表展示。
func (s *Store) RenameSession(sessionID, name string) error {
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("session name is required")
	}
	res, err := s.db.Exec(
		`UPDATE sessions SET name = ? WHERE id = ? AND project_id = ?`,
		name, sessionID, s.projectID,
	)
	if err != nil {
		return fmt.Errorf("rename session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		if err := s.EnsureSession(sessionID); err != nil {
			return err
		}
		_, err = s.db.Exec(
			`UPDATE sessions SET name = ? WHERE id = ? AND project_id = ?`,
			name, sessionID, s.projectID,
		)
		if err != nil {
			return fmt.Errorf("rename session after ensure: %w", err)
		}
	}
	return nil
}

// ForkSession 复制源会话消息到新 session，并记录 forked_from。
func (s *Store) ForkSession(fromSessionID string) (newSessionID string, err error) {
	if fromSessionID == "" {
		return "", fmt.Errorf("session id is required")
	}
	summary, msgs, err := s.LoadSession(fromSessionID)
	if err != nil {
		return "", err
	}
	newSessionID = uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.Exec(
		`INSERT INTO sessions (id, project_id, created_at, summary, forked_from) VALUES (?, ?, ?, ?, ?)`,
		newSessionID, s.projectID, now, nullIfEmpty(summary), fromSessionID,
	)
	if err != nil {
		return "", fmt.Errorf("insert fork session: %w", err)
	}
	if len(msgs) > 0 {
		if err := s.SyncMessages(newSessionID, msgs); err != nil {
			return "", err
		}
	}
	return newSessionID, nil
}

// InsertDenial 写入一条工具拒绝记录，供 /permissions Recently denied 展示。
func (s *Store) InsertDenial(tool, input, reason string) error {
	if tool == "" {
		return fmt.Errorf("tool is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`INSERT INTO denial_log (id, project_id, tool, input, reason, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), s.projectID, tool, input, reason, now,
	)
	if err != nil {
		return fmt.Errorf("insert denial: %w", err)
	}
	return nil
}

// ListDenials 返回当前项目最近 limit 条拒绝记录，按时间降序。
func (s *Store) ListDenials(limit int) ([]DenialEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT id, tool, input, reason, created_at FROM denial_log
		 WHERE project_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		s.projectID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list denials: %w", err)
	}
	defer rows.Close()

	var entries []DenialEntry
	for rows.Next() {
		var e DenialEntry
		var createdRaw string
		if err := rows.Scan(&e.ID, &e.Tool, &e.Input, &e.Reason, &createdRaw); err != nil {
			return nil, fmt.Errorf("scan denial: %w", err)
		}
		e.CreatedAt, err = parseSQLiteTime(createdRaw)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate denials: %w", err)
	}
	return entries, nil
}

// MemoryInjectEnabled 返回当前项目是否启用 BM25 记忆注入；默认 true。
func (s *Store) MemoryInjectEnabled() (bool, error) {
	var enabled int
	err := s.db.QueryRow(
		`SELECT memory_inject_enabled FROM project_settings WHERE project_id = ?`,
		s.projectID,
	).Scan(&enabled)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("read memory inject enabled: %w", err)
	}
	return enabled != 0, nil
}

// SetMemoryInjectEnabled 切换 BM25 记忆注入开关。
func (s *Store) SetMemoryInjectEnabled(enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO project_settings (project_id, memory_inject_enabled) VALUES (?, ?)
		 ON CONFLICT(project_id) DO UPDATE SET memory_inject_enabled = excluded.memory_inject_enabled`,
		s.projectID, val,
	)
	if err != nil {
		return fmt.Errorf("set memory inject enabled: %w", err)
	}
	return nil
}

// SessionName 返回会话显示名；无 name 时返回空字符串。
func (s *Store) SessionName(sessionID string) (string, error) {
	var nameNull sql.NullString
	err := s.db.QueryRow(
		`SELECT name FROM sessions WHERE id = ? AND project_id = ?`,
		sessionID, s.projectID,
	).Scan(&nameNull)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return "", fmt.Errorf("read session name: %w", err)
	}
	if nameNull.Valid {
		return nameNull.String, nil
	}
	return "", nil
}

// ExportMessages 将会话消息格式化为 markdown 文本。
func ExportMessages(msgs []llm.Message) string {
	var b strings.Builder
	for _, msg := range msgs {
		switch msg.Role {
		case llm.RoleUser:
			for _, block := range msg.Content {
				if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
					b.WriteString("## User\n\n")
					b.WriteString(block.Text)
					b.WriteString("\n\n")
				}
			}
		case llm.RoleAssistant:
			for _, block := range msg.Content {
				switch block.Type {
				case "text":
					if strings.TrimSpace(block.Text) != "" {
						b.WriteString("## Assistant\n\n")
						b.WriteString(block.Text)
						b.WriteString("\n\n")
					}
				case "tool_use":
					b.WriteString("### Tool: ")
					b.WriteString(block.Name)
					b.WriteString("\n\n")
				}
			}
		}
	}
	return b.String()
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
