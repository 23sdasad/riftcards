package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type bootstrapOptions struct {
	Skip  map[string]bool
	Force bool
}

func parseBootstrapArgs(args []string) (bootstrapOptions, error) {
	opts := bootstrapOptions{Skip: map[string]bool{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--force":
			opts.Force = true
		case arg == "--without" && i+1 < len(args):
			i++
			for _, name := range strings.Split(args[i], ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					opts.Skip[name] = true
				}
			}
		case strings.HasPrefix(arg, "--without="):
			for _, name := range strings.Split(strings.TrimPrefix(arg, "--without="), ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					opts.Skip[name] = true
				}
			}
		default:
			return opts, fmt.Errorf("未知参数 %q", arg)
		}
	}
	return opts, nil
}

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

	fmt.Printf("引导 riftcards 依赖：%s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("安装目录：%s\n\n", relToRepo(cfg.ToolsDir))

	if !opts.Skip["gotools"] {
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
	reportPaths(paths, envPath)
	return nil
}

func installGoTools(cfg *Config, force bool) error {
	fmt.Println("Go 工具")
	gopathBin, err := goPathBin()
	if err != nil {
		return err
	}

	items := []struct {
		name string
		pkg  string
		ver  string
		args []string
		dest string
	}{
		{"task", "github.com/go-task/task/v3/cmd/task", cfg.TaskVersion, []string{"--version"}, filepath.Join(gopathBin, "task"+exeSuffix())},
		{"golangci-lint", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", cfg.GolangciLintVersion, []string{"version"}, filepath.Join(cfg.BinDir, "golangci-lint"+exeSuffix())},
		{"govulncheck", "golang.org/x/vuln/cmd/govulncheck", cfg.GovulncheckVersion, []string{"-version"}, filepath.Join(cfg.BinDir, "govulncheck"+exeSuffix())},
	}
	for _, item := range items {
		version := strings.TrimPrefix(item.ver, "v")
		if existing := firstExisting(item.dest, which(item.name)); !force && toolVersionMatches(existing, item.ver, item.args...) {
			fmt.Printf("  OK   %s %s（使用已有 %s）\n", item.name, version, relToRepo(existing))
			continue
		}
		gobin := cfg.BinDir
		if item.name == "task" {
			// Task 需要能被开发者直接执行，装到 Go 约定的 GOPATH/bin。
			gobin = ""
		}
		if err := goInstall(gobin, item.pkg, item.ver); err != nil {
			return err
		}
		fmt.Printf("  OK   %s %s -> %s\n", item.name, version, relToRepo(item.dest))
	}
	fmt.Println()
	return nil
}

func goInstall(gobin, pkg, version string) error {
	env := os.Environ()
	if gobin != "" {
		env = append(env, "GOBIN="+gobin)
	} else {
		env = append(env, "GOBIN=")
	}
	target := pkg + "@" + version
	fmt.Printf("  安装 %s\n", target)
	if err := runInherit("", env, "go", "install", target); err != nil {
		return fmt.Errorf("go install %s 失败：%w", target, err)
	}
	return nil
}

// toolVersionMatches 运行工具的版本命令，确认输出里出现锁定版本号。
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
	if err := os.WriteFile(filepath.Join(comp.TargetDir, ".version"), []byte(comp.Version+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Println()
	return nil
}

func componentReady(cfg *Config, comp component) bool {
	content, err := os.ReadFile(filepath.Join(comp.TargetDir, ".version"))
	if err != nil || strings.TrimSpace(string(content)) != comp.Version {
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

// prepareComponent 做解压后的补充处理：定位可执行文件、补执行位、生成 shim。
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
		if _, err := findLlvmClangCl(comp.TargetDir); err != nil {
			return err
		}
	case "llvm-mingw":
		if _, err := findMingwRoot(comp.TargetDir); err != nil {
			return err
		}
	}
	return nil
}

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
