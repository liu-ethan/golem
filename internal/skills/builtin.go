package skills

// builtinSkills 返回编译进二进制的示范 Skill。
func builtinSkills() []Skill {
	return []Skill{
		{
			Name:   "golang-expert",
			Source: "builtin",
			SystemPrompt: `你是 Go 专家，遵循 Effective Go 与项目现有风格。
- error 必须显式处理，避免滥用 panic。
- 优先使用标准库与项目已有抽象，改动保持最小。
- 测试与源码同包 colocated，遵循 Go 惯例。`,
			AllowedTools: []string{"bash", "read_file", "write_file", "grep", "edit_file", "list_dir"},
			DeniedTools:  []string{"web_search"},
			Rules:        []string{"allow go *", "allow git *", "deny rm -rf *"},
		},
		{
			Name:   "code-reviewer",
			Source: "builtin",
			SystemPrompt: `你是严格的 code reviewer，聚焦正确性、安全与可维护性。
- 先读 diff/相关文件，再给出按优先级排序的问题清单。
- 区分 blocker / suggestion / nit，并给出具体修复建议。
- 不擅自扩大改动范围，不做无关重构建议。`,
			AllowedTools: []string{"bash", "read_file", "grep", "list_dir"},
			DeniedTools:  []string{"write_file", "edit_file", "web_search"},
		},
	}
}
