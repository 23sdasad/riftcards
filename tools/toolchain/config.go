package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	// taskfileMarker 用于从任意子目录定位仓库根。
	taskfileMarker = "Taskfile.yml"
	// installEntry 是缺失依赖时统一给出的安装入口。
	installEntry = "go -C tools/toolchain run . bootstrap"
	// defaultToolsDir 是引导器安装依赖的仓库内目录（已被 git 忽略）。
	defaultToolsDir = ".tools"
	// windowsLLVMDir 是 Windows 上 LLVM 的系统默认安装位置，仅作兜底探测。
	windowsLLVMDir = `C:\Program Files\LLVM\bin`
)

// Config 是引导器运行所需的全部信息。
//
// 版本、下载地址和校验哈希只在仓库根 Taskfile.yml 的 vars: 中声明，引导器在运行时读取；
// 任何 TC_<KEY> 环境变量都可以临时覆盖同名声明。引导器本身不硬编码版本。
// 安装目录默认为仓库内 .tools/，可用 TC_TOOLS_DIR 改为仓库内其他相对路径。
type Config struct {
	RepoRoot string // 仓库根（含 Taskfile.yml 的目录）
	ToolsDir string // 依赖安装目录，默认 <RepoRoot>/.tools
	BinDir   string // 引导器生成的可执行文件与 shim，位于 ToolsDir/bin
	CacheDir string // 下载的归档缓存，位于 ToolsDir/cache

	TaskVersion         string
	GolangciLintVersion string
	GovulncheckVersion  string

	GoMinVersion     string
	DotnetMinVersion string
	GodotMinVersion  string
	LlvmMinVersion   string

	DotnetVersion string
	DotnetURL     string
	DotnetHash    string

	GodotVersion string
	GodotURL     string
	GodotHash    string

	LlvmVersion string
	LlvmURL     string
	LlvmHash    string

	MingwVersion string
	MingwURL     string
	MingwHash    string
}

// component 描述一个可下载依赖在当前平台上的具体形态。
type component struct {
	Name      string // 组件名，同时用于 --without 跳过
	Version   string // 锁定版本，写入 .version 标记
	URL       string // 下载地址
	Hash      string // 锁定的十六进制哈希，算法由 HashAlgo 指定
	HashAlgo  string // "sha256" 或 "sha512"
	Archive   string // 解压方式："zip"、"targz" 或 "msi"（见 extractArchive）
	TargetDir string // 解压目标目录
}

// missingVarsError 汇总所有缺失的声明，便于一次性修正 Taskfile。
type missingVarsError struct {
	names []string
}

func (e *missingVarsError) Error() string {
	sort.Strings(e.names)
	return fmt.Sprintf("缺少依赖声明（应写在 %s 的 vars:）：%s", taskfileMarker, strings.Join(e.names, "、"))
}

// LoadConfig 合并 Taskfile 声明与当前平台，得到完整配置。
func LoadConfig() (*Config, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	vars, err := taskfileVars(root)
	if err != nil {
		return nil, err
	}

	var missing []string
	cfg := &Config{
		RepoRoot: root,
		ToolsDir: filepath.Join(root, firstNonEmpty(os.Getenv("TC_TOOLS_DIR"), defaultToolsDir)),

		TaskVersion:         depValue(vars, "TASK_VERSION", &missing),
		GolangciLintVersion: depValue(vars, "GOLANGCI_LINT_VERSION", &missing),
		GovulncheckVersion:  depValue(vars, "GOVULNCHECK_VERSION", &missing),

		GoMinVersion:     depValue(vars, "GO_MIN_VERSION", &missing),
		DotnetMinVersion: depValue(vars, "DOTNET_MIN_VERSION", &missing),
		GodotMinVersion:  depValue(vars, "GODOT_MIN_VERSION", &missing),
		LlvmMinVersion:   depValue(vars, "LLVM_MIN_VERSION", &missing),

		DotnetVersion: depValue(vars, "DOTNET_VERSION", &missing),
		GodotVersion:  depValue(vars, "GODOT_VERSION", &missing),
		LlvmVersion:   depValue(vars, "LLVM_VERSION", &missing),
		MingwVersion:  depValue(vars, "LLVM_MINGW_VERSION", &missing),
	}
	cfg.BinDir = filepath.Join(cfg.ToolsDir, "bin")
	cfg.CacheDir = filepath.Join(cfg.ToolsDir, "cache")

	platform, ok := platformAssets[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return nil, fmt.Errorf("不支持的平台 %s/%s：请在 tools/toolchain/config.go 的 platformAssets 中补充", runtime.GOOS, runtime.GOARCH)
	}
	cfg.DotnetURL = dotnetURL(cfg.DotnetVersion, platform.dotnetRID, platform.dotnetExt)
	cfg.DotnetHash = depValue(vars, platform.dotnetHashKey, &missing)
	cfg.GodotURL = godotURL(cfg.GodotVersion, platform.godotAsset)
	cfg.GodotHash = depValue(vars, platform.godotHashKey, &missing)
	if platform.llvmURLKey != "" {
		cfg.LlvmURL = depValue(vars, platform.llvmURLKey, &missing)
		cfg.LlvmHash = depValue(vars, platform.llvmHashKey, &missing)
		cfg.MingwURL = depValue(vars, "LLVM_MINGW_URL", &missing)
		cfg.MingwHash = depValue(vars, "LLVM_MINGW_SHA256", &missing)
	}

	if len(missing) > 0 {
		return nil, &missingVarsError{names: missing}
	}
	return cfg, nil
}

