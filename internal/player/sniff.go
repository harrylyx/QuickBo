package player

import (
	"bytes"
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/harrylyx/QuickBo/internal/httpx"
)

// Unwrap turns a player-page address into something the media engine can
// open. Direct media URLs are left alone. An address that already returns
// an HLS playlist is kept. Otherwise the HTML is scanned for an m3u8, and
// a /play/<id> style path is also tried as /play/<id>/index.m3u8.
// ok is true when the returned URL is known to be playable media.
func Unwrap(ctx context.Context, raw string, headers map[string]string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || IsDirect(raw) {
		return raw, false
	}
	body, ct, err := httpx.Get(ctx, nil, raw, sniffHeaders(headers), 512<<10)
	if err == nil && playlist(body, ct) {
		return raw, true
	}
	if err == nil {
		if found := extractM3U8(body, raw); found != "" && found != raw {
			return found, true
		}
	}
	if alt := indexSibling(raw); alt != "" {
		b, ct, err := httpx.Get(ctx, nil, alt, sniffHeaders(headers), 64<<10)
		if err == nil && playlist(b, ct) {
			return alt, true
		}
	}
	return raw, false
}

func sniffHeaders(headers map[string]string) map[string]string {
	out := map[string]string{"User-Agent": httpx.BrowserUA}
	for k, v := range headers {
		if strings.TrimSpace(k) == "" || v == "" {
			continue
		}
		if strings.EqualFold(k, "User-Agent") {
			continue
		}
		out[k] = v
	}
	return out
}

func playlist(body []byte, contentType string) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "mpegurl") || strings.Contains(ct, "mpeg-url") {
		return true
	}
	b := bytes.TrimSpace(body)
	return bytes.HasPrefix(b, []byte("#EXTM3U"))
}

var (
	absM3U8 = regexp.MustCompile(`(?i)https?://[^\s"'<>]+?\.m3u8[^\s"'<>]*`)
	relM3U8 = regexp.MustCompile(`(?i)(?:^|['"\s=])(/[^\s"'<>]+?\.m3u8[^\s"'<>]*)`)
)

func extractM3U8(body []byte, page string) string {
	if m := absM3U8.Find(body); m != nil {
		return cleanURL(string(m))
	}
	base, err := url.Parse(page)
	if err != nil {
		return ""
	}
	if m := relM3U8.FindSubmatch(body); m != nil {
		ref, err := url.Parse(cleanURL(string(m[1])))
		if err != nil {
			return ""
		}
		return base.ResolveReference(ref).String()
	}
	return ""
}

func cleanURL(s string) string {
	return strings.TrimRight(s, `).,;'"\`)
}

// indexSibling is the conventional real playlist behind a player path
// whose last segment has no file extension.
func indexSibling(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	path := u.Path
	if path == "" || path == "/" {
		return ""
	}
	seg := path[strings.LastIndex(path, "/")+1:]
	if seg == "" || strings.Contains(seg, ".") {
		return ""
	}
	u.Path = strings.TrimRight(path, "/") + "/index.m3u8"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
