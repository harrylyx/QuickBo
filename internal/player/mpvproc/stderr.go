package mpvproc

import (
	"fmt"
	"strings"
	"sync"
)

// mpvStderrLimit is how many trailing bytes of mpv's stderr to keep. The tail
// holds the failure cause; the head is just the version banner.
const mpvStderrLimit = 4 << 10

// stderrTail collects the last few bytes of the child's stderr. The writer is
// exec's copy goroutine; the reader is Load's failure path, which runs only
// after cmd.Wait has returned (so the copy goroutine has finished).
type stderrTail struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (s *stderrTail) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = append(s.buf, p...)
	if len(s.buf) > s.max {
		s.buf = s.buf[len(s.buf)-s.max:]
	}
	return len(p), nil
}

func (s *stderrTail) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.TrimSpace(string(s.buf))
}

// mpvLoadError appends mpv's own output to a Load failure so "IPC connect
// failed" is not the only thing the user sees.
func mpvLoadError(err error, stderr string) error {
	if stderr = strings.TrimSpace(stderr); stderr == "" {
		return err
	}
	return fmt.Errorf("%w（mpv 输出: %s）", err, stderr)
}
