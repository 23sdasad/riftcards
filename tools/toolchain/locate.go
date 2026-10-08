package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func goPathBin() (string, error) {
	out, err := runCapture("go", "env", "GOBIN")
	if err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out), nil
	}
	gopath, err := runCapture("go", "env", "GOPATH")
	if err != nil {
		return "", err
	}
	return filepath.Join(strings.TrimSpace(gopath), "bin"), nil
}

// findFile 返回 root 下第一个满足条件的文件路径。
func findFile(root string, match func(path string, info os.FileInfo) bool) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil
		}
		if match(path, info) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("在 %s 下未找到匹配文件", relToRepo(root))
	}
	return found, nil
}

func findDotnetExe(root string) (string, error) {
	candidate := filepath.Join(root, "dotnet"+exeSuffix())
	if _, err := os.Stat(candidate); err != nil {
		return "", fmt.Errorf("缺少 %s", relToRepo(candidate))
	}
	return candidate, nil
}

// findGodotExe 按平台特征定位 Godot 可执行文件，避免绑定具体文件名。
func findGodotExe(root string) (string, error) {
	return findFile(root, func(path string, info os.FileInfo) bool {
		base := strings.ToLower(filepath.Base(path))
		if !strings.Contains(base, "godot") {
			return false
		}
		switch runtime.GOOS {
		case "windows":
			return strings.HasSuffix(base, ".exe") && !strings.Contains(base, "console")
		case "darwin":
			return strings.Contains(filepath.ToSlash(path), "/Contents/MacOS/")
		default:
			if strings.HasSuffix(base, ".dll") || strings.HasSuffix(base, ".so") || strings.Contains(base, "godotsharp") {
				return false
			}
			return info.Mode().Perm()&0o111 != 0
		}
	})
}

func findLlvmClangCl(root string) (string, error) {
	return findFile(root, func(path string, _ os.FileInfo) bool {
		return strings.EqualFold(filepath.Base(path), "clang-cl.exe")
	})
}

func findLlvmClang(root string) (string, error) {
	return findFile(root, func(path string, _ os.FileInfo) bool {
		return strings.EqualFold(filepath.Base(path), "clang"+exeSuffix())
	})
}

// findMingwRoot 返回包含 lib/clang/<major> 的 LLVM-MinGW 根目录。
func findMingwRoot(root string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() || !strings.EqualFold(entry.Name(), "clang") {
			return nil
		}
		libDir := filepath.Dir(path)
		if !strings.EqualFold(filepath.Base(libDir), "lib") {
			return nil
		}
		found = filepath.Dir(libDir)
		return filepath.SkipAll
	})
	if err != nil && err != filepath.SkipAll {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("在 %s 下未找到 lib/clang", relToRepo(root))
	}
	return found, nil
}

// writeShim 在 .tools/bin 生成跨平台转发脚本，让工具名与路径保持稳定。
func writeShim(cfg *Config, name, target string) (string, error) {
	if err := os.MkdirAll(cfg.BinDir, 0o755); err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		path := filepath.Join(cfg.BinDir, name+".cmd")
		content := "@echo off\r\nrem 由 tools/toolchain 生成。\r\n\"" + target + "\" %*\r\n"
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			return "", err
		}
		return path, nil
	}
	path := filepath.Join(cfg.BinDir, name)
	content := "#!/bin/sh\n# 由 tools/toolchain 生成。\nexec \"" + target + "\" \"$@\"\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func which(name string) string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name+exeSuffix())
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func firstExisting(paths ...string) string {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func joinPathList(parts ...string) string {
	var kept []string
	seen := map[string]bool{}
	for _, part := range parts {
		for _, item := range filepath.SplitList(part) {
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			kept = append(kept, item)
		}
	}
	return strings.Join(kept, string(filepath.ListSeparator))
}
