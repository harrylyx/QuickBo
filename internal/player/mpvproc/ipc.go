package mpvproc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
)

// ipcRetryInterval is the pause between IPC connect attempts: mpv creates its
// endpoint a moment after starting, so we retry instead of failing once.
const ipcRetryInterval = 50 * time.Millisecond

// errMPVExitedEarly means mpv exited before its IPC endpoint appeared.
var errMPVExitedEarly = errors.New("mpv 启动后立即退出")

// dialFunc connects to mpv's IPC endpoint (Unix socket / Windows named pipe).
type dialFunc func(string) (io.ReadWriteCloser, error)

// waitForIPC retries the dial until the timeout, while watching for early
// process exit so a broken mpv fails with its real cause, not a generic
// "endpoint missing" error.
func waitForIPC(dial dialFunc, path string, timeout time.Duration, w *waiter) (io.ReadWriteCloser, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		conn, err := dial(path)
		if err == nil {
			return conn, nil
		}
		lastErr = err

		select {
		case <-w.exited():
			return nil, errMPVExitedEarly
		default:
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("连接 mpv IPC 失败: %w", lastErr)
		}
		time.Sleep(ipcRetryInterval)
	}
}

// encodeCommand encodes an mpv JSON IPC command as a newline-terminated line.
func encodeCommand(args []any) string {
	b, err := json.Marshal(map[string]any{"command": args})
	if err != nil {
		return "{}\n"
	}
	return string(b) + "\n"
}

type mpvEventKind int

const (
	evNone mpvEventKind = iota
	evPosition
	evBuffering // paused-for-cache became true
	evPlaying   // paused-for-cache became false
	evDuration
	evBuffered
	evFullscreen
	evEOF
	evError
)

// mpvEvent is a decoded async event that readLoop folds into procState.
type mpvEvent struct {
	kind       mpvEventKind
	position   float64
	duration   float64
	buffered   float64 // absolute end of the buffered range (seekable-ranges max end)
	fullscreen bool
	err        error
}

// parseEvent decodes the events the player cares about. Everything else
// (idle, start-file, quit, redirect, …) returns ok=false.
func parseEvent(line []byte) (mpvEvent, bool) {
	var raw struct {
		Event  string          `json:"event"`
		Name   string          `json:"name"`
		Data   json.RawMessage `json:"data"`
		Reason string          `json:"reason"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return mpvEvent{}, false
	}
	switch {
	case raw.Event == "end-file":
		switch raw.Reason {
		case "eof":
			return mpvEvent{kind: evEOF}, true
		case "error":
			return mpvEvent{kind: evError, err: fmt.Errorf("mpv 播放出错: %s", raw.Reason)}, true
		}
		// stop/quit/redirect are not failures; ignore them so they cannot
		// trigger a spurious auto-switch.
		return mpvEvent{}, false
	case raw.Event == "property-change":
		switch raw.Name {
		case "time-pos":
			f, err := strconv.ParseFloat(string(raw.Data), 64)
			if err != nil {
				return mpvEvent{}, false
			}
			return mpvEvent{kind: evPosition, position: f}, true
		case "paused-for-cache":
			var paused *bool
			if err := json.Unmarshal(raw.Data, &paused); err != nil || paused == nil {
				return mpvEvent{}, false
			}
			if *paused {
				return mpvEvent{kind: evBuffering}, true
			}
			return mpvEvent{kind: evPlaying}, true
		case "duration":
			f, ok := parseNullableFloat(raw.Data)
			if !ok {
				return mpvEvent{}, false
			}
			return mpvEvent{kind: evDuration, duration: f}, true
		case "demuxer-cache-state":
			end, ok := parseCacheEnd(raw.Data)
			if !ok {
				return mpvEvent{}, false
			}
			return mpvEvent{kind: evBuffered, buffered: end}, true
		case "fullscreen":
			var on *bool
			if err := json.Unmarshal(raw.Data, &on); err != nil || on == nil {
				return mpvEvent{}, false
			}
			return mpvEvent{kind: evFullscreen, fullscreen: *on}, true
		}
	}
	return mpvEvent{}, false
}

// parseNullableFloat reads a JSON number or null. null yields ok=false so the
// caller keeps the unknown (-1) default.
func parseNullableFloat(data json.RawMessage) (float64, bool) {
	s := string(data)
	if s == "null" || s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// parseCacheEnd reads the end of the buffered range mpv reports in
// demuxer-cache-state's seekable-ranges: the furthest point the demuxer can
// play without hitting the network again. No ranges (as for a local file)
// yields ok=false so the player keeps the unknown default.
func parseCacheEnd(data json.RawMessage) (float64, bool) {
	var st struct {
		Ranges []struct {
			End float64 `json:"end"`
		} `json:"seekable-ranges"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return 0, false
	}
	end := -1.0
	for _, r := range st.Ranges {
		if r.End > end {
			end = r.End
		}
	}
	if end < 0 {
		return 0, false
	}
	return end, true
}

// parsePauseProperty reads the pause property. pause never maps to a buffering
// or playing signal (user pause is not buffering); it only lets readLoop
// shield cache events while the user has paused.
func parsePauseProperty(line []byte) (paused bool, ok bool) {
	var raw struct {
		Event string          `json:"event"`
		Name  string          `json:"name"`
		Data  json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return false, false
	}
	if raw.Event != "property-change" || raw.Name != "pause" {
		return false, false
	}
	var value *bool
	if err := json.Unmarshal(raw.Data, &value); err != nil || value == nil {
		return false, false
	}
	return *value, true
}
