package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type envEntry struct {
	Key   string
	Value string
}

// toolchainPaths 汇总所有依赖的绝对路径，供环境文件与检查命令共用。
type toolchainPaths struct {
	ToolsBin      string
	Task          string
	GolangciLint  string
	Govulncheck   string
	Dotnet        string
	DotnetRoot    string
	Godot         string
	Clang         string
	ClangCl       string
	Cc            string
	Cxx           string
	LlvmMingwRoot string
	LlvmClangCl   string
}

func resolvePaths(cfg *Config) toolchainPaths {
	paths := toolchainPaths{ToolsBin: cfg.BinDir}

	gopathBin, _ := goPathBin()
	paths.Task = firstExisting(
		filepath.Join(gopathBin, "task"+exeSuffix()),
		filepath.Join(cfg.BinDir, "task"+exeSuffix()),
		which("task"),
	)
	paths.GolangciLint = firstExisting(
		filepath.Join(cfg.BinDir, "golangci-lint"+exeSuffix()),
		which("golangci-lint"),
	)
	paths.Govulncheck = firstExisting(
		filepath.Join(cfg.BinDir, "govulncheck"+exeSuffix()),
		which("govulncheck"),
	)

	dotnetDir := filepath.Join(cfg.ToolsDir, "dotnet")
	if exe, err := findDotnetExe(dotnetDir); err == nil {
		paths.Dotnet = exe
		paths.DotnetRoot = dotnetDir
	} else {
		paths.Dotnet = which("dotnet")
	}

	paths.Godot = firstExisting(
		filepath.Join(cfg.BinDir, "godot.cmd"),
		filepath.Join(cfg.BinDir, "godot"),
		which("godot"),
	)

	llvmDir := filepath.Join(cfg.ToolsDir, "llvm")
	if exe, err := findLlvmClang(llvmDir); err == nil {
		paths.Clang = exe
	} else if runtime.GOOS == "windows" {
		// 兜底：引导器未装 LLVM 时，仍可能已有系统 LLVM（clang-cl.cmd 的默认路径）。
		paths.Clang = firstExisting(`C:\Program Files\LLVM\bin\clang.exe`, which("clang"))
	} else {
		paths.Clang = which("clang")
	}
	if exe, err := findLlvmClangCl(llvmDir); err == nil {
		paths.ClangCl = exe
	} else if runtime.GOOS == "windows" {
		paths.ClangCl = firstExisting(`C:\Program Files\LLVM\bin\clang-cl.exe`)
	}
	if root, err := findMingwRoot(filepath.Join(cfg.ToolsDir, "llvm-mingw")); err == nil {
		paths.LlvmMingwRoot = root
	}
	paths.LlvmClangCl = paths.ClangCl

	wrapper := filepath.Join(cfg.RepoRoot, "tools", "llvm", "clang-cl.cmd")
	if runtime.GOOS == "windows" {
		// 只有确实存在 clang-cl 与 MinGW 时才把 CC/CXX 指向包装器，避免给出坏路径。
		if _, err := os.Stat(wrapper); err == nil && paths.LlvmMingwRoot != "" && paths.ClangCl != "" {
			paths.Cc = wrapper
			paths.Cxx = wrapper
		}
	} else {
		if paths.Clang != "" {
			paths.Cc = paths.Clang
		}
		if cxx := which("clang++"); cxx != "" {
			paths.Cxx = cxx
		}
	}
	return paths
}

// envEntries 生成 Taskfile 读取的映射；键名统一 TC_ 前缀，避免覆盖开发者环境。
func envEntries(cfg *Config, paths toolchainPaths) []envEntry {
	gopathBin, _ := goPathBin()
	entries := []envEntry{
		{Key: "TC_TOOLS_BIN", Value: paths.ToolsBin},
		{Key: "TC_PATH", Value: joinPathList(paths.ToolsBin, paths.DotnetRoot, gopathBin, os.Getenv("PATH"))},
		{Key: "TC_TASK", Value: paths.Task},
		{Key: "TC_GOLANGCI_LINT", Value: paths.GolangciLint},
		{Key: "TC_GOVULNCHECK", Value: paths.Govulncheck},
		{Key: "TC_DOTNET", Value: paths.Dotnet},
		{Key: "TC_DOTNET_ROOT", Value: paths.DotnetRoot},
		{Key: "TC_GODOT", Value: paths.Godot},
		{Key: "TC_CLANG", Value: paths.Clang},
		{Key: "TC_CLANG_CL", Value: paths.ClangCl},
		{Key: "TC_CC", Value: paths.Cc},
		{Key: "TC_CXX", Value: paths.Cxx},
		{Key: "TC_LLVM_MINGW_ROOT", Value: paths.LlvmMingwRoot},
		{Key: "TC_LLVM_CLANG_CL", Value: paths.LlvmClangCl},
		{Key: "TC_LLVM_MAJOR", Value: majorVersion(cfg.LlvmVersion)},
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries
}

func envFilePath(cfg *Config) string {
	return filepath.Join(cfg.ToolsDir, "toolchain.env")
}

// writeEnvFile 写入 Taskfile 的 dotenv 映射。必须是 UTF-8 无 BOM：Task 使用
// 的 dotenv 解析器遇到 BOM 会直接报错。
func writeEnvFile(cfg *Config, paths toolchainPaths) (string, error) {
	if err := os.MkdirAll(cfg.ToolsDir, 0o755); err != nil {
		return "", err
	}
	path := envFilePath(cfg)
	if err := os.WriteFile(path, []byte(renderEnvFile(envEntries(cfg, paths))), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func renderEnvFile(entries []envEntry) string {
	var b strings.Builder
	b.WriteString("# 由 tools/toolchain 生成，请勿手工编辑；重新生成：task bootstrap\n")
	for _, entry := range entries {
		if entry.Value == "" {
			continue
		}
		b.WriteString(entry.Key)
		b.WriteString("=")
		b.WriteString(quoteEnvValue(entry.Value))
		b.WriteString("\n")
	}
	return b.String()
}

// quoteEnvValue 按需加单引号：dotenv 中不加引号即可保留空格和反斜杠，但 # 会被
// 当成注释，因此仅在必要时使用不处理转义的单引号。
func quoteEnvValue(value string) string {
	if strings.ContainsAny(value, "#") || value != strings.TrimSpace(value) {
		return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
	}
	return value
}

func readEnvFile(cfg *Config) ([]envEntry, error) {
	content, err := os.ReadFile(envFilePath(cfg))
	if err != nil {
		return nil, err
	}
	var entries []envEntry
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		entries = append(entries, envEntry{Key: strings.TrimSpace(key), Value: strings.TrimSpace(value)})
	}
	return entries, nil
}
