// Package player opens streams. Native playback uses mygo's system media
// engine (package ui/video): H.264/AAC in MP4 on macOS, Windows and Linux,
// and HLS on macOS and Windows. Linux HLS and anything the system decoder
// rejects are handed to mpv, which is also available as an explicit choice.
package player

import (
	"context"
	"encoding/json"
	"net/url"
	"os/exec"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/httpx"
)

// OpenNative opens a URL with the system media framework. The player
// loads in the background; ErrUnsupported means this machine has no
// framework (for example Linux without GStreamer).
func OpenNative(raw string) (*ui.VideoPlayer, error) {
	return video.OpenURL(raw)
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

// LaunchMPV starts mpv in the background. headers are passed as referrer,
// user agent and http-header-fields. A missing mpv returns an error.
func LaunchMPV(raw string, headers map[string]string) error {
	bin, err := exec.LookPath("mpv")
	if err != nil {
		return err
	}
	args := []string{"--force-window=yes", "--keep-open=yes", "--no-terminal"}
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
	cmd := exec.Command(bin, args...)
	hideWindow(cmd)
	return cmd.Start()
}

// ApplyParses asks type=1 JSON parsers for a real stream URL. Other parse
// types (page sniffers, jar) are skipped. ok is false when nothing usable
// came back.
func ApplyParses(ctx context.Context, raw string, parses []config.ParseItem) (string, map[string]string, bool) {
	if IsDirect(raw) || strings.TrimSpace(raw) == "" {
		return raw, nil, false
	}
	for _, p := range parses {
		if p.Type != 1 || strings.TrimSpace(p.URL) == "" {
			continue
		}
		u := joinParse(p.URL, raw)
		body, _, err := httpx.Get(ctx, nil, u, map[string]string{"User-Agent": httpx.TVBoxUA}, 2<<20)
		if err != nil {
			continue
		}
		if found, hdr, ok := extractPlay(body); ok && found != "" && found != raw {
			return found, hdr, true
		}
	}
	return raw, nil, false
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