// platformAsset 描述某个 GOOS/GOARCH 下的资产命名与对应的 Taskfile 键名。
// 键名与 Taskfile.yml 的 vars: 一一对应。
type platformAsset struct {
	dotnetRID     string // .NET SDK 的运行时标识，如 win-x64
	dotnetExt     string // .NET SDK 归档扩展名：zip 或 tar.gz
	dotnetHashKey string // SHA512 哈希对应的 Taskfile 键名
	godotAsset    string // Godot 发布资产中的平台段，如 win64
	godotHashKey  string // Godot SHA256 哈希对应的 Taskfile 键名
	llvmURLKey    string // 仅 Windows：LLVM MSI 地址的 Taskfile 键名；其他平台为空
	llvmHashKey   string // 仅 Windows：LLVM MSI SHA256 的 Taskfile 键名
}

// platformAssets 是唯一的平台资产映射表。新增平台只需在这里补一行。
var platformAssets = map[string]platformAsset{
	"windows/amd64": {
		dotnetRID: "win-x64", dotnetExt: "zip", dotnetHashKey: "DOTNET_SHA512_WIN_X64",
		godotAsset: "win64", godotHashKey: "GODOT_SHA256_WIN64",
		llvmURLKey: "LLVM_WINDOWS_MSI_URL", llvmHashKey: "LLVM_WINDOWS_MSI_SHA256",
	},
	"linux/amd64": {
		dotnetRID: "linux-x64", dotnetExt: "tar.gz", dotnetHashKey: "DOTNET_SHA512_LINUX_X64",
		godotAsset: "linux_x86_64", godotHashKey: "GODOT_SHA256_LINUX_X64",
	},
	"linux/arm64": {
		dotnetRID: "linux-arm64", dotnetExt: "tar.gz", dotnetHashKey: "DOTNET_SHA512_LINUX_ARM64",
		godotAsset: "linux_arm64", godotHashKey: "GODOT_SHA256_LINUX_ARM64",
	},
	"darwin/amd64": {
		dotnetRID: "osx-x64", dotnetExt: "tar.gz", dotnetHashKey: "DOTNET_SHA512_OSX_X64",
		godotAsset: "macos.universal", godotHashKey: "GODOT_SHA256_MACOS_UNIVERSAL",
	},
	"darwin/arm64": {
		dotnetRID: "osx-arm64", dotnetExt: "tar.gz", dotnetHashKey: "DOTNET_SHA512_OSX_ARM64",
		godotAsset: "macos.universal", godotHashKey: "GODOT_SHA256_MACOS_UNIVERSAL",
	},
}

func dotnetURL(version, rid, ext string) string {
	return fmt.Sprintf("https://builds.dotnet.microsoft.com/dotnet/Sdk/%s/dotnet-sdk-%s-%s.%s", version, version, rid, ext)
}

func godotURL(version, asset string) string {
	return fmt.Sprintf("https://github.com/godotengine/godot/releases/download/%s-stable/Godot_v%s-stable_mono_%s.zip", version, version, asset)
}

// components 返回当前平台需要安装的依赖。
//
// Windows 额外需要 LLVM 与 LLVM-MinGW：clang-cl 包装器强制 -resource-dir=<MinGW>\lib\clang\<major>，
// 驱动版本必须与之一致。LLVM 使用官方 MSI 的管理安装展开到 .tools/llvm，不注册产品、
// 不写系统目录、不需要管理员权限。
func (c *Config) components() []component {
	platform := platformAssets[runtime.GOOS+"/"+runtime.GOARCH]
	items := []component{
		{
			Name: "dotnet", Version: c.DotnetVersion, URL: c.DotnetURL,
			Hash: c.DotnetHash, HashAlgo: "sha512", Archive: archiveKind(platform.dotnetExt),
			TargetDir: filepath.Join(c.ToolsDir, "dotnet"),
		},
		{
			Name: "godot", Version: c.GodotVersion, URL: c.GodotURL,
			Hash: c.GodotHash, HashAlgo: "sha256", Archive: "zip",
			TargetDir: filepath.Join(c.ToolsDir, "godot"),
		},
	}
	// 仅 Windows 需要 LLVM 工具链（见上方说明）。
	if platform.llvmURLKey != "" {
		items = append(items,
			component{
				Name: "llvm", Version: c.LlvmVersion, URL: c.LlvmURL,
				Hash: c.LlvmHash, HashAlgo: "sha256", Archive: "msi",
				TargetDir: filepath.Join(c.ToolsDir, "llvm"),
			},
			component{
				Name: "llvm-mingw", Version: c.MingwVersion, URL: c.MingwURL,
				Hash: c.MingwHash, HashAlgo: "sha256", Archive: "zip",
				TargetDir: filepath.Join(c.ToolsDir, "llvm-mingw"),
			},
		)
	}
	return items
}

// archiveKind 把 Taskfile 中的扩展名映射为 extractArchive 使用的解压种类。
func archiveKind(ext string) string {
	if ext == "tar.gz" {
		return "targz"
	}
	return ext
}

// repoRoot 从当前目录向上查找 Taskfile.yml；TC_REPO_ROOT 可显式覆盖。
func repoRoot() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("TC_REPO_ROOT")); explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, taskfileMarker)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("未找到仓库根（缺少 Taskfile.yml）；可从仓库内运行或设置 TC_REPO_ROOT")
		}
		dir = parent
	}
}
