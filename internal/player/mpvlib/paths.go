package mpvlib

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// candidates lists where libmpv may live, most specific first: next to the
// executable (and an .app bundle's Frameworks), then the usual system
// locations, then the bare name for the loader's own search path.
func candidates() []string {
	var names []string
	switch runtime.GOOS {
	case "darwin":
		names = []string{"libmpv.2.dylib", "libmpv.dylib"}
	case "windows":
		names = []string{"libmpv-2.dll", "mpv-2.dll"}
	default:
		names = []string{"libmpv.so.2", "libmpv.so"}
	}
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		dirs = append(dirs, dir, filepath.Join(dir, "mpv"))
		if runtime.GOOS == "darwin" {
			dirs = append(dirs, filepath.Join(dir, "..", "Frameworks"))
		}
	}
	switch runtime.GOOS {
	case "darwin":
		dirs = append(dirs, "/opt/homebrew/lib", "/usr/local/lib", "/opt/local/lib")
	case "windows":
		// An mpv on PATH from a libmpv build usually ships the DLL beside it.
		if p, err := exec.LookPath("mpv"); err == nil {
			dirs = append(dirs, filepath.Dir(p))
		}
	}
	var out []string
	for _, d := range dirs {
		for _, n := range names {
			p := filepath.Join(d, n)
			if _, err := os.Stat(p); err == nil {
				out = append(out, p)
			}
		}
	}
	return append(out, names...)
}

// InstallHint returns a one-line hint for getting libmpv on this OS.
func InstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "请安装 mpv（含 libmpv）：brew install mpv"
	case "windows":
		return "请把 libmpv-2.dll 放到 QuickBo 程序目录（可从 https://sourceforge.net/projects/mpv-player-windows/files/libmpv/ 获取）"
	default:
		return "请安装 libmpv：sudo apt install libmpv2"
	}
}
