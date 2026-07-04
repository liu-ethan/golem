package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readFileTool(projectRoot string) Tool {
	return Tool{
		Name:        "read_file",
		Description: "Read the contents of a file. Path must be inside project_root.",
		InputSchema: objectSchema(map[string]any{
			"path": stringProperty("Relative path to the file inside project_root"),
		}, "path"),
		Execute: func(ctx context.Context, input map[string]any) (string, error) {
			path, err := requiredString(input, "path")
			if err != nil {
				return "", err
			}
			return readFile(projectRoot, path)
		},
	}
}

func writeFileTool(projectRoot string) Tool {
	return Tool{
		Name:        "write_file",
		Description: "Write content to a file, creating parent directories if needed. Path must be inside project_root.",
		InputSchema: objectSchema(map[string]any{
			"path":    stringProperty("Relative path to the file inside project_root"),
			"content": stringProperty("Full file content to write"),
		}, "path", "content"),
		Execute: func(ctx context.Context, input map[string]any) (string, error) {
			path, err := requiredString(input, "path")
			if err != nil {
				return "", err
			}
			content, err := requiredString(input, "content")
			if err != nil {
				return "", err
			}
			return writeFileInProject(projectRoot, path, content)
		},
	}
}

func editFileTool(projectRoot string) Tool {
	return Tool{
		Name:        "edit_file",
		Description: "Replace the first occurrence of old_string with new_string in a file. Path must be inside project_root.",
		InputSchema: objectSchema(map[string]any{
			"path":        stringProperty("Relative path to the file inside project_root"),
			"old_string":  stringProperty("Exact substring to replace once"),
			"new_string":  stringProperty("Replacement text"),
		}, "path", "old_string", "new_string"),
		Execute: func(ctx context.Context, input map[string]any) (string, error) {
			path, err := requiredString(input, "path")
			if err != nil {
				return "", err
			}
			oldString, err := requiredString(input, "old_string")
			if err != nil {
				return "", err
			}
			newString, err := optionalString(input, "new_string")
			if err != nil {
				return "", err
			}
			return editFile(projectRoot, path, oldString, newString)
		},
	}
}

// readFile 读取 projectRoot 内相对路径文件的全部内容。
func readFile(projectRoot, path string) (string, error) {
	abs, err := ValidatePath(projectRoot, path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeFileInProject 写入 projectRoot 内相对路径文件，必要时创建父目录。
func writeFileInProject(projectRoot, path, content string) (string, error) {
	abs, err := ValidatePath(projectRoot, path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(content), path), nil
}

// editFile 在 projectRoot 内文件中将 oldString 首次替换为 newString 并写回。
func editFile(projectRoot, path, oldString, newString string) (string, error) {
	abs, err := ValidatePath(projectRoot, path)
	if err != nil {
		return "", err
	}
	original, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	content := string(original)
	if !strings.Contains(content, oldString) {
		return "", fmt.Errorf("old_string not found in %s", path)
	}
	updated := strings.Replace(content, oldString, newString, 1)
	if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("edited %s", path), nil
}
