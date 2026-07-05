package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/tencent-docs/golem/internal/tools"
	"github.com/tencent-docs/golem/internal/tui/style"
)

const maxToolDiffLines = 48

type diffOp int

const (
	diffEqual diffOp = iota
	diffDelete
	diffInsert
)

type diffLine struct {
	op   diffOp
	text string
}

// formatToolDetail 渲染工具卡片正文；write_file/edit_file 展示 git 风格 diff。
func formatToolDetail(toolName string, input map[string]any, projectRoot string, width int) string {
	if len(input) == 0 {
		return ""
	}
	switch toolName {
	case "write_file":
		return formatWriteFileDiff(input, projectRoot, width)
	case "edit_file":
		return formatEditFileDiff(input, projectRoot, width)
	case "bash":
		return formatBashInput(input, width)
	default:
		return formatToolInput(input)
	}
}

// formatBashInput 渲染 bash 命令行，以 $ 前缀 + CodeText 对标 Claude Code 终端风格。
func formatBashInput(input map[string]any, width int) string {
	cmd, ok := inputString(input, "command")
	if !ok || cmd == "" {
		return formatToolInput(input)
	}
	inner := toolInnerWidth(width)
	line := "$ " + cmd
	return "│ " + style.CodeText.Render(truncateRunes(line, inner)) + "\n"
}

// formatBashOutput 渲染 bash 执行结果；成功用 CodeText，失败用 ErrText，过长时截断。
func formatBashOutput(output string, isErr bool, width int) string {
	output = strings.TrimRight(output, "\n")
	if output == "" {
		return ""
	}
	inner := toolInnerWidth(width)
	bodyStyle := style.CodeText
	if isErr {
		bodyStyle = style.ErrText
	}
	var b strings.Builder
	lines := strings.Split(output, "\n")
	shown := 0
	omitted := 0
	for _, ln := range lines {
		if shown >= maxToolDiffLines {
			omitted++
			continue
		}
		b.WriteString("│ ")
		b.WriteString(bodyStyle.Render(truncateRunes(ln, inner)))
		b.WriteString("\n")
		shown++
	}
	if omitted > 0 {
		b.WriteString("│ ")
		b.WriteString(style.Muted.Render(fmt.Sprintf("… 还有 %d 行未显示", omitted)))
		b.WriteString("\n")
	}
	return b.String()
}

// toolInnerWidth 返回工具卡片正文可用宽度（扣除边框与 padding）。
func toolInnerWidth(width int) int {
	inner := width - 4
	if inner < 12 {
		return 12
	}
	return inner
}

// formatWriteFileDiff 渲染 write_file 确认内容：路径 + 行级 diff（红删绿增）。
func formatWriteFileDiff(input map[string]any, projectRoot string, width int) string {
	path, _ := inputString(input, "path")
	content, _ := inputString(input, "content")
	if path == "" {
		return formatToolInput(input)
	}
	oldText := readProjectFile(projectRoot, path)
	lines := lineDiff(oldText, content)
	return renderFileDiffHeader(path, lines, width)
}

// formatEditFileDiff 渲染 edit_file 确认内容：路径 + old/new 行级 diff。
func formatEditFileDiff(input map[string]any, projectRoot string, width int) string {
	path, _ := inputString(input, "path")
	oldStr, _ := inputString(input, "old_string")
	newStr, _ := inputString(input, "new_string")
	if path == "" {
		return formatToolInput(input)
	}
	var chunks []diffLine
	for _, line := range splitLines(oldStr) {
		chunks = append(chunks, diffLine{op: diffDelete, text: line})
	}
	for _, line := range splitLines(newStr) {
		chunks = append(chunks, diffLine{op: diffInsert, text: line})
	}
	if len(chunks) == 0 {
		return renderFileDiffHeader(path, nil, width)
	}
	return renderFileDiffHeader(path, chunks, width)
}

// readProjectFile 读取 projectRoot 内相对路径文件；不存在或读失败时返回空字符串。
func readProjectFile(projectRoot, path string) string {
	abs, err := tools.ValidatePath(projectRoot, path)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return ""
	}
	return string(data)
}

// lineDiff 计算两段文本的行级 diff（LCS），用于 write_file 前后对比。
func lineDiff(oldText, newText string) []diffLine {
	a := splitLines(oldText)
	b := splitLines(newText)
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}

	// dp[i][j] = LCS length of a[i:] and b[j:]
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var out []diffLine
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			out = append(out, diffLine{op: diffEqual, text: a[i]})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			out = append(out, diffLine{op: diffDelete, text: a[i]})
			i++
		} else {
			out = append(out, diffLine{op: diffInsert, text: b[j]})
			j++
		}
	}
	for i < n {
		out = append(out, diffLine{op: diffDelete, text: a[i]})
		i++
	}
	for j < m {
		out = append(out, diffLine{op: diffInsert, text: b[j]})
		j++
	}
	return out
}

// renderFileDiffHeader 渲染路径标题与 git 风格 diff 行（- 红 + 绿）。
func renderFileDiffHeader(path string, lines []diffLine, width int) string {
	inner := width - 4
	if inner < 12 {
		inner = 12
	}
	var b strings.Builder
	b.WriteString("│ ")
	b.WriteString(style.PathText.Render(path))
	b.WriteString("\n")

	changes := filterDiffChanges(lines)
	if len(changes) == 0 {
		b.WriteString("│ ")
		b.WriteString(style.Muted.Render("（无变更）"))
		b.WriteString("\n")
		return b.String()
	}

	shown := 0
	omitted := 0
	for _, ln := range changes {
		if shown >= maxToolDiffLines {
			omitted++
			continue
		}
		b.WriteString(renderDiffLine(ln, inner))
		shown++
	}
	if omitted > 0 {
		b.WriteString("│ ")
		b.WriteString(style.Muted.Render(fmt.Sprintf("… 还有 %d 行未显示", omitted)))
		b.WriteString("\n")
	}
	return b.String()
}

// filterDiffChanges 仅保留增删行，跳过未改动的 equal 行以保持确认框紧凑。
func filterDiffChanges(lines []diffLine) []diffLine {
	out := make([]diffLine, 0, len(lines))
	for _, ln := range lines {
		if ln.op != diffEqual {
			out = append(out, ln)
		}
	}
	return out
}

// renderDiffLine 渲染单行 diff：删除红 -，新增浅绿 +。
func renderDiffLine(ln diffLine, inner int) string {
	text := truncateRunes(ln.text, inner-2)
	var prefix, body string
	switch ln.op {
	case diffDelete:
		prefix = "- "
		body = style.DiffDel.Render(prefix + text)
	case diffInsert:
		prefix = "+ "
		body = style.DiffAdd.Render(prefix + text)
	default:
		prefix = "  "
		body = style.Muted.Render(prefix + text)
	}
	return "│ " + body + "\n"
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func inputString(input map[string]any, key string) (string, bool) {
	v, ok := input[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
