package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mjq/riftcards/server/internal/platform/pathutil"
	"github.com/mjq/riftcards/server/internal/platform/proc"
	"github.com/mjq/riftcards/server/internal/testfixtures"
)

const (
	buildTimeout    = 3 * time.Minute
	startupTimeout  = 30 * time.Second
	shutdownTimeout = 20 * time.Second
)

var listenPattern = regexp.MustCompile(`addr=(\S+)`)

// syncBuffer 是可并发读写的日志缓冲：服务进程在写，测试在轮询。
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(payload []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(payload)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// serverHarness 管理一个真实服务进程的生命周期。
type serverHarness struct {
	baseURL string
	wsURL   string
	logs    *syncBuffer

	done    chan proc.Result
	stopped chan struct{}
	result  proc.Result

	cancel context.CancelFunc
	stop   sync.Once
}

// startServer 编译并启动真实服务进程；测试结束时会终止整棵进程树。
//
// 服务端用 `-addr 127.0.0.1:0` 让操作系统选端口，测试从结构化日志里读取真实地址，
// 因此不依赖固定端口，也不会因端口残留而失败。
func startServer(t *testing.T, seed int64) *serverHarness {
	t.Helper()

	root, err := testfixtures.RepoRoot()
	if err != nil {
		t.Fatalf("定位仓库根失败：%v", err)
	}
	hostOS := proc.HostOS()
	binaryName, err := pathutil.ExecutableName(hostOS, "riftcards-server")
	if err != nil {
		t.Fatalf("构造可执行文件名失败：%v", err)
	}
	binary, err := pathutil.Join(hostOS, t.TempDir(), binaryName)
	if err != nil {
		t.Fatalf("拼接二进制路径失败：%v", err)
	}
	serverDir, err := pathutil.Join(hostOS, root, "server")
	if err != nil {
		t.Fatalf("拼接服务端目录失败：%v", err)
	}

	buildCtx, buildCancel := context.WithTimeout(context.Background(), buildTimeout)
	defer buildCancel()
	build, err := proc.Run(buildCtx, proc.Spec{
		Name: "go",
		Args: []string{"build", "-o", binary, "./cmd/riftcards-server"},
		Dir:  serverDir,
	})
	if err != nil {
		t.Fatalf("构建服务端失败：%v", err)
	}
	if build.ExitCode != 0 {
		t.Fatalf("构建服务端退出码 %d：\n%s", build.ExitCode, build.Stderr)
	}

	logs := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan proc.Result, 1)
	go func() {
		result, _ := proc.Run(ctx, proc.Spec{
			Path:   binary,
			Args:   []string{"-addr", "127.0.0.1:0", "-seed", fmt.Sprintf("%d", seed), "-log-level", "debug"},
			Stdout: logs,
			Stderr: logs,
		})
		done <- result
	}()

	harness := &serverHarness{logs: logs, done: done, stopped: make(chan struct{}), cancel: cancel}
	t.Cleanup(func() { harness.shutdown(t) })

	addr := harness.waitForAddress(t)
	harness.baseURL = "http://" + addr
	harness.wsURL = "ws://" + addr + "/ws"
	harness.waitForHealth(t)
	return harness
}

// waitForAddress 从服务端结构化日志里读取解析后的监听地址。
func (h *serverHarness) waitForAddress(t *testing.T) string {
	t.Helper()
	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		if match := listenPattern.FindStringSubmatch(h.logs.String()); match != nil {
			return match[1]
		}
		select {
		case result := <-h.done:
			t.Fatalf("服务进程提前退出：exit=%d canceled=%v\n%s", result.ExitCode, result.Canceled, h.logs.String())
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("等待服务端监听地址超时：\n%s", h.logs.String())
	return ""
}

func (h *serverHarness) waitForHealth(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		response, err := http.Get(h.baseURL + "/healthz")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("等待 /healthz 超时：\n%s", h.logs.String())
}

// metrics 读取 /metrics 文本并解析成 map。
func (h *serverHarness) metrics(t *testing.T) map[string]int64 {
	t.Helper()
	response, err := http.Get(h.baseURL + "/metrics")
	if err != nil {
		t.Fatalf("读取 /metrics 失败：%v", err)
	}
	defer func() { _ = response.Body.Close() }()

	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(response.Body); err != nil {
		t.Fatalf("读取 /metrics 内容失败：%v", err)
	}

	values := make(map[string]int64)
	for _, line := range strings.Split(strings.TrimSpace(body.String()), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("/metrics 行格式异常：%q", line)
		}
		var value int64
		if _, err := fmt.Sscanf(fields[1], "%d", &value); err != nil {
			t.Fatalf("/metrics 数值解析失败：%q", line)
		}
		values[fields[0]] = value
	}
	return values
}

// shutdown 终止服务进程并确认它已经退出（不留孤儿进程）。
// 可重复调用：结果只收集一次，后续调用等待同一结果。
func (h *serverHarness) shutdown(t *testing.T) {
	h.stop.Do(func() {
		h.cancel()
		select {
		case result := <-h.done:
			h.result = result
			t.Logf("服务进程已退出：exit=%d canceled=%v duration=%s", result.ExitCode, result.Canceled, result.Duration)
		case <-time.After(shutdownTimeout):
			t.Errorf("服务进程在 %s 内没有退出：\n%s", shutdownTimeout, h.logs.String())
		}
		close(h.stopped)
	})
	<-h.stopped
}

// assertNoOrphans 确认服务进程已经退出，且健康检查不再响应。
func (h *serverHarness) assertNoOrphans(t *testing.T) {
	t.Helper()
	select {
	case <-h.stopped:
	case <-time.After(shutdownTimeout):
		t.Fatalf("服务进程仍在运行：\n%s", h.logs.String())
	}

	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(h.baseURL + "/healthz")
	if err == nil {
		_ = response.Body.Close()
		t.Fatalf("服务端在退出后仍响应 /healthz：%s", h.baseURL)
	}
}

// logContains 断言结构化日志包含给定字段片段。
func (h *serverHarness) logContains(t *testing.T, fragments ...string) {
	t.Helper()
	logs := h.logs.String()
	for _, fragment := range fragments {
		if !strings.Contains(logs, fragment) {
			t.Errorf("服务端日志缺少 %q\n完整日志：\n%s", fragment, logs)
		}
	}
}

// waitForLog 等待日志出现给定片段：日志写入是异步的，不能用一次性断言。
func (h *serverHarness) waitForLog(t *testing.T, fragment string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(h.logs.String(), fragment) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("等待日志 %q 超时：\n%s", fragment, h.logs.String())
}

// serverLog 返回当前服务端日志。
func (h *serverHarness) serverLog() string {
	return h.logs.String()
}
