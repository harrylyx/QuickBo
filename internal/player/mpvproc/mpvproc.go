// Package mpvproc implements player.Player over an mpv subprocess and its
// JSON IPC endpoint. --input-ipc-server uses a Unix socket on Linux/macOS and
// a named pipe on Windows (see ipc_unix.go / ipc_windows.go).
package mpvproc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/harrylyx/QuickBo/internal/player"
)

// ipcConnectTimeout is the longest we wait for mpv to create its IPC endpoint.
const ipcConnectTimeout = 5 * time.Second

// cmdResponseTimeout bounds a single command round-trip. Opening a slow stream
// can block mpv's main loop so IPC replies are delayed; keep it generous.
var cmdResponseTimeout = 15 * time.Second

// response is a command reply routed to send, tagged with the session so a
// late reply from a previous Load cannot be mistaken for the current one.
type response struct {
	session int64
	data    []byte
}

type playState int

const (
	psStopped playState = iota
	psPlaying
	psPaused
	psBuffering
)

// procState is the latest observable playback state, updated by readLoop and
// read by the getters on the UI thread.
type procState struct {
	playing  playState
	paused   bool // user-initiated pause, shields cache events while paused
	position float64
	duration float64
	// bufferedEnd is the absolute end of the buffered range mpv reports via
	// demuxer-cache-state seekable-ranges (-1 while unknown). Buffered derives
	// the ahead-of-playhead amount from it.
	bufferedEnd float64
	ended       bool
	err         error
	muted       bool
	fullscr     bool
	volume      int // 0-100
	rate        float64
	posAt       time.Time // wall clock of the last position sample (extrapolation)
}

type mpvProc struct {
	exePath string
	ipcPath string

	// lifecycleMu guards cmd/conn/ipcPath/session/stderr. Take a snapshot under
	// the lock, never block or wait for replies while holding it.
	lifecycleMu sync.Mutex
	session     int64
	cmd         *exec.Cmd
	wait        *waiter
	conn        io.ReadWriteCloser
	stderr      *stderrTail

	sendMu    sync.Mutex
	responses chan response

	stateMu sync.Mutex
	state   procState
}

// New validates the mpv executable and returns an idle player.
func New(exePath string) (player.Player, error) {
	if _, err := os.Stat(exePath); err != nil {
		return nil, fmt.Errorf("mpv 可执行文件不可用: %w", err)
	}
	return &mpvProc{
		exePath:   exePath,
		responses: make(chan response, 16),
		state:     procState{playing: psStopped, duration: -1, bufferedEnd: -1, rate: 1, volume: 100},
	}, nil
}

func (p *mpvProc) Load(ctx context.Context, raw string, headers map[string]string) error {
	// Close any previous session (idempotent when there is none).
	_ = p.Close()
	ipcPath, err := newIPCPath()
	if err != nil {
		return err
	}
	args := buildArgs(raw, headers, ipcPath)

	// mpv failing to start (missing DLL, blocked, stale args) only explains
	// itself on stderr; capture its tail so those failures stay diagnosable.
	stderr := &stderrTail{max: mpvStderrLimit}
	cmd := exec.CommandContext(ctx, p.exePath, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	setupProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		cleanupIPC(ipcPath)
		return mpvLoadError(fmt.Errorf("启动 mpv 失败: %w", err), stderr.String())
	}
	p.lifecycleMu.Lock()
	p.stderr = stderr
	p.lifecycleMu.Unlock()

	w := startWaiter(cmd)

	conn, err := waitForIPC(dialIPC, ipcPath, ipcConnectTimeout, w)
	if err != nil {
		_ = cmd.Process.Kill()
		waitErr := w.wait()
		cleanupIPC(ipcPath)
		if errors.Is(err, errMPVExitedEarly) {
			err = mpvExitError(waitErr)
		}
		return mpvLoadError(err, stderr.String())
	}

	p.lifecycleMu.Lock()
	p.session++
	sess := p.session
	p.cmd = cmd
	p.wait = w
	p.conn = conn
	p.ipcPath = ipcPath
	p.lifecycleMu.Unlock()

	p.stateMu.Lock()
	p.state = procState{playing: psPlaying, duration: -1, bufferedEnd: -1, rate: 1, volume: 100}
	p.stateMu.Unlock()

	go p.readLoop(conn, sess)

	// Observe position, cache state, user pause, duration and the real
	// buffered range. Observation failures do not stop playback, so ignore.
	_ = p.send("observe_property", 0, "time-pos")
	_ = p.send("observe_property", 1, "paused-for-cache")
	_ = p.send("observe_property", 2, "pause")
	_ = p.send("observe_property", 3, "duration")
	_ = p.send("observe_property", 4, "demuxer-cache-state")
	_ = p.send("observe_property", 5, "fullscreen")
	return nil
}

