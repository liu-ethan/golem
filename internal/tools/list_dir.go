package tools

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

func listDirTool(projectRoot string) Tool {
	return Tool{
		Name:        "list_dir",
		Description: "List files and directories at a path inside project_root.",
		InputSchema: objectSchema(map[string]any{
			"path": stringProperty("Relative directory path inside project_root; defaults to ."),
		}),
		Execute: func(ctx context.Context, input map[string]any) (string, error) {
			path, err := optionalString(input, "path")
			if err != nil {
				return "", err
			}
			if path == "" {
				path = "."
			}
			return listDir(projectRoot, path)
		},
	}
}

// listDir 列出 projectRoot 内相对路径目录下的条目名称（含目录后缀 /）。
func listDir(projectRoot, path string) (string, error) {
	abs, err := ValidatePath(projectRoot, path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", path)
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += string(os.PathSeparator)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "(empty directory)", nil
	}
	return strings.Join(names, "\n"), nil
}
