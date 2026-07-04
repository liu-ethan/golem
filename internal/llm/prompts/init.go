package prompts

import "strings"

// InitSystemPrompt 返回 /init 生成 AGENTS.md 的 system prompt。
func InitSystemPrompt() string {
	return strings.TrimSpace(`你是项目 onboarding 助手，为当前仓库生成 AGENTS.md 行为准则文件。

## 角色与目标
- 基于项目目录结构、语言栈、测试与配置惯例，写出简洁、可执行的 AGENTS.md。
- 内容应对 LLM 编码助手有直接约束力，避免空泛口号。

## 输出格式
- 只输出完整的 AGENTS.md Markdown 正文，不要包裹代码围栏。
- 必须包含：项目栈说明、目录约定、测试要求、安全/权限相关约束、代码风格要点。
- 使用中文撰写说明正文；路径、命令、标识符保持英文原文。
- 控制在 80–120 行以内，条目清晰，可直接提交到仓库根目录。

## 边界
- 不要编造项目中不存在的工具或目录；不确定处用「待确认」标注。
- 不要复制过长的许可证或无关模板段落。`)
}

// InitTemplate 返回 /init 在无 LLM 时的静态 AGENTS.md 模板。
func InitTemplate() string {
	return strings.TrimSpace(`# AGENTS.md

本文件为 golem / LLM 编码助手在本仓库中的行为准则。

## 项目栈
- 语言与运行时：（根据仓库填写）
- 依赖管理：（go mod / npm / 等）
- 测试：运行 ` + "`go test ./...`" + ` 或项目等价命令

## 目录约定
- 业务代码：（填写主要目录）
- 配置：（填写 .golem/ 或项目配置路径）
- 测试：单元测试与源码同包，不设独立 test/ 总目录

## 编码原则
1. 先理解再改动；最小 diff，不做无关重构。
2. 匹配现有命名、错误处理与注释风格。
3. 导出函数与复杂内部函数写中文 godoc 注释。

## 测试要求
- 功能改动须同步补充或更新测试。
- bug 修复先写复现测试再改代码。

## 安全与权限
- 文件工具不得访问 project_root 外路径。
- bash 受 rules 与 approval 模式约束；destructive 命令需谨慎。

## 待确认
- （列出需要维护者补充的项目特定约定）
`)
}
