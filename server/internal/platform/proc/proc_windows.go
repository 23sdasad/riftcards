//go:build windows

package proc

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
)

// configureSysProcAttr 在 Windows 上不需要额外进程组设置：终止整棵树由 taskkill 完成。
func configureSysProcAttr(cmd *exec.Cmd) {
	_ = cmd
}

// isExecutable 在 Windows 上以扩展名判断，LookPath 已经补过 .exe。
func isExecutable(info os.FileInfo) bool {
	return !info.IsDir()
}

// killTree 用系统自带的 taskkill 终止整棵进程树；进程已退出时不算错误。
func killTree(pid int) error {
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if err := kill.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil
		}
		return err
	}
	return nil
}
