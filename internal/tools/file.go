package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	readFileMaxReturnBytes = 100 << 10 // 单次最多返回 100KB 文本
	readFileMaxFileBytes   = 10 << 20  // 超过 10MB 的文件拒绝读取
	readFileProbeBytes     = 8192      // 二进制检测采样大小
)

func readFileTool(projectRoot string) Tool {
	return Tool{
		Name:        "read_file",
		Description: "Read the contents of a text file (returns up to 100KB). Binary files and files over 10MB are rejected. Path must be inside project_root.",
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

// readFile 读取 projectRoot 内相对路径的文本文件，拒绝二进制与过大文件。
func readFile(projectRoot, path string) (string, error) {
	abs, err := ValidatePath(projectRoot, path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory; use list_dir instead", path)
	}
	if info.Size() > readFileMaxFileBytes {
		return "", fmt.Errorf("file too large (%d bytes); read_file supports files up to %d bytes", info.Size(), readFileMaxFileBytes)
	}

	f, err := os.Open(abs)
	if err != nil {
		return "", err
	}
	defer f.Close()

	probe := make([]byte, readFileProbeBytes)
	n, err := io.ReadFull(f, probe)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	if isBinaryContent(probe[:n]) {
		return "", fmt.Errorf("binary file (%d bytes); read_file only supports text files", info.Size())
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(f, readFileMaxReturnBytes+1))
	if err != nil {
		return "", err
	}

	truncated := info.Size() > int64(len(data)) || len(data) > readFileMaxReturnBytes
	if len(data) > readFileMaxReturnBytes {
		data = data[:readFileMaxReturnBytes]
	}
	out := string(data)
	if truncated {
		out += fmt.Sprintf("\n\n(truncated: showing first %d of %d bytes)", len(data), info.Size())
	}
	return out, nil
}

func isBinaryContent(sample []byte) bool {
	if len(sample) == 0 {
		return false
	}
	return bytes.IndexByte(sample, 0) >= 0
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
