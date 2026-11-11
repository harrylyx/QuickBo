//go:build !windows

package mpvproc

import (
	"io"
	"net"
	"os"
	"os/exec"
)

// newIPCPath returns a unique Unix socket path for --input-ipc-server.
func newIPCPath() (string, error) {
	sock, err := os.CreateTemp("", "quickbo-mpv-*.sock")
	if err != nil {
		return "", err
	}
	path := sock.Name()
	_ = sock.Close()
	_ = os.Remove(path)
	return path, nil
}

// dialIPC makes one attempt to connect to the mpv IPC socket.
func dialIPC(path string) (io.ReadWriteCloser, error) {
	return net.Dial("unix", path)
}

// cleanupIPC removes the Unix socket file.
func cleanupIPC(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}

// setupProcAttr is a no-op on non-Windows.
func setupProcAttr(cmd *exec.Cmd) {}
