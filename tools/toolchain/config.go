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
)

// Config 是引导器运行所需的全部信息。版本、URL 和哈希只在仓库根 Taskfile.yml
// 的 vars: 声明（可用 TC_* 覆盖），引导器不硬编码任何版本。
type Config struct {
	RepoRoot string
	ToolsDir string
	BinDir   string
	CacheDir string

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
	Name      string
	Version   string
	URL       string
	Hash      string // 十六进制；算法由 HashAlgo 指定
	HashAlgo  string // "sha256" 或 "sha512"
	Archive   string // "zip"、"targz" 或 "tarxz"
	TargetDir string
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
	toolsDir := firstNonEmpty(os.Getenv("TC_TOOLS_DIR"), ".tools")
	cfg := &Config{
		RepoRoot: root,
		ToolsDir: filepath.Join(root, toolsDir),

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

	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		cfg.DotnetURL = dotnetURL(cfg.DotnetVersion, "win-x64", "zip")
		cfg.DotnetHash = depValue(vars, "DOTNET_SHA512_WIN_X64", &missing)
		cfg.GodotURL = godotURL(cfg.GodotVersion, "win64")
		cfg.GodotHash = depValue(vars, "GODOT_SHA256_WIN64", &missing)
		cfg.LlvmURL = depValue(vars, "LLVM_WINDOWS_MSI_URL", &missing)
		cfg.LlvmHash = depValue(vars, "LLVM_WINDOWS_MSI_SHA256", &missing)
		cfg.MingwURL = depValue(vars, "LLVM_MINGW_URL", &missing)
		cfg.MingwHash = depValue(vars, "LLVM_MINGW_SHA256", &missing)
	case "linux/amd64":
		cfg.DotnetURL = dotnetURL(cfg.DotnetVersion, "linux-x64", "tar.gz")
		cfg.DotnetHash = depValue(vars, "DOTNET_SHA512_LINUX_X64", &missing)
		cfg.GodotURL = godotURL(cfg.GodotVersion, "linux_x86_64")
		cfg.GodotHash = depValue(vars, "GODOT_SHA256_LINUX_X64", &missing)
	case "linux/arm64":
		cfg.DotnetURL = dotnetURL(cfg.DotnetVersion, "linux-arm64", "tar.gz")
		cfg.DotnetHash = depValue(vars, "DOTNET_SHA512_LINUX_ARM64", &missing)
		cfg.GodotURL = godotURL(cfg.GodotVersion, "linux_arm64")
		cfg.GodotHash = depValue(vars, "GODOT_SHA256_LINUX_ARM64", &missing)
	case "darwin/amd64":
		cfg.DotnetURL = dotnetURL(cfg.DotnetVersion, "osx-x64", "tar.gz")
		cfg.DotnetHash = depValue(vars, "DOTNET_SHA512_OSX_X64", &missing)
		cfg.GodotURL = godotURL(cfg.GodotVersion, "macos.universal")
		cfg.GodotHash = depValue(vars, "GODOT_SHA256_MACOS_UNIVERSAL", &missing)
	case "darwin/arm64":
		cfg.DotnetURL = dotnetURL(cfg.DotnetVersion, "osx-arm64", "tar.gz")
		cfg.DotnetHash = depValue(vars, "DOTNET_SHA512_OSX_ARM64", &missing)
		cfg.GodotURL = godotURL(cfg.GodotVersion, "macos.universal")
		cfg.GodotHash = depValue(vars, "GODOT_SHA256_MACOS_UNIVERSAL", &missing)
	default:
		return nil, fmt.Errorf("不支持的平台 %s/%s：请在 Taskfile 中补充资产映射", runtime.GOOS, runtime.GOARCH)
	}

	if len(missing) > 0 {
		return nil, &missingVarsError{names: missing}
	}
	return cfg, nil
}

func dotnetURL(version, rid, ext string) string {
	return fmt.Sprintf("https://builds.dotnet.microsoft.com/dotnet/Sdk/%s/dotnet-sdk-%s-%s.%s", version, version, rid, ext)
}

func godotURL(version, asset string) string {
	return fmt.Sprintf("https://github.com/godotengine/godot/releases/download/%s-stable/Godot_v%s-stable_mono_%s.zip", version, version, asset)
}

// components 返回当前平台应下载的依赖。Windows 额外需要固定 LLVM 与 LLVM-MinGW：
// clang-cl 包装器强制 -resource-dir=<MinGW>\lib\clang\<major>，驱动版本必须一致。
// LLVM 用官方 MSI 的管理安装展开到 .tools/llvm：不注册产品、不写系统目录、不需要管理员。
func (c *Config) components() []component {
	dotnetArchive := "targz"
	if runtime.GOOS == "windows" {
		dotnetArchive = "zip"
	}
	items := []component{
		{
			Name: "dotnet", Version: c.DotnetVersion, URL: c.DotnetURL,
			Hash: c.DotnetHash, HashAlgo: "sha512", Archive: dotnetArchive,
			TargetDir: filepath.Join(c.ToolsDir, "dotnet"),
		},
		{
			Name: "godot", Version: c.GodotVersion, URL: c.GodotURL,
			Hash: c.GodotHash, HashAlgo: "sha256", Archive: "zip",
			TargetDir: filepath.Join(c.ToolsDir, "godot"),
		},
	}
	if runtime.GOOS == "windows" {
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
