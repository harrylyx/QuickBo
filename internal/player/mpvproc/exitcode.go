package mpvproc

import (
	"errors"
	"fmt"
	"os/exec"
)

// windowsExitReasons maps the exit codes a Windows mpv prints nothing about
// before dying to a readable reason.
var windowsExitReasons = map[int]string{
	3221225595: "架构不匹配，通常是 32/64 位用错", // 0xC000007B STATUS_INVALID_IMAGE_FORMAT
	3221225477: "进程崩溃（访问冲突）",          // 0xC0000005 STATUS_ACCESS_VIOLATION
	3221225781: "找不到依赖的 DLL",          // 0xC0000135 STATUS_DLL_NOT_FOUND
	3221225785: "DLL 缺少入口点",           // 0xC0000139 STATUS_ENTRYPOINT_NOT_FOUND
	3221225794: "DLL 初始化失败",           // 0xC0000142 STATUS_DLL_INIT_FAILED
}

func describeExit(code int) string {
	if reason, ok := windowsExitReasons[code]; ok {
		return fmt.Sprintf("退出码 %d (0x%X)：%s", code, code, reason)
	}
	return fmt.Sprintf("退出码 %d", code)
}

// mpvExitError turns "exited before IPC" into a readable error carrying the
// exit code when available.
func mpvExitError(waitErr error) error {
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		return errMPVExitedEarly
	}
	return fmt.Errorf("%w（%s）", errMPVExitedEarly, describeExit(exitErr.ExitCode()))
}
