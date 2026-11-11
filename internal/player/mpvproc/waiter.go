package mpvproc

import "os/exec"

// waiter calls exec.Cmd.Wait exactly once and broadcasts the result. Load's
// failure path, command timeouts and Close all need to reap the child, and
// Wait must not run twice.
type waiter struct {
	done chan struct{}
	err  error
}

func startWaiter(cmd *exec.Cmd) *waiter {
	w := &waiter{done: make(chan struct{})}
	go func() {
		w.err = cmd.Wait()
		close(w.done)
	}()
	return w
}

// exited returns a channel closed when the process exits (nil when no process).
func (w *waiter) exited() <-chan struct{} {
	if w == nil {
		return nil
	}
	return w.done
}

// wait blocks until exit and returns Wait's result; safe to call repeatedly.
func (w *waiter) wait() error {
	if w == nil {
		return nil
	}
	<-w.done
	return w.err
}
