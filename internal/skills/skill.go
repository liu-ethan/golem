package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill 描述一个可加载的专家人格配置。
type Skill struct {
	Name         string
	Version      string
	SystemPrompt string
	AllowedTools []string
	DeniedTools  []string
	Rules        []string
	Source       string
	Dir          string
}

// Loader 从项目级、全局与内置来源加载 Skill。
type Loader struct {
	projectRoot string
}

// NewLoader 创建绑定 projectRoot 的 Skill 加载器。
func NewLoader(projectRoot string) *Loader {
	return &Loader{projectRoot: projectRoot}
}

// ScanPaths 返回 Skill 扫描来源说明（builtin、全局、项目）。
func ScanPaths(projectRoot string) []string {
	home, err := os.UserHomeDir()
	global := "~/.golem/skills"
	if err == nil {
		global = filepath.Join(home, ".golem", "skills")
	}
	return []string{
		"builtin",
		global,
		filepath.Join(projectRoot, ".golem", "skills"),
	}
}

// LoadByName 按名称加载 Skill，名称匹配不区分大小写。
func (l *Loader) LoadByName(name string) (Skill, error) {
	list, err := l.List()
	if err != nil {
		return Skill{}, err
	}
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, s := range list {
		if strings.ToLower(s.Name) == lower {
			return s, nil
		}
	}
	return Skill{}, fmt.Errorf("skill not found: %s", name)
}

// List 返回全部可用 Skill，按名称排序；同名时项目级覆盖全局。
func (l *Loader) List() ([]Skill, error) {
	byName := map[string]Skill{}

	for _, s := range builtinSkills() {
		byName[s.Name] = s
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	globalDir := filepath.Join(home, ".golem", "skills")
	if err := l.loadDir(globalDir, "global", byName, false); err != nil {
		return nil, err
	}

	projectDir := filepath.Join(l.projectRoot, ".golem", "skills")
	if err := l.loadDir(projectDir, "project", byName, true); err != nil {
		return nil, err
	}

	out := make([]Skill, 0, len(byName))
	for _, s := range byName {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Load 按名称加载 Skill；未找到时返回 error。
func (l *Loader) Load(name string) (Skill, error) {
	skills, err := l.List()
	if err != nil {
		return Skill{}, err
	}
	for _, s := range skills {
		if s.Name == name {
			return s, nil
		}
	}
	return Skill{}, fmt.Errorf("skill not found: %s", name)
}

func (l *Loader) loadDir(dir, source string, byName map[string]Skill, override bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read skills dir %s: %w", dir, err)
	}
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		skillDir := filepath.Join(dir, ent.Name())
		s, err := parseSkillDir(skillDir, source)
		if err != nil {
			continue
		}
		if s.Name == "" {
			s.Name = ent.Name()
		}
		if existing, ok := byName[s.Name]; ok && !override && existing.Source == "project" {
			continue
		}
		byName[s.Name] = s
	}
	return nil
}

func parseSkillDir(dir, source string) (Skill, error) {
	jsonPath := filepath.Join(dir, "skill.json")
	if data, err := os.ReadFile(jsonPath); err == nil {
		var raw skillJSON
		if err := json.Unmarshal(data, &raw); err != nil {
			return Skill{}, err
		}
		return Skill{
			Name:         firstNonEmpty(raw.Name, filepath.Base(dir)),
			Version:      raw.Version,
			SystemPrompt: strings.TrimSpace(raw.SystemPrompt),
			AllowedTools: raw.ToolPermissions,
			DeniedTools:  raw.DeniedTools,
			Rules:        raw.Rules,
			Source:       source,
			Dir:          dir,
		}, nil
	}

	mdPath := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return Skill{}, err
	}
	return parseSkillMarkdown(string(data), source, dir)
}

type skillJSON struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	SystemPrompt    string   `json:"system_prompt"`
	ToolPermissions []string `json:"tool_permissions"`
	DeniedTools     []string `json:"denied_tools"`
	Rules           []string `json:"rules"`
}

func parseSkillMarkdown(content, source, dir string) (Skill, error) {
	lines := strings.Split(content, "\n")
	s := Skill{Source: source, Dir: dir}
	section := "prompt"
	var promptLines []string

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "# ") && s.Name == "" {
			s.Name = strings.TrimSpace(strings.TrimPrefix(trim, "# "))
			continue
		}
		switch strings.ToLower(trim) {
		case "## 工具权限", "## tools", "## tool permissions":
			section = "tools"
			continue
		case "## 规则覆盖", "## rules", "## rule overrides":
			section = "rules"
			continue
		}
		if strings.HasPrefix(trim, "## ") {
			section = "prompt"
		}

		switch section {
		case "prompt":
			if trim != "" || len(promptLines) > 0 {
				promptLines = append(promptLines, line)
			}
		case "tools":
			lower := strings.ToLower(trim)
			if strings.HasPrefix(lower, "allowed:") {
				s.AllowedTools = splitCSV(trim[strings.Index(trim, ":")+1:])
			}
			if strings.HasPrefix(lower, "denied:") {
				s.DeniedTools = splitCSV(trim[strings.Index(trim, ":")+1:])
			}
		case "rules":
			if trim == "" {
				continue
			}
			s.Rules = append(s.Rules, trim)
		}
	}
	s.SystemPrompt = strings.TrimSpace(strings.Join(promptLines, "\n"))
	if s.Name == "" {
		s.Name = filepath.Base(dir)
	}
	return s, nil
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ToolAllowed 判断工具是否被 Skill 允许；无限制时返回 true。
func (s Skill) ToolAllowed(name string) bool {
	for _, denied := range s.DeniedTools {
		if denied == name {
			return false
		}
	}
	if len(s.AllowedTools) == 0 {
		return true
	}
	for _, allowed := range s.AllowedTools {
		if allowed == name {
			return true
		}
	}
	return false
}

// Summary 返回 Skill 的一行摘要，供目录展示与 BM25 检索。
func (s Skill) Summary() string {
	for _, line := range strings.Split(s.SystemPrompt, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-•* "))
		if line != "" {
			if len([]rune(line)) > 120 {
				return string([]rune(line)[:119]) + "…"
			}
			return line
		}
	}
	return s.Name
}

// SearchText 返回用于 BM25 语义匹配的检索文本。
func (s Skill) SearchText() string {
	return s.Name + " " + strings.TrimSpace(s.SystemPrompt)
}

// PromptOverlay 返回应追加到 base system prompt 的 Skill 文本块。
func (s Skill) PromptOverlay() string {
	if strings.TrimSpace(s.SystemPrompt) == "" {
		return ""
	}
	return "\n\n## Skill: " + s.Name + "\n" + strings.TrimSpace(s.SystemPrompt)
}
