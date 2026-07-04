package skills

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var githubRefPattern = regexp.MustCompile(`^github:([^/]+)/([^/]+)/([^/]+)$`)

// InstallGitHub 从 GitHub 仓库安装 Skill 到 ~/.golem/skills/<name>/。
// ref 格式：github:user/repo/skill-name
func InstallGitHub(ref string) (string, error) {
	m := githubRefPattern.FindStringSubmatch(strings.TrimSpace(ref))
	if m == nil {
		return "", fmt.Errorf("invalid skill ref %q (want github:user/repo/skill-name)", ref)
	}
	user, repo, skillName := m[1], m[2], m[3]

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	destDir := filepath.Join(home, ".golem", "skills", skillName)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create skill dir: %w", err)
	}

	baseURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s", user, repo, skillName)
	client := &http.Client{Timeout: 30 * time.Second}

	wrote := false
	for _, file := range []string{"SKILL.md", "skill.json"} {
		url := baseURL + "/" + file
		data, err := fetchURL(client, url)
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(destDir, file), data, 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", file, err)
		}
		wrote = true
	}
	if !wrote {
		return "", fmt.Errorf("skill files not found at %s", baseURL)
	}
	return destDir, nil
}

func fetchURL(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}
