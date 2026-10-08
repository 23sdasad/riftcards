package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// goToolsComponent 是 --without 可用于跳过 Go 工具安装的名字。
const goToolsComponent = "gotools"

// bootstrapOptions 是 bootstrap 子命令的参数。
type bootstrapOptions struct {
	Skip  map[string]bool // 要跳过的组件名（见 components 与 goToolsComponent）
	Force bool            // 忽略已就绪的标记，全部重装
}

// parseBootstrapArgs 解析 bootstrap 的参数。
// 支持 --force、--without a,b（或 --without=a,b）。
func parseBootstrapArgs(args []string) (bootstrapOptions, error) {
	opts := bootstrapOptions{Skip: map[string]bool{}}
	addSkips := func(list string) {
		for _, name := range strings.Split(list, ",") {
			if name = strings.TrimSpace(name); name != "" {
				opts.Skip[name] = true
			}
		}
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--force":
			opts.Force = true
		case arg == "--without" && i+1 < len(args):
			i++
			addSkips(args[i])
		case strings.HasPrefix(arg, "--without="):
			addSkips(strings.TrimPrefix(arg, "--without="))
		default:
			return opts, fmt.Errorf("未知参数 %q", arg)
		}
	}
	return opts, nil
}

// runBootstrap 按顺序完成引导：Go 工具 -> 各组件 -> NuGet 还原 -> 写映射文件 -> 报告。
// 任一步失败立即返回，已完成的步骤保留，重跑时会跳过已就绪的部分。
func runBootstrap(opts bootstrapOptions) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.BinDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		return err
	}
	cleanPartialDownloads(cfg.CacheDir)

	fmt.Printf("引导 riftcards 依赖：%s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("安装目录：%s\n\n", relToRepo(cfg.ToolsDir))

	if !opts.Skip[goToolsComponent] {
		if err := installGoTools(cfg, opts.Force); err != nil {
			return err
		}
	}

	for _, comp := range cfg.components() {
		if opts.Skip[comp.Name] {
			fmt.Printf("-- 跳过 %s（--without）\n", comp.Name)
			continue
		}
		if err := ensureComponent(cfg, comp, opts.Force); err != nil {
			return err
		}
	}

	if !opts.Skip["dotnet"] {
		if err := restoreNuGet(cfg); err != nil {
			return err
		}
	}

	paths := resolvePaths(cfg)
	envPath, err := writeEnvFile(cfg, paths)
	if err != nil {
		return err
	}
	reportLLVMTools(cfg)
	reportPaths(paths, envPath)
	return nil
}

// llvmToolsOfInterest 是本项目关心的 LLVM 工具清单，用于确认 MSI 管理安装实际产出了什么。
var llvmToolsOfInterest = []string{
	"clang", "clang-cl", "clang++", "lld", "lld-link", "llvm-ar", "llvm-rc",
	"clang-format", "clang-tidy", "clangd",
}

// reportLLVMTools 在 .tools/llvm 中定位 clang-cl 所在目录，逐个检查 llvmToolsOfInterest 是否存在。
// 结果来自磁盘上的实际文件，不做假设；找不到 clang-cl 时静默返回（非 Windows 或未安装 LLVM）。
func reportLLVMTools(cfg *Config) {
	clangCl, err := findLlvmClangCl(filepath.Join(cfg.ToolsDir, "llvm"))
	if err != nil {
		return
	}
	bin := filepath.Dir(clangCl)
	fmt.Printf("LLVM 工具（%s）\n", relToRepo(bin))
	for _, tool := range llvmToolsOfInterest {
		name := tool + exeSuffix()
		state := "缺失"
		if info, err := os.Stat(filepath.Join(bin, name)); err == nil && !info.IsDir() {
			state = "存在"
		}
		fmt.Printf("  %-12s %s\n", tool, state)
	}
	fmt.Println()
}

// cleanPartialDownloads 删除缓存目录中上次被中断留下的 *.part 文件。
// 这些文件从未通过哈希校验，不能复用，也不能让它们长期占用空间。
func cleanPartialDownloads(cacheDir string) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".part") {
			_ = os.Remove(filepath.Join(cacheDir, entry.Name()))
		}
	}
}

// goTool 描述一个通过 go install 获取的命令行工具。
type goTool struct {
	Name       string   // 可执行文件名（不含平台后缀）
	Package    string   // go install 的包路径
	Version    string   // 锁定版本（如 v2.14.0）
	VersionArg []string // 打印版本号所用的参数
	// UseGOPATHBin 为 true 时装到 Go 约定的 GOPATH/bin，让开发者直接执行；
	// 否则装到仓库内 .tools/bin。
	UseGOPATHBin bool
}

