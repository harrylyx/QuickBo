//go:build windows

package mpvproc

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
)

// pipeSeq keeps named-pipe base names unique within the process.
var pipeSeq atomic.Uint64

// newIPCPath returns a unique named-pipe base name (without the \\.\pipe\
// prefix; mpv adds that itself).
func newIPCPath() (string, error) {
	return fmt.Sprintf("quickbo-mpv-%d", pipeSeq.Add(1)), nil
}

// dialIPC makes one attempt to open mpv's named pipe client end.
func dialIPC(path string) (io.ReadWriteCloser, error) {
	return os.OpenFile(`\\.\pipe\`+path, os.O_RDWR, 0)
}

// cleanupIPC is a no-op: the named pipe dies with the mpv process.
func cleanupIPC(path string) {}

// setupProcAttr hides mpv's console window (mpv.exe is a console program).
func setupProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
