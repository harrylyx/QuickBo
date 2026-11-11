// Package player opens streams with an external mpv process and exposes the
// playback state the UI needs. The video renders in mpv's own window; this
// process talks to mpv over JSON IPC to drive it and to read the real
// position and buffered range.
package player

import (
	"context"
	"encoding/json"
	"image"
	"net/url"
	"strings"
	"time"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/httpx"
)

// Player is a live playback session. It is driven from the UI thread; the
// getters are cheap mutex reads so the view can poll them every frame.
type Player interface {
	Load(ctx context.Context, raw string, headers map[string]string) error
	Play() error
	Pause() error
	Seek(d time.Duration) error
	SetRate(r float64) error
	SetVolume(v float64) error
	SetMuted(m bool) error
	SetFullscreen(f bool) error
	Fullscreen() bool
	Muted() bool
	Ready() bool
	Playing() bool
	Ended() bool
	Buffering() bool
	// Position is the current playback position. Duration is the total length
	// (0 while unknown, e.g. live). Buffered is the real buffered range ahead
	// of the playhead (0 while unknown).
	Position() time.Duration
	Duration() time.Duration
	Buffered() time.Duration
	// Err is the last terminal playback error, cleared by Load.
	Err() error
	Close() error
}

// VideoSource is a Player that renders its video into frames the app draws
// itself, rather than into a window of its own.
type VideoSource interface {
	// SetVideoSize tells the player the size, in device pixels, the video
	// is shown at; frames are rendered at that size.
	SetVideoSize(w, h int)
	// TakeFrame returns the newest frame not yet taken, or nil. Its pixels
	// stay unchanged until the next frame is taken, so it can be shown
	// without a copy.
	TakeFrame() *image.RGBA
}

// IsDirect reports whether raw already looks like a media address, so it
// should not be sent through a JSON parse.
func IsDirect(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false
	}
	low := strings.ToLower(s)
	for _, p := range []string{"rtmp://", "rtsp://", "udp://", "mms://", "rtp://"} {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	path := low
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	for _, ext := range []string{".m3u8", ".mp4", ".m4v", ".flv", ".mkv", ".webm", ".mov", ".ts", ".mpd", ".m3u", ".avi"} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// ParseAttempt is the outcome of asking one online parser for a stream. The
// caller uses it to keep a per-parser track record (success rate and latency)
// so well-behaved parsers are tried first.
type ParseAttempt struct {
	Parser  string        // the parser's URL, its stable identity
	Latency time.Duration // round-trip for this one parser
	OK      bool          // whether it produced a usable stream
}

// ApplyParses asks type=1 JSON parsers for a real stream URL, in the given
// order, and reports how each parser performed. Other parse types (page
// sniffers, jar) are skipped. It stops at the first usable result; ok is false
// when nothing usable came back.
func ApplyParses(ctx context.Context, raw string, parses []config.ParseItem) (attempts []ParseAttempt, stream string, headers map[string]string, ok bool) {
	if IsDirect(raw) || strings.TrimSpace(raw) == "" {
		return nil, raw, nil, false
	}
	for _, p := range parses {
		if p.Type != 1 || strings.TrimSpace(p.URL) == "" {
			continue
		}
		u := joinParse(p.URL, raw)
		start := time.Now()
		body, _, err := httpx.Get(ctx, nil, u, map[string]string{"User-Agent": httpx.TVBoxUA}, 2<<20)
		latency := time.Since(start)
		if err != nil {
			attempts = append(attempts, ParseAttempt{Parser: p.URL, Latency: latency, OK: false})
			continue
		}
		found, hdr, got := extractPlay(body)
		if got && found != "" && found != raw {
			attempts = append(attempts, ParseAttempt{Parser: p.URL, Latency: latency, OK: true})
			return attempts, found, hdr, true
		}
		attempts = append(attempts, ParseAttempt{Parser: p.URL, Latency: latency, OK: false})
	}
	return attempts, raw, nil, false
}

func joinParse(base, raw string) string {
	if strings.Contains(base, "{url}") {
		return strings.ReplaceAll(base, "{url}", url.QueryEscape(raw))
	}
	if strings.HasSuffix(base, "=") {
		return base + url.QueryEscape(raw)
	}
	if strings.Contains(base, "?") {
		return base + "&url=" + url.QueryEscape(raw)
	}
	return base + "?url=" + url.QueryEscape(raw)
}

func extractPlay(body []byte) (string, map[string]string, bool) {
	var v any
	if json.Unmarshal(bytesTrim(body), &v) != nil {
		return "", nil, false
	}
	headers := map[string]string{}
	if m, ok := v.(map[string]any); ok {
		headers = headerOf(m["header"])
		if len(headers) == 0 {
			headers = headerOf(m["headers"])
		}
	}
	if u := findURL(v); u != "" {
		return u, headers, true
	}
	return "", nil, false
}

func findURL(v any) string {
	switch t := v.(type) {
	case map[string]any:
		for _, key := range []string{"url", "playUrl", "play_url", "m3u8"} {
			if s, ok := t[key].(string); ok && strings.Contains(s, "://") {
				return strings.TrimSpace(s)
			}
		}
		for _, key := range []string{"data", "info"} {
			if u := findURL(t[key]); u != "" {
				return u
			}
		}
	case []any:
		for _, item := range t {
			if u := findURL(item); u != "" {
				return u
			}
		}
	}
	return ""
}

func headerOf(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]string{}
	for k, val := range m {
		if s, ok := val.(string); ok && s != "" {
			out[k] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