// buildArgs constructs mpv's startup arguments.
func buildArgs(raw string, headers map[string]string, ipcPath string) []string {
	args := []string{
		"--idle=yes",
		"--input-ipc-server=" + ipcPath,
		"--keep-open=yes",
		// Bound each network operation so a dead stream fails fast instead of
		// wedging mpv's main loop and every IPC command behind it.
		"--network-timeout=15",
		"--force-window=yes",
		"--osc=yes",
	}
	for k, v := range headers {
		if strings.TrimSpace(v) == "" {
			continue
		}
		switch strings.ToLower(k) {
		case "user-agent":
			args = append(args, "--user-agent="+v)
		case "referer", "referrer":
			args = append(args, "--referrer="+v)
		default:
			args = append(args, "--http-header-fields="+k+": "+v)
		}
	}
	args = append(args, raw)
	return args
}

func (p *mpvProc) Play() error  { return p.send("set_property", "pause", false) }
func (p *mpvProc) Pause() error { return p.send("set_property", "pause", true) }
func (p *mpvProc) Seek(d time.Duration) error {
	return p.send("seek", d.Seconds(), "absolute")
}

func (p *mpvProc) SetRate(r float64) error {
	if err := p.send("set_property", "speed", r); err != nil {
		return err
	}
	p.stateMu.Lock()
	p.state.rate = r
	p.stateMu.Unlock()
	return nil
}

func (p *mpvProc) SetVolume(v float64) error {
	vol := int(v * 100)
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	if err := p.send("set_property", "volume", vol); err != nil {
		return err
	}
	p.stateMu.Lock()
	p.state.volume = vol
	p.stateMu.Unlock()
	return nil
}

func (p *mpvProc) SetMuted(m bool) error {
	if err := p.send("set_property", "mute", m); err != nil {
		return err
	}
	p.stateMu.Lock()
	p.state.muted = m
	p.stateMu.Unlock()
	return nil
}

func (p *mpvProc) SetFullscreen(f bool) error {
	return p.send("set_property", "fullscreen", f)
}

func (p *mpvProc) Fullscreen() bool {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.state.fullscr
}

func (p *mpvProc) Muted() bool {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.state.muted
}

func (p *mpvProc) Ready() bool {
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()
	return p.conn != nil
}

func (p *mpvProc) Playing() bool {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.state.playing == psPlaying
}

func (p *mpvProc) Ended() bool {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.state.ended
}

func (p *mpvProc) Buffering() bool {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.state.playing == psBuffering
}

func (p *mpvProc) Position() time.Duration {
	p.stateMu.Lock()
	pos := p.state.position
	posAt := p.state.posAt
	playing := p.state.playing == psPlaying
	rate := p.state.rate
	p.stateMu.Unlock()
	// mpv reports time-pos roughly once a second; extrapolate between samples
	// using wall clock so the progress bar moves smoothly without guessing.
	if playing && rate > 0 && !posAt.IsZero() {
		pos += time.Since(posAt).Seconds() * rate
	}
	return time.Duration(pos * float64(time.Second))
}

func (p *mpvProc) Duration() time.Duration {
	p.stateMu.Lock()
	d := p.state.duration
	p.stateMu.Unlock()
	if d <= 0 {
		return 0
	}
	return time.Duration(d * float64(time.Second))
}

func (p *mpvProc) Buffered() time.Duration {
	p.stateMu.Lock()
	end := p.state.bufferedEnd
	pos := p.state.position
	p.stateMu.Unlock()
	if end <= 0 || pos < 0 {
		return 0
	}
	ahead := end - pos
	if ahead <= 0 {
		return 0
	}
	return time.Duration(ahead * float64(time.Second))
}

func (p *mpvProc) Err() error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.state.err
}

