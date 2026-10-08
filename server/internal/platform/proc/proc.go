// Package proc 启动并管理外部进程，提供跨平台一致的参数、目录、环境、
// 标准输入输出、退出状态、取消与超时语义。
//
// 它是平台适配层：允许访问宿主文件系统和进程表。规则层（game、match、
// protocol）不得依赖本包；工具与端到端测试用它拉起服务端和客户端。
// 命令只以参数数组传递，不经过系统 shell。
package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mjq/riftcards/server/internal/platform/pathutil"
)

const (
	// DefaultCaptureLimit 是未显式接管输出时单个流的捕获上限，超出后截断。
	DefaultCaptureLimit = 1 << 20
	// DefaultWaitDelay 是取消或超时后等待 I/O 收尾与进程退出的宽限时间。
	DefaultWaitDelay = 2 * time.Second
)

var (
	// ErrEmptyName 表示没有给出可执行文件名。
	ErrEmptyName = errors.New("proc: 可执行文件名为空")
	// ErrNotFound 表示在附加目录与 PATH 中都没有找到可执行文件。
	ErrNotFound = errors.New("proc: 未找到可执行文件")
)

// Spec 描述一次进程启动。Name 与 Path 二选一：Path 非空时直接使用。
type Spec struct {
	Name       string
	Path       string
	Args       []string
	Dir        string
	Env        []string // nil 表示继承父进程环境；非 nil 表示整体替换
	Stdin      io.Reader
	Stdout     io.Writer // nil 表示捕获到 Result.Stdout
	Stderr     io.Writer // nil 表示捕获到 Result.Stderr
	ExtraPaths []string  // 查找 Name 时优先搜索的目录
	Timeout    time.Duration
	// CaptureLimit 为 0 时使用 DefaultCaptureLimit。
	CaptureLimit int
}

// Result 是一次运行的观察结果。ExitCode 为 -1 表示进程被信号终止或启动后异常结束。
type Result struct {
	Path            string
	Args            []string
	ExitCode        int
	Stdout          string
	Stderr          string
	StdoutTruncated bool
	StderrTruncated bool
	Duration        time.Duration
	TimedOut        bool
	Canceled        bool
}

// HostOS 返回当前主机对应的 pathutil 目标系统。
func HostOS() pathutil.OS {
	switch runtime.GOOS {
	case "windows":
		return pathutil.Windows
	case "darwin":
		return pathutil.MacOS
	default:
		return pathutil.Linux
	}
}

// LookPath 按附加目录、PATH 的顺序查找可执行文件，并按主机平台补可执行后缀。
func LookPath(name string, extraPaths []string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", ErrEmptyName
	}
	exeName, err := pathutil.ExecutableName(HostOS(), name)
	if err != nil {
		return "", err
	}

	dirs := make([]string, 0, len(extraPaths)+8)
	dirs = append(dirs, extraPaths...)
	dirs = append(dirs, filepathList(os.Getenv("PATH"))...)

	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		candidate, err := pathutil.Join(HostOS(), dir, exeName)
		if err != nil {
			continue
		}
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if !isExecutable(info) {
			continue
		}
		return candidate, nil
	}
	return "", fmt.Errorf("%w: %s（查找目录：%s）", ErrNotFound, exeName, strings.Join(dirs, string(os.PathListSeparator)))
}

// Run 启动进程并等待结束。取消或超时会终止整棵子进程树。
func Run(ctx context.Context, spec Spec) (Result, error) {
	if spec.Path == "" {
		path, err := LookPath(spec.Name, spec.ExtraPaths)
		if err != nil {
			return Result{}, err
		}
		spec.Path = path
	}

	runCtx := ctx
	if spec.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, spec.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(runCtx, spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	if spec.Env != nil {
		cmd.Env = spec.Env
	}
	cmd.Stdin = spec.Stdin
	cmd.WaitDelay = DefaultWaitDelay

	limit := spec.CaptureLimit
	if limit <= 0 {
		limit = DefaultCaptureLimit
	}
	var outBuffer, errBuffer *limitedBuffer
	if spec.Stdout != nil {
		cmd.Stdout = spec.Stdout
	} else {
		outBuffer = newLimitedBuffer(limit)
		cmd.Stdout = outBuffer
	}
	if spec.Stderr != nil {
		cmd.Stderr = spec.Stderr
	} else {
		errBuffer = newLimitedBuffer(limit)
		cmd.Stderr = errBuffer
	}

	// 取消或超时时终止整棵进程树，而不是只杀直接子进程。
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return killTree(cmd.Process.Pid)
	}
	configureSysProcAttr(cmd)

	started := time.Now()
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("启动 %s 失败：%w", spec.Path, err)
	}

	waitErr := cmd.Wait()
	result := Result{
		Path:     spec.Path,
		Args:     spec.Args,
		ExitCode: exitCode(waitErr),
		Duration: time.Since(started),
	}
	if outBuffer != nil {
		result.Stdout = outBuffer.String()
		result.StdoutTruncated = outBuffer.Truncated()
	}
	if errBuffer != nil {
		result.Stderr = errBuffer.String()
		result.StderrTruncated = errBuffer.Truncated()
	}
	switch {
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		result.TimedOut = true
	case ctx.Err() != nil:
		result.Canceled = true
	}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			return result, fmt.Errorf("运行 %s 失败：%w", spec.Path, waitErr)
		}
	}
	return result, nil
}

func exitCode(waitErr error) int {
	if waitErr == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func filepathList(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, string(os.PathListSeparator))
}

// limitedBuffer 是带上限的并发安全缓冲，超出后丢弃多余字节并标记截断。
type limitedBuffer struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		if _, err := b.buffer.Write(p[:remaining]); err != nil {
			return 0, err
		}
		b.truncated = true
		return len(p), nil
	}
	if _, err := b.buffer.Write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *limitedBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}
