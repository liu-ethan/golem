package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const grepResultLimit = 200

func grepTool(projectRoot string) Tool {
	return Tool{
		Name:        "grep",
		Description: "Recursively search for a regex pattern under project_root (or a subdirectory). Skips .git/. Returns up to 200 matches.",
		InputSchema: objectSchema(map[string]any{
			"pattern": stringProperty("Regular expression pattern to search for"),
			"path":    stringProperty("Optional relative directory to search; defaults to project_root"),
		}, "pattern"),
		Execute: func(ctx context.Context, input map[string]any) (string, error) {
			pattern, err := requiredString(input, "pattern")
			if err != nil {
				return "", err
			}
			searchPath, err := optionalString(input, "path")
			if err != nil {
				return "", err
			}
			return grep(projectRoot, searchPath, pattern, grepResultLimit)
		},
	}
}

// grep 在 projectRoot 内递归搜索正则 pattern，跳过 .git/，最多返回 limit 条匹配。
func grep(projectRoot, searchPath, pattern string, limit int) (string, error) {
	if searchPath == "" {
		searchPath = "."
	}
	rootAbs, err := ValidatePath(projectRoot, searchPath)
	if err != nil {
		return "", err
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}

	var matches []string
	truncated := false

	err = filepath.WalkDir(rootAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if len(matches) >= limit {
			truncated = true
			return filepath.SkipAll
		}

		fileMatches, err := grepFile(projectRoot, path, re, limit-len(matches))
		if err != nil {
			return nil
		}
		matches = append(matches, fileMatches...)
		if len(matches) >= limit {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return "", err
	}

	if len(matches) == 0 {
		return "no matches found", nil
	}
	result := strings.Join(matches, "\n")
	if truncated {
		result += fmt.Sprintf("\n\n(truncated at %d matches)", limit)
	}
	return result, nil
}

func grepFile(projectRoot, absPath string, re *regexp.Regexp, remaining int) ([]string, error) {
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return nil, err
	}
	if info.Size() > 1<<20 {
		return nil, nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	if strings.IndexByte(string(data), 0) >= 0 {
		return nil, nil
	}

	rel, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		rel = absPath
	}

	var matches []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, fmt.Sprintf("%s:%d:%s", rel, lineNum, line))
			if len(matches) >= remaining {
				break
			}
		}
	}
	return matches, scanner.Err()
}
