package proc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mjq/riftcards/server/internal/platform/pathutil"
)

const (
	helperEnv     = "PROC_HELPER"
	helperModeEnv = "PROC_HELPER_MODE"
	helperArgEnv  = "PROC_HELPER_ARG"
)

// TestHelperProcess 不是普通测试：它作为子进程助手被 Run 拉起。
func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		return
	}
	mode := os.Getenv(helperModeEnv)
	arg := os.Getenv(helperArgEnv)

	switch mode {
	case "describe":
		wd, _ := os.Getwd()
		var extra []string
		for _, arg := range os.Args[1:] {
			if strings.HasPrefix(arg, "-test.") {
				continue
			}
			extra = append(extra, arg)
		}
		fmt.Printf("args=%s\n", strings.Join(extra, ","))
		fmt.Printf("wd=%s\n", wd)
		fmt.Printf("arg=%s\n", arg)
		os.Exit(0)
	case "fail":
		fmt.Fprintln(os.Stderr, "boom")
		os.Exit(3)
	case "stdin":
		data, _ := io.ReadAll(os.Stdin)
		fmt.Printf("stdin=%s\n", string(data))
		os.Exit(0)
	case "sleep":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "tree":
		// 先启动孙进程，再长时间休眠；父进程被终止时孙进程也必须消失。
		child := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
		child.Env = append(helperEnvList(), helperEnv+"=1", helperModeEnv+"=writefile", helperArgEnv+"="+arg)
		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case "writefile":
		time.Sleep(3 * time.Second)
		_ = os.WriteFile(arg, []byte("grandchild"), 0o644)
		os.Exit(0)
	case "big":
		payload := strings.Repeat("x", 4096)
		for i := 0; i < 512; i++ {
			fmt.Println(payload)
		}
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "unknown helper mode %q\n", mode)
	os.Exit(5)
}

// helperEnvList 给出替换式环境变量：保留 PATH 与 Windows 运行所需的基础变量。
func helperEnvList() []string {
	env := []string{"PATH=" + os.Getenv("PATH")}
	for _, name := range []string{"SYSTEMROOT", "TEMP", "TMP", "HOME", "USERPROFILE"} {
		if value := os.Getenv(name); value != "" {
			env = append(env, name+"="+value)
		}
	}
	return env
}

func specForHelper(mode, arg string) Spec {
	env := append(helperEnvList(), helperEnv+"=1", helperModeEnv+"="+mode)
	if arg != "" {
		env = append(env, helperArgEnv+"="+arg)
	}
	return Spec{Path: os.Args[0], Args: []string{"-test.run=TestHelperProcess"}, Env: env}
}

func TestLookPathResolvesPlatformSuffix(t *testing.T) {
	found, err := LookPath("go", nil)
	if err != nil {
		t.Fatalf("LookPath(go) 失败：%v", err)
	}
	want, err := pathutil.ExecutableName(HostOS(), "go")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(found) != want {
		t.Fatalf("解析结果 %q，期望文件名 %q", found, want)
	}

	if _, err := LookPath("", nil); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("空名字应返回 ErrEmptyName，实际 %v", err)
	}
	if _, err := LookPath(filepath.Join("bin", "go"), nil); err == nil {
		t.Fatal("带路径分隔符的名字应被拒绝")
	}
	if _, err := LookPath("definitely-not-a-tool-xyz", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在的工具应返回 ErrNotFound，实际 %v", err)
	}
}

