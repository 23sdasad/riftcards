// Package testfixtures 读取 ai-docs/contracts/protocol-fixtures 下的共享协议样例。
//
// 只允许测试代码导入：它不是运行时依赖，也不包含规则或状态。
// 路径解析使用 pathutil 并显式传入宿主系统（见 ai-docs/architecture/paths-and-boundaries.md），
// 不手写分隔符，因此三平台行为一致。
package testfixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mjq/riftcards/server/internal/platform/pathutil"
)

// Dir 是仓库内共享协议样例目录，相对仓库根。
const Dir = "ai-docs/contracts/protocol-fixtures"

// Sample 是一条带名字的协议样例。
type Sample struct {
	Name    string          `json:"name"`
	Message json.RawMessage `json:"message"`
}

// File 是样例文件的结构。
type File struct {
	Version  string   `json:"version"`
	Messages []Sample `json:"messages"`
}

// HostOS 返回宿主系统对应的 pathutil 目标系统。
func HostOS() pathutil.OS {
	switch runtime.GOOS {
	case "windows":
		return pathutil.Windows
	case "darwin":
		return pathutil.MacOS
	default:
		return pathutil.Linux
	}
}

// RepoRoot 从本文件位置向上查找包含 Taskfile.yml 的目录，不依赖当前工作目录。
func RepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("无法定位 testfixtures 源码位置")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "Taskfile.yml")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("未找到仓库根（缺少 Taskfile.yml）")
		}
		dir = parent
	}
}

// Load 读取并校验一个样例文件。name 是 Dir 下的文件名。
func Load(name string) (File, error) {
	root, err := RepoRoot()
	if err != nil {
		return File{}, err
	}
	path, err := pathutil.Join(HostOS(), root, Dir, name)
	if err != nil {
		return File{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}

	var file File
	if err := json.Unmarshal(content, &file); err != nil {
		return File{}, fmt.Errorf("解析 %s 失败：%w", name, err)
	}
	if len(file.Messages) == 0 {
		return File{}, fmt.Errorf("%s 不含样例", name)
	}
	for _, sample := range file.Messages {
		if strings.TrimSpace(sample.Name) == "" || len(sample.Message) == 0 {
			return File{}, fmt.Errorf("%s 存在缺少 name 或 message 的样例", name)
		}
	}
	return file, nil
}
