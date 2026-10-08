package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// taskfileVars 读取仓库根 Taskfile.yml 的顶层 vars: 块，返回 键 -> 值。
//
// 依赖声明只有这一处，而首次引导时 task 命令还不存在，因此引导器必须自己读取。
// 这里只实现了 vars: 块所需的最小 YAML 子集：2 个空格缩进的 `KEY: value`，
// 值两端的引号会被去掉；不处理锚点、多行值或嵌套结构。Taskfile 的 vars: 必须保持这种形式。
func taskfileVars(root string) (map[string]string, error) {
	path := filepath.Join(root, taskfileMarker)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	vars := map[string]string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	inVars := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "vars:") {
			inVars = true
			continue
		}
		if !inVars {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// 顶层键（无缩进）结束 vars 块。
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		vars[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(vars) == 0 {
		return nil, fmt.Errorf("未从 %s 的 vars: 读到依赖声明", taskfileMarker)
	}
	return vars, nil
}

// depValue 返回依赖声明值：环境变量 TC_<KEY> 覆盖优先，其次 Taskfile 的 vars 声明。
// 两者都缺失时把 key 记入 missing，由调用方汇总报错。
func depValue(vars map[string]string, key string, missing *[]string) string {
	if value := strings.TrimSpace(os.Getenv("TC_" + key)); value != "" {
		return value
	}
	if value, ok := vars[key]; ok && value != "" {
		return value
	}
	*missing = append(*missing, key)
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