func TestRunCapturesOutputDirEnvAndArgs(t *testing.T) {
	dir := t.TempDir()
	spec := specForHelper("describe", "hello")
	spec.Args = append(spec.Args, "one", "two")
	spec.Dir = dir

	result, err := Run(context.Background(), spec)
	if err != nil {
		t.Fatalf("Run 失败：%v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("退出码 = %d，期望 0（stderr=%s）", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "args=one,two") {
		t.Fatalf("参数未按数组传递：%q", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "arg=hello") {
		t.Fatalf("环境变量未生效：%q", result.Stdout)
	}
	if !strings.Contains(strings.ReplaceAll(result.Stdout, "\\", "/"), strings.ReplaceAll(dir, "\\", "/")) {
		t.Fatalf("工作目录未生效：%q", result.Stdout)
	}
	if result.Duration <= 0 {
		t.Fatal("Duration 应大于 0")
	}
}

func TestRunReportsNonZeroExitAndStderr(t *testing.T) {
	result, err := Run(context.Background(), specForHelper("fail", ""))
	if err != nil {
		t.Fatalf("Run 失败：%v", err)
	}
	if result.ExitCode != 3 {
		t.Fatalf("退出码 = %d，期望 3", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "boom") {
		t.Fatalf("标准错误未捕获：%q", result.Stderr)
	}
}

func TestRunFeedsStdin(t *testing.T) {
	spec := specForHelper("stdin", "")
	file := filepath.Join(t.TempDir(), "stdin.txt")
	if err := os.WriteFile(file, []byte("piped-input"), 0o644); err != nil {
		t.Fatal(err)
	}
	handle, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = handle.Close() }()
	spec.Stdin = handle

	result, err := Run(context.Background(), spec)
	if err != nil {
		t.Fatalf("Run 失败：%v", err)
	}
	if !strings.Contains(result.Stdout, "stdin=piped-input") {
		t.Fatalf("标准输入未传递：%q", result.Stdout)
	}
}

func TestRunTimesOutAndKillsProcess(t *testing.T) {
	spec := specForHelper("sleep", "")
	spec.Timeout = 400 * time.Millisecond

	start := time.Now()
	result, err := Run(context.Background(), spec)
	if err != nil {
		t.Fatalf("Run 失败：%v", err)
	}
	if !result.TimedOut {
		t.Fatalf("应标记 TimedOut：%+v", result)
	}
	if result.Canceled {
		t.Fatalf("超时不应标记 Canceled：%+v", result)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("超时后未及时返回：%s", elapsed)
	}
}

func TestRunHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	result, err := Run(ctx, specForHelper("sleep", ""))
	if err != nil {
		t.Fatalf("Run 失败：%v", err)
	}
	if !result.Canceled {
		t.Fatalf("应标记 Canceled：%+v", result)
	}
	if result.TimedOut {
		t.Fatalf("取消不应标记 TimedOut：%+v", result)
	}
}

func TestRunKillsGrandchildren(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "grandchild.txt")
	spec := specForHelper("tree", marker)
	spec.Timeout = 500 * time.Millisecond

	result, err := Run(context.Background(), spec)
	if err != nil {
		t.Fatalf("Run 失败：%v", err)
	}
	if !result.TimedOut {
		t.Fatalf("应标记 TimedOut：%+v", result)
	}
	// 孙进程本应在 3 秒后写文件；如果进程树被正确终止，文件不应出现。
	time.Sleep(4 * time.Second)
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("孙进程仍存活并写入了 %s", marker)
	}
}

func TestRunTruncatesLargeOutput(t *testing.T) {
	spec := specForHelper("big", "")
	spec.CaptureLimit = 1024

	result, err := Run(context.Background(), spec)
	if err != nil {
		t.Fatalf("Run 失败：%v", err)
	}
	if !result.StdoutTruncated {
		t.Fatalf("应标记截断：%+v", result)
	}
	if len(result.Stdout) > 1024 {
		t.Fatalf("捕获长度 %d 超过上限", len(result.Stdout))
	}
}

func TestHostOSMatchesRuntime(t *testing.T) {
	var want pathutil.OS
	switch runtime.GOOS {
	case "windows":
		want = pathutil.Windows
	case "darwin":
		want = pathutil.MacOS
	default:
		want = pathutil.Linux
	}
	if got := HostOS(); got != want {
		t.Fatalf("HostOS = %v，期望 %v", got, want)
	}
}
