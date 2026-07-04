package session

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// RunListCLI 列出当前 project 的最近会话。
func RunListCLI(projectRoot string) error {
	st, err := Open(projectRoot)
	if err != nil {
		return err
	}
	defer st.Close()

	entries, err := st.ListSessions(0)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCREATED\tPREVIEW")
	for _, e := range entries {
		id := e.ID
		if len(id) > 8 {
			id = id[:8]
		}
		preview := e.Preview
		if preview == "" {
			preview = "(empty)"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", id, e.CreatedAt.Local().Format("2006-01-02 15:04"), preview)
	}
	return w.Flush()
}

// RunDeleteCLI 删除指定 session。
func RunDeleteCLI(projectRoot, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}
	st, err := Open(projectRoot)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.DeleteSession(sessionID); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "deleted session %s\n", sessionID)
	return nil
}
