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

// toolchainPaths 汇总所有依赖的绝对路径。它由 resolvePaths 得到，供映射文件、
// 引导结束时的报告和 check 命令共用，保证三处看到的是同一份结果。
type toolchainPaths struct {
	ToolsBin      string // 引导器的 bin 目录（.tools/bin）
	Task          string // task 可执行文件（装在 GOPATH/bin）
	GolangciLint  string
	Govulncheck   string
	Dotnet        string // .NET SDK 的 dotnet 可执行文件
	DotnetRoot    string // .NET SDK 根目录，映射为 DOTNET_ROOT
	Godot         string // Godot 的 shim（.tools/bin 下）
	Clang         string // 用于报告版本的 clang
	ClangCl       string // Windows 上的 clang-cl
	Cc            string // 映射为 CC
	Cxx           string // 映射为 CXX
	LlvmMingwRoot string // 包含 lib/clang/<major> 的 LLVM-MinGW 根目录
	LlvmClangCl   string // 映射为 LLVM_CLANG_CL（与 ClangCl 相同）
}

// resolvePaths 按“仓库内安装优先、系统工具兜底”的顺序定位每个依赖。
func resolvePaths(cfg *Config) toolchainPaths {
	paths := toolchainPaths{ToolsBin: cfg.BinDir}

	gopathBin, _ := goPathBin()
	paths.Task = firstExisting(
		filepath.Join(gopathBin, "task"+exeSuffix()),
		filepath.Join(cfg.BinDir, "task"+exeSuffix()),
		which("task"),
	)
	paths.GolangciLint = firstExisting(filepath.Join(cfg.BinDir, "golangci-lint"+exeSuffix()), which("golangci-lint"))
	paths.Govulncheck = firstExisting(filepath.Join(cfg.BinDir, "govulncheck"+exeSuffix()), which("govulncheck"))

	dotnetDir := filepath.Join(cfg.ToolsDir, "dotnet")
	if exe, err := findDotnetExe(dotnetDir); err == nil {
		paths.Dotnet = exe
		paths.DotnetRoot = dotnetDir
	} else {
		paths.Dotnet = which("dotnet")
	}

	paths.Godot = firstExisting(filepath.Join(cfg.BinDir, "godot.cmd"), filepath.Join(cfg.BinDir, "godot"), which("godot"))

	llvmDir := filepath.Join(cfg.ToolsDir, "llvm")
	if exe, err := findLlvmClang(llvmDir); err == nil {
		paths.Clang = exe
	} else if runtime.GOOS == "windows" {
		// 兜底：引导器未装 LLVM 时，仍可能已有系统 LLVM（clang-cl.cmd 的默认位置）。
		paths.Clang = firstExisting(filepath.Join(windowsLLVMDir, "clang.exe"), which("clang"))
	} else {
		paths.Clang = which("clang")
	}
	if exe, err := findLlvmClangCl(llvmDir); err == nil {
		paths.ClangCl = exe
	} else if runtime.GOOS == "windows" {
		paths.ClangCl = firstExisting(filepath.Join(windowsLLVMDir, "clang-cl.exe"))
	}
	if root, err := findMingwRoot(filepath.Join(cfg.ToolsDir, "llvm-mingw")); err == nil {
		paths.LlvmMingwRoot = root
	}
	paths.LlvmClangCl = paths.ClangCl

	paths.Cc, paths.Cxx = compilerPaths(cfg, paths)
	return paths
}

// compilerPaths 决定 CC 与 CXX。
//
// Windows：只有 clang-cl 与 LLVM-MinGW 都就绪时才指向仓库内的包装器 tools/llvm/clang-cl.cmd，
// 避免给出一个必然失败的路径。其他平台：使用系统 clang / clang++。
func compilerPaths(cfg *Config, paths toolchainPaths) (cc, cxx string) {
	if runtime.GOOS == "windows" {
		wrapper := filepath.Join(cfg.RepoRoot, "tools", "llvm", "clang-cl.cmd")
		if _, err := os.Stat(wrapper); err == nil && paths.LlvmMingwRoot != "" && paths.ClangCl != "" {
			return wrapper, wrapper
		}
		return "", ""
	}
	return paths.Clang, which("clang++")
}

// envEntries 生成映射文件的全部条目，键名统一以 TC_ 开头。
//
// 这些键不会直接覆盖开发者的环境变量：Taskfile 只用它们来设置 CC、CXX、DOTNET_ROOT 等标准变量，
// 并且只在值非空时生效。TC_PATH 是拼接结果（引导器 bin、.NET 根、GOPATH/bin，再接原 PATH），
// 仅供参考，Taskfile 当前并不把它写回 PATH。
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

// quoteEnvValue 决定一个值是否需要用单引号包裹。
//
// 规则：值含 # 或首尾有空白时加单引号（dotenv 会把 # 当作注释，并去掉首尾空白）。
// 单引号内不做转义处理，因此反斜杠（Windows 路径）与空格可以原样保留。
// 不使用双引号：dotenv 会展开双引号内的 \n、\t 等转义，会破坏 C:\new\tools 这类路径。
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
