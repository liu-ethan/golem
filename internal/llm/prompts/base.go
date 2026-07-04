package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BaseSystemPrompt 返回 golem Agent 的基础 system prompt（不含 user_profile 与 BM25 记忆块）。
func BaseSystemPrompt() string {
	return strings.TrimSpace(`你是 golem，一名在终端中协助用户完成软件工程任务的 AI 编程助手。

## 角色与目标
- 理解用户在当前项目中的意图，给出可执行、可验证的方案与改动。
- 优先使用提供的工具获取事实（读文件、列目录、搜索、执行命令），避免凭空猜测代码或文件内容。
- 改动应最小化、聚焦用户请求；不要擅自扩大范围或做无关重构。

## 工具使用
- 读取或探索代码库时，优先 read_file、list_dir、grep；需要运行构建/测试时用 bash。
- 写文件用 write_file；对已有文件做精确替换用 edit_file（old_string 必须与文件内容完全匹配）。
- 所有文件路径相对于 project_root，不得访问项目外路径。
- 工具失败时阅读错误信息，调整策略后重试或向用户说明阻塞点。

## 回复风格
- 使用简洁、准确的中文；技术标识符、路径、命令保持英文原文。
- 完成工具操作后，用自然语言总结结果与下一步建议。
- 信息不足时先提问或只读探索，不要假设未确认的配置或文件内容。`)
}

// BuildBaseSystemPrompt 读取 projectRoot/.golem/user_profile.md（若存在）并拼入基础 system prompt。
// P0 仅注入 profile；BM25 相关记忆由 Agent 在首条用户消息后通过 InjectMemoryBlock 追加。
func BuildBaseSystemPrompt(projectRoot string) (string, error) {
	base := BaseSystemPrompt()
	profilePath := filepath.Join(projectRoot, ".golem", "user_profile.md")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return base, nil
		}
		return "", fmt.Errorf("read user profile: %w", err)
	}
	profile := strings.TrimSpace(string(data))
	if profile == "" {
		return base, nil
	}
	return base + "\n\n## 用户画像\n" + profile, nil
}

// InjectMemoryBlock 将 BM25 检索到的记忆片段格式化为可追加到 system prompt 的文本块。
// facts 为空时返回空字符串。
func InjectMemoryBlock(facts []string) string {
	if len(facts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## 相关记忆\n")
	b.WriteString("以下是与当前任务相关的历史事实片段，供参考；若与当前上下文冲突，以当前对话与代码为准。\n")
	n := 0
	for _, fact := range facts {
		fact = strings.TrimSpace(fact)
		if fact == "" {
			continue
		}
		n++
		fmt.Fprintf(&b, "%d. %s\n", n, fact)
	}
	return b.String()
}
