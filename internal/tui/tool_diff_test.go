package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/testutil"
	"github.com/tencent-docs/golem/internal/tui/style"
)

func TestLineDiffOverwrite(t *testing.T) {
	old := "package main\n\nfunc main() {\n\tfmt.Println(\"old\")\n}\n"
	newText := "package main\n\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n"
	lines := lineDiff(old, newText)
	changes := filterDiffChanges(lines)
	var dels, adds int
	for _, ln := range changes {
		switch ln.op {
		case diffDelete:
			dels++
		case diffInsert:
			adds++
		}
	}
	if dels != 1 || adds != 1 {
		t.Fatalf("changes = %+v, want 1 del 1 add", changes)
	}
}

func TestLineDiffNewFile(t *testing.T) {
	changes := filterDiffChanges(lineDiff("", "hello\nworld\n"))
	if len(changes) != 2 {
		t.Fatalf("changes = %+v", changes)
	}
	for _, ln := range changes {
		if ln.op != diffInsert {
			t.Fatalf("want all inserts, got %+v", ln)
		}
	}
}

func TestFormatWriteFileDiffNewFile(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	out := formatWriteFileDiff(map[string]any{
		"path":    "main.go",
		"content": "package main\n\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n",
	}, root, 80)
	if !strings.Contains(out, "main.go") {
		t.Fatalf("missing path: %s", out)
	}
	if !strings.Contains(out, "+") {
		t.Fatal("expected + prefix for new file")
	}
	if !strings.Contains(stripVisible(out), "fmt.Println") {
		t.Fatal("expected content line")
	}
}

func TestFormatWriteFileDiffOverwrite(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	path := filepath.Join(root, "main.go")
	if err := os.WriteFile(path, []byte("old line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := formatWriteFileDiff(map[string]any{
		"path":    "main.go",
		"content": "new line\n",
	}, root, 80)
	plain := stripVisible(out)
	if !strings.Contains(plain, "old line") {
		t.Fatalf("missing deleted line: %s", plain)
	}
	if !strings.Contains(plain, "new line") {
		t.Fatalf("missing added line: %s", plain)
	}
}

func TestFormatEditFileDiff(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	out := formatEditFileDiff(map[string]any{
		"path":       "x.go",
		"old_string": "foo()",
		"new_string": "bar()",
	}, root, 80)
	plain := stripVisible(out)
	if !strings.Contains(plain, "- foo()") {
		t.Fatalf("missing delete: %s", plain)
	}
	if !strings.Contains(plain, "+ bar()") {
		t.Fatalf("missing add: %s", plain)
	}
}

func TestFormatToolCardWriteFileDiffColors(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	out := formatToolCard(ChatLine{
		Kind:     LineTool,
		ToolName: "write_file",
		ToolInput: map[string]any{
			"path":    "main.go",
			"content": "hello\n",
		},
		ToolState: ToolConfirm,
	}, 80, root)
	if !strings.Contains(out, style.DiffAdd.Render("+ hello")) {
		t.Fatal("expected green add line styling")
	}
	if !strings.Contains(out, "是否允许") {
		t.Fatal("expected confirm prompt")
	}
}

func TestFormatBashInput(t *testing.T) {
	out := formatBashInput(map[string]any{"command": "go run main.go"}, 80)
	if !strings.Contains(out, style.CodeText.Render("$ go run main.go")) {
		t.Fatalf("expected $ command styling: %s", out)
	}
	if strings.Contains(stripVisible(out), "command:") {
		t.Fatal("should not use generic command: prefix")
	}
}

func TestFormatToolCardBashSuccess(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	out := formatToolCard(ChatLine{
		Kind:      LineTool,
		ToolName:  "bash",
		ToolInput: map[string]any{"command": "go run main.go"},
		ToolOutput: "hello world",
		ToolState: ToolDone,
	}, 80, root)
	if !strings.Contains(out, style.CodeText.Render("$ go run main.go")) {
		t.Fatal("expected command line")
	}
	if !strings.Contains(out, style.CodeText.Render("hello world")) {
		t.Fatalf("expected output in code style: %s", stripVisible(out))
	}
	if !strings.Contains(out, style.Success.Render("[✓ 已执行]")) {
		t.Fatal("expected green success badge")
	}
}

func TestFormatBashOutputBinarySanitized(t *testing.T) {
	out := formatBashOutput("/usr/bin/go: symbolic link to ../lib/go-1.22/bin/go\n\x7fELF\x00\x01\x02", false, 80)
	plain := stripVisible(out)
	if !strings.Contains(plain, "symbolic link") {
		t.Fatalf("expected readable line preserved: %s", plain)
	}
	if strings.Contains(plain, "ELF") {
		t.Fatalf("expected binary line omitted: %s", plain)
	}
	if !strings.Contains(plain, "[二进制输出已省略]") {
		t.Fatalf("expected binary omission marker: %s", plain)
	}
}

func TestFormatToolCardBashFailure(t *testing.T) {
	root := testutil.TempProjectRoot(t)
	out := formatToolCard(ChatLine{
		Kind:       LineTool,
		ToolName:   "bash",
		ToolInput:  map[string]any{"command": "go build ./..."},
		ToolOutput: "undefined: foo",
		ToolError:  true,
		ToolState:  ToolDone,
	}, 80, root)
	if !strings.Contains(out, style.ErrText.Render("undefined: foo")) {
		t.Fatalf("expected error output styling: %s", stripVisible(out))
	}
	if !strings.Contains(out, style.ErrText.Render("[✗ 执行失败]")) {
		t.Fatal("expected red failure badge")
	}
	if strings.Contains(stripVisible(out), "[错误]") {
		t.Fatal("bash failure should use dedicated badge, not generic [错误]")
	}
	if strings.Contains(stripVisible(out), "[已拒绝]") {
		t.Fatal("exec failure must not show policy denial badge")
	}
}

func TestUpdateLastToolBashExecFailureNotDenied(t *testing.T) {
	m := testModel(t)
	m.lines = append(m.lines, ChatLine{
		Kind:      LineTool,
		ToolName:  "bash",
		ToolState: ToolRunning,
	})
	m.updateLastTool("bash", nil, "exit status 1", true)
	line := m.lines[len(m.lines)-1]
	if line.ToolState != ToolDone {
		t.Fatalf("ToolState = %v, want ToolDone", line.ToolState)
	}
	if !line.ToolError {
		t.Fatal("expected ToolError true")
	}
	out := formatToolCard(line, 80, m.projectRoot)
	if strings.Contains(stripVisible(out), "[已拒绝]") {
		t.Fatal("approved bash with non-zero exit must not show [已拒绝]")
	}
	if !strings.Contains(out, style.ErrText.Render("[✗ 执行失败]")) {
		t.Fatalf("expected exec failure badge: %s", stripVisible(out))
	}
}

func TestUpdateLastToolPolicyDenied(t *testing.T) {
	m := testModel(t)
	m.lines = append(m.lines, ChatLine{
		Kind:      LineTool,
		ToolName:  "bash",
		ToolState: ToolConfirm,
	})
	m.updateLastTool("bash", nil, "Error: user denied tool execution", true)
	line := m.lines[len(m.lines)-1]
	if line.ToolState != ToolDenied {
		t.Fatalf("ToolState = %v, want ToolDenied", line.ToolState)
	}
	out := formatToolCard(line, 80, m.projectRoot)
	if !strings.Contains(out, style.ErrText.Render("[已拒绝]")) {
		t.Fatalf("expected denial badge: %s", stripVisible(out))
	}
}