func (p *mpvProc) Close() error {
	p.lifecycleMu.Lock()
	conn := p.conn
	cmd := p.cmd
	w := p.wait
	ipcPath := p.ipcPath
	p.conn = nil
	p.cmd = nil
	p.wait = nil
	p.ipcPath = ""
	p.lifecycleMu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	_ = w.wait()
	if ipcPath != "" {
		cleanupIPC(ipcPath)
	}
	return nil
}

// send serialises one command and waits for its reply. readLoop routes command
// replies to p.responses and asynchronous events to the state; the two streams
// never interfere.
func (p *mpvProc) send(args ...any) error {
	p.sendMu.Lock()
	defer p.sendMu.Unlock()

	p.lifecycleMu.Lock()
	c := p.conn
	sess := p.session
	p.lifecycleMu.Unlock()
	if c == nil {
		return errors.New("mpvproc: 尚未 Load")
	}
	if _, err := c.Write([]byte(encodeCommand(args))); err != nil {
		return err
	}

	timer := time.NewTimer(cmdResponseTimeout)
	defer timer.Stop()
	for {
		select {
		case r := <-p.responses:
			if r.session != sess {
				continue // stale reply from a previous session
			}
			var resp struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(r.data, &resp); err != nil {
				return fmt.Errorf("解析 mpv 应答失败: %w", err)
			}
			if resp.Error != "" && resp.Error != "success" {
				return errors.New(resp.Error)
			}
			return nil
		case <-timer.C:
			// Timeout means this session's connection is broken. Only tear down
			// if p.conn is still the one we wrote to; a concurrent Load may have
			// already started a fresh session.
			p.lifecycleMu.Lock()
			same := p.conn == c
			stderr := p.stderr
			var conn io.ReadWriteCloser
			var cmd *exec.Cmd
			var w *waiter
			var ipcPath string
			if same {
				conn = p.conn
				cmd = p.cmd
				w = p.wait
				ipcPath = p.ipcPath
				p.conn = nil
				p.cmd = nil
				p.wait = nil
				p.ipcPath = ""
			}
			p.lifecycleMu.Unlock()

			if same {
				if conn != nil {
					_ = conn.Close()
				}
				if cmd != nil && cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				_ = w.wait()
				if ipcPath != "" {
					cleanupIPC(ipcPath)
				}
				p.stateMu.Lock()
				p.state.playing = psStopped
				p.stateMu.Unlock()
			}
			err := errors.New("mpvproc: 命令应答超时")
			if stderr != nil {
				err = mpvLoadError(err, stderr.String())
			}
			return err
		}
	}
}

// readLoop is the only reader of the IPC connection. Command replies and
// async events arrive interleaved on one stream, so a single reader must
// demultiplex them.
func (p *mpvProc) readLoop(conn io.ReadWriteCloser, sess int64) {
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var probe struct {
			Event string `json:"event"`
		}
		_ = json.Unmarshal(line, &probe)
		if probe.Event != "" {
			if paused, ok := parsePauseProperty(line); ok {
				p.stateMu.Lock()
				p.state.paused = paused
				if paused {
					p.state.playing = psPaused
				} else if p.state.playing == psPaused {
					p.state.playing = psPlaying
				}
				p.stateMu.Unlock()
				continue
			}
			if evt, ok := parseEvent(line); ok {
				p.applyEvent(evt)
			}
			continue
		}
		// Command reply. Scanner reuses its buffer, so copy before enqueueing.
		reply := append([]byte(nil), line...)
		select {
		case p.responses <- response{session: sess, data: reply}:
		default:
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		log.Printf("mpvproc: 读取 mpv IPC 中断: %v", err)
	}
}

func (p *mpvProc) applyEvent(evt mpvEvent) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	switch evt.kind {
	case evPosition:
		p.state.position = evt.position
		p.state.posAt = time.Now()
	case evBuffering:
		if !p.state.paused {
			p.state.playing = psBuffering
		}
	case evPlaying:
		if !p.state.paused {
			p.state.playing = psPlaying
		}
	case evDuration:
		p.state.duration = evt.duration
	case evBuffered:
		p.state.bufferedEnd = evt.buffered
	case evFullscreen:
		p.state.fullscr = evt.fullscreen
	case evEOF:
		p.state.ended = true
		p.state.playing = psStopped
	case evError:
		p.state.err = evt.err
		p.state.playing = psStopped
	}
}
