// Package mpvplugin locates and validates an external mpv install.
package mpvplugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Find locates mpv: PATH first, then app-bundled locations. It returns the
// path or an error whose message carries an install hint.
func Find() (string, error) {
	if p, err := exec.LookPath("mpv"); err == nil {
		return p, nil
	}
	if p := bundledPath(); p != "" {
		return p, nil
	}
	return "", fmt.Errorf("未找到 mpv 可执行文件。%s", InstallHint())
}

// bundledPath returns an mpv shipped next to the app (Windows mpv/ dir).
func bundledPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Join(filepath.Dir(exe), "mpv", exeName())
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	return ""
}

func exeName() string {
	if runtime.GOOS == "windows" {
		return "mpv.exe"
	}
	return "mpv"
}

// Check runs mpv --version and returns its first line, or an error.
func Check(path string) (string, error) {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("mpv 预检失败: %w", err)
	}
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]), nil
}

// InstallHint returns a one-line install command for the current OS.
func InstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "请安装 mpv：brew install mpv"
	case "linux":
		for _, pair := range []struct{ name, cmd string }{
			{"apt", "sudo apt install mpv"},
			{"dnf", "sudo dnf install mpv"},
			{"pacman", "sudo pacman -S mpv"},
		} {
			if _, err := exec.LookPath(pair.name); err == nil {
				return "请安装 mpv：" + pair.cmd
			}
		}
		return "请安装 mpv：sudo apt install mpv"
	case "windows":
		return "请从 https://mpv.io/installation/ 下载并安装 mpv"
	default:
		return "请安装 mpv"
	}
}
