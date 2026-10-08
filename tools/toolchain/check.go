package main

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`\d+(?:\.\d+)+`)

// parseVersion 提取输出里第一个形如 1.27.0 的版本号。
func parseVersion(text string) string {
	return versionPattern.FindString(text)
}

// compareVersions 比较点分数字版本；缺失段按 0 处理。
func compareVersions(a, b string) int {
	as := strings.Split(parseVersion(a), ".")
	bs := strings.Split(parseVersion(b), ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var av, bv int
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

// majorVersion 返回 23.1.3 -> 23，用于 clang 的 lib/clang/<major> 资源目录。
func majorVersion(version string) string {
	parsed := parseVersion(version)
	if parsed == "" {
		return ""
	}
	major, _, _ := strings.Cut(parsed, ".")
	return major
}

type checkResult struct {
	Name    string
	Status  string
	Detail  string
	Install string
}

func runCheck() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	paths := resolvePaths(cfg)

	results := []checkResult{
		checkTool("go", which("go"), cfg.GoMinVersion, false, []string{"version"},
			fmt.Sprintf("Go %s+（权威版本见 server/go.mod）", cfg.GoMinVersion), "https://go.dev/dl/"),
		checkTool("task", paths.Task, cfg.TaskVersion, true, []string{"--version"},
			"Go Task "+strings.TrimPrefix(cfg.TaskVersion, "v"), installEntry),
		checkTool("golangci-lint", paths.GolangciLint, cfg.GolangciLintVersion, true, []string{"version"},
			cfg.GolangciLintVersion, installEntry),
		checkTool("govulncheck", paths.Govulncheck, cfg.GovulncheckVersion, true, []string{"-version"},
			cfg.GovulncheckVersion, installEntry),
		checkTool("dotnet", paths.Dotnet, cfg.DotnetMinVersion, false, []string{"--version"},
			".NET SDK "+cfg.DotnetMinVersion+"+（权威版本见 global.json）", installEntry),
		checkTool("godot", paths.Godot, cfg.GodotMinVersion, true, []string{"--version"},
			"Godot "+cfg.GodotVersion+" .NET 版", installEntry),
		checkTool("clang", paths.Clang, cfg.LlvmMinVersion, false, []string{"--version"},
			"LLVM "+cfg.LlvmVersion+"（Windows cgo 见 tools/llvm/clang-cl.cmd）", installEntry),
	}
	if runtime.GOOS == "windows" {
		result := checkResult{Name: "llvm-mingw", Status: "OK", Detail: relToRepo(paths.LlvmMingwRoot), Install: installEntry}
		if paths.LlvmMingwRoot == "" {
			result.Status = "MISSING"
			result.Detail = "需要 LLVM-MinGW " + cfg.MingwVersion
		}
		results = append(results, result)
	}

	fmt.Println("环境检查（riftcards）")
	missing := 0
	for _, result := range results {
		switch result.Status {
		case "OK":
			fmt.Printf("OK       %-14s %s\n", result.Name, result.Detail)
		case "WARN":
			fmt.Printf("WARN     %-14s %s\n", result.Name, result.Detail)
		default:
			missing++
			fmt.Printf("MISSING  %-14s %s；安装入口 %s\n", result.Name, result.Detail, result.Install)
		}
	}
	if missing > 0 {
		return fmt.Errorf("缺少 %d 个必需工具，先运行：%s", missing, installEntry)
	}
	return nil
}

func checkTool(name, bin, pinned string, exact bool, args []string, requirement, install string) checkResult {
	if bin == "" {
		return checkResult{Name: name, Status: "MISSING", Detail: "需要 " + requirement, Install: install}
	}
	out, err := runCapture(bin, args...)
	if err != nil {
		return checkResult{Name: name, Status: "MISSING", Detail: "无法执行 " + relToRepo(bin), Install: install}
	}
	line := versionLine(name, out)
	version := parseVersion(line)
	if version == "" {
		return checkResult{Name: name, Status: "WARN", Detail: line + "（未能解析版本）", Install: install}
	}
	if exact {
		if version != parseVersion(pinned) {
			return checkResult{Name: name, Status: "WARN", Detail: fmt.Sprintf("%s；锁定 %s", line, pinned), Install: install}
		}
		return checkResult{Name: name, Status: "OK", Detail: line}
	}
	if compareVersions(version, pinned) < 0 {
		return checkResult{Name: name, Status: "WARN", Detail: fmt.Sprintf("%s；需要 %s 或更新", line, pinned), Install: install}
	}
	return checkResult{Name: name, Status: "OK", Detail: line}
}

// versionLine 优先取包含工具名的输出行：govulncheck 等工具的首行是 Go 版本。
func versionLine(name, output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(line), strings.ToLower(name)) {
			return line
		}
	}
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}
