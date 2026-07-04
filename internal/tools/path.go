package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePath 校验并解析相对 project_root 的路径，拒绝逃逸到项目根之外。
// 先将 path 与 projectRoot 拼接再取绝对路径，再用 filepath.Rel 检查是否含 ".." 前缀。
func ValidatePath(projectRoot, path string) (string, error) {
	root := filepath.Clean(projectRoot)
	abs, err := filepath.Abs(filepath.Join(root, path))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path outside project root")
	}
	return abs, nil
}