// goTools 是引导器需要的全部 Go 工具。版本取自 Taskfile.yml（经 Config 传入）。
func (c *Config) goTools() []goTool {
	return []goTool{
		{Name: "task", Package: "github.com/go-task/task/v3/cmd/task", Version: c.TaskVersion, VersionArg: []string{"--version"}, UseGOPATHBin: true},
		{Name: "golangci-lint", Package: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", Version: c.GolangciLintVersion, VersionArg: []string{"version"}},
		{Name: "govulncheck", Package: "golang.org/x/vuln/cmd/govulncheck", Version: c.GovulncheckVersion, VersionArg: []string{"-version"}},
	}
}

// installGoTools 逐个确认 goTools 是否已是锁定版本，不是则 go install。
// 已存在的工具（包括 PATH 中的）若版本匹配则直接复用。
func installGoTools(cfg *Config, force bool) error {
	fmt.Println("Go 工具")
	gopathBin, err := goPathBin()
	if err != nil {
		return err
	}

	for _, tool := range cfg.goTools() {
		version := strings.TrimPrefix(tool.Version, "v")
		dest := filepath.Join(cfg.BinDir, tool.Name+exeSuffix())
		if tool.UseGOPATHBin {
			dest = filepath.Join(gopathBin, tool.Name+exeSuffix())
		}
		if existing := firstExisting(dest, which(tool.Name)); !force && toolVersionMatches(existing, tool.Version, tool.VersionArg...) {
			fmt.Printf("  OK   %s %s（使用已有 %s）\n", tool.Name, version, relToRepo(existing))
			continue
		}
		gobin := cfg.BinDir
		if tool.UseGOPATHBin {
			gobin = ""
		}
		if err := goInstall(gobin, tool.Package, tool.Version); err != nil {
			return err
		}
		fmt.Printf("  OK   %s %s -> %s\n", tool.Name, version, relToRepo(dest))
	}
	fmt.Println()
	return nil
}

// goInstall 执行 go install pkg@version。gobin 为空时装到 Go 默认的 GOBIN/GOPATH 位置。
func goInstall(gobin, pkg, version string) error {
	env := os.Environ()
	if gobin != "" {
		env = append(env, "GOBIN="+gobin)
	} else {
		// 清空 GOBIN，让 go install 落到 GOPATH/bin。
		env = append(env, "GOBIN=")
	}
	target := pkg + "@" + version
	fmt.Printf("  安装 %s\n", target)
	if err := runInherit("", env, "go", "install", target); err != nil {
		return fmt.Errorf("go install %s 失败：%w", target, err)
	}
	return nil
}

// toolVersionMatches 运行工具的版本命令，若输出中包含锁定版本号（忽略前导 v）则返回 true。
// 用来判断已有工具是否可复用，而不是只看它是否存在。
func toolVersionMatches(bin, version string, args ...string) bool {
	if bin == "" {
		return false
	}
	if _, err := os.Stat(bin); err != nil {
		return false
	}
	out, err := runCapture(bin, args...)
	if err != nil {
		return false
	}
	return strings.Contains(out, strings.TrimPrefix(version, "v"))
}

func ensureComponent(cfg *Config, comp component, force bool) error {
	fmt.Printf("%s %s\n", comp.Name, comp.Version)
	if !force && componentReady(cfg, comp) {
		fmt.Printf("  已就绪\n\n")
		return nil
	}

	if err := os.RemoveAll(comp.TargetDir); err != nil {
		return err
	}
	archive, err := fetchArchive(comp, cfg.CacheDir)
	if err != nil {
		return err
	}
	fmt.Printf("  解压到 %s\n", relToRepo(comp.TargetDir))
	if err := extractArchive(archive, comp.Archive, comp.TargetDir); err != nil {
		return err
	}
	if err := prepareComponent(cfg, comp); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(comp.TargetDir, ".version"), []byte(componentMarker(comp)+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Println()
	return nil
}

// componentMarker 生成组件的就绪标记：版本号加锁定哈希。
// 哈希一并写入，是为了在资产地址或格式变化（同版本不同文件）时让旧安装自动失效。
func componentMarker(comp component) string {
	return comp.Version + " " + strings.ToLower(comp.Hash)
}

// componentReady 判断组件是否已安装且可用：标记匹配，并且能定位到关键可执行文件。
func componentReady(cfg *Config, comp component) bool {
	content, err := os.ReadFile(filepath.Join(comp.TargetDir, ".version"))
	if err != nil || strings.TrimSpace(string(content)) != componentMarker(comp) {
		return false
	}
	switch comp.Name {
	case "dotnet":
		_, err := findDotnetExe(comp.TargetDir)
		return err == nil
	case "godot":
		_, err := findGodotExe(comp.TargetDir)
		return err == nil
	case "llvm":
		_, err := findLlvmClangCl(comp.TargetDir)
		return err == nil
	case "llvm-mingw":
		_, err := findMingwRoot(comp.TargetDir)
		return err == nil
	}
	return false
}

// prepareComponent 在解压之后做收尾：校验关键产物存在、补执行位，并为 Godot 生成稳定的 shim。
// 返回错误意味着安装失败，调用方不会写入 .version 标记。
func prepareComponent(cfg *Config, comp component) error {
	switch comp.Name {
	case "dotnet":
		exe, err := findDotnetExe(comp.TargetDir)
		if err != nil {
			return err
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(exe, 0o755); err != nil {
				return err
			}
		}
	case "godot":
		exe, err := findGodotExe(comp.TargetDir)
		if err != nil {
			return err
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(exe, 0o755); err != nil {
				return err
			}
		}
		if _, err := writeShim(cfg, "godot", exe); err != nil {
			return err
		}
	case "llvm":
		// msiexec 管理安装会交给 Windows Installer 服务，退出码可能是 0 而文件尚未落盘，
		// 因此以“能否定位到 clang-cl”为成功判据，并给它足够时间。
		if _, err := waitForFind(comp.TargetDir, 5*time.Minute, findLlvmClangCl); err != nil {
			return err
		}
	case "llvm-mingw":
		if _, err := findMingwRoot(comp.TargetDir); err != nil {
			return err
		}
	}
	return nil
}

// waitForFind 反复调用 find，直到它成功或超时。用于 MSI 管理安装这类可能异步完成的步骤。
// 轮询间隔 2 秒。
func waitForFind(root string, timeout time.Duration, find func(string) (string, error)) (string, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		path, err := find(root)
		if err == nil {
			return path, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return "", fmt.Errorf("%v（等待 %s 后仍未见文件）", lastErr, timeout)
		}
		time.Sleep(2 * time.Second)
	}
}

// restoreNuGet 用锁定的 .NET SDK 按 packages.lock.json 还原三个项目（--locked-mode）。
// 若 .NET SDK 不可用则跳过并提示。
func restoreNuGet(cfg *Config) error {
	paths := resolvePaths(cfg)
	if paths.Dotnet == "" {
		fmt.Println(".NET SDK 不可用，跳过 NuGet 还原")
		fmt.Println()
		return nil
	}
	fmt.Println("NuGet 依赖（--locked-mode）")
	env := append(os.Environ(),
		"DOTNET_ROOT="+paths.DotnetRoot,
		"DOTNET_CLI_TELEMETRY_OPTOUT=1",
		"DOTNET_NOLOGO=1",
		"PATH="+joinPathList(paths.DotnetRoot, os.Getenv("PATH")),
	)
	projects := []string{
		"client/src/Core/Riftcards.Client.Core.csproj",
		"client/Riftcards.Client.csproj",
		"client/tests/Riftcards.Client.Core.Tests/Riftcards.Client.Core.Tests.csproj",
	}
	for _, project := range projects {
		if err := runInherit(cfg.RepoRoot, env, paths.Dotnet, "restore", project, "--locked-mode"); err != nil {
			return fmt.Errorf("dotnet restore %s 失败：%w", project, err)
		}
	}
	fmt.Println()
	return nil
}

func reportPaths(paths toolchainPaths, envPath string) {
	fmt.Println("工具位置")
	fmt.Printf("  task           %s\n", displayOrMissing(paths.Task))
	fmt.Printf("  golangci-lint  %s\n", displayOrMissing(paths.GolangciLint))
	fmt.Printf("  govulncheck    %s\n", displayOrMissing(paths.Govulncheck))
	fmt.Printf("  dotnet         %s\n", displayOrMissing(paths.Dotnet))
	fmt.Printf("  godot          %s\n", displayOrMissing(paths.Godot))
	fmt.Printf("  clang          %s\n", displayOrMissing(paths.Clang))
	if paths.LlvmMingwRoot != "" {
		fmt.Printf("  llvm-mingw     %s\n", relToRepo(paths.LlvmMingwRoot))
	}
	fmt.Printf("\n映射文件：%s\n", relToRepo(envPath))
	fmt.Printf("如果 task 不在 PATH 中，把下面目录加入 PATH（一次性）：\n  %s\n", mustGoPathBin())
	fmt.Println("\n下一步：task env")
}

func displayOrMissing(path string) string {
	if path == "" {
		return "（未找到）"
	}
	return relToRepo(path)
}

func mustGoPathBin() string {
	bin, err := goPathBin()
	if err != nil {
		return "$(go env GOPATH)/bin"
	}
	return bin
}

func runCapture(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out.String(), nil
}

func runInherit(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if env != nil {
		cmd.Env = env
	}
	return cmd.Run()
}
