package player

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/harrylyx/QuickBo/internal/httpx"
	"golang.org/x/net/idna"
)

const probeChunk = 128 << 10

var resolutionTag = regexp.MustCompile(`(?i)RESOLUTION\s*=\s*(\d+)\s*x\s*(\d+)`)

// Measure reads the picture size of a media address. HLS master playlists
// contribute their largest RESOLUTION. MP4 and MOV contribute the video
// track header. A page that only links to one of those is followed once.
// Zero means the size could not be read.
func Measure(ctx context.Context, raw string, headers map[string]string) (int, int) {
	return measure(ctx, raw, headers, 0)
}

func measure(ctx context.Context, raw string, headers map[string]string, depth int) (int, int) {
	raw = strings.TrimSpace(raw)
	if raw == "" || ctx.Err() != nil {
		return 0, 0
	}
	body, _, err := fetchPart(ctx, raw, headers, 0, probeChunk)
	if err != nil || len(body) == 0 {
		return 0, 0
	}
	if w, h, ok := playlistSize(body); ok {
		return w, h
	}
	if w, h := mp4Size(body); w > 0 || h > 0 {
		return w, h
	}
	if at := moovOffset(body); at > int64(len(body)) {
		tail, _, err := fetchPart(ctx, raw, headers, at, probeChunk)
		if err == nil {
			if w, h := mp4Size(tail); w > 0 || h > 0 {
				return w, h
			}
		}
	}
	if depth == 0 {
		if found := extractM3U8(body, raw); found != "" && found != raw {
			return measure(ctx, found, headers, depth+1)
		}
	}
	return 0, 0
}

func playlistSize(body []byte) (int, int, bool) {
	if !playlist(body, "") && !bytesContainsFold(body, []byte("#EXT-X-STREAM-INF")) {
		return 0, 0, false
	}
	bestW, bestH := 0, 0
	for _, m := range resolutionTag.FindAllSubmatch(body, -1) {
		w, _ := strconv.Atoi(string(m[1]))
		h, _ := strconv.Atoi(string(m[2]))
		if h > bestH || (h == bestH && w > bestW) {
			bestW, bestH = w, h
		}
	}
	if bestW == 0 && bestH == 0 {
		return 0, 0, playlist(body, "")
	}
	return bestW, bestH, true
}

func bytesContainsFold(b, sub []byte) bool {
	return strings.Contains(strings.ToUpper(string(b)), strings.ToUpper(string(sub)))
}

func fetchPart(ctx context.Context, raw string, headers map[string]string, start, n int64) ([]byte, int64, error) {
	raw, err := requestURL(raw)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", httpx.TVBoxUA)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, start+n-1))
	for k, v := range headers {
		if strings.TrimSpace(k) == "" || v == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, n))
	if err != nil {
		return nil, 0, err
	}
	return body, contentTotal(resp, start, int64(len(body))), nil
}

func contentTotal(resp *http.Response, start, got int64) int64 {
	cr := resp.Header.Get("Content-Range")
	if i := strings.LastIndex(cr, "/"); i >= 0 {
		if n, err := strconv.ParseInt(cr[i+1:], 10, 64); err == nil {
			return n
		}
	}
	if resp.ContentLength > 0 && resp.StatusCode == http.StatusOK {
		return resp.ContentLength
	}
	return start + got
}

func requestURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return u.String(), nil
	}
	host, err := idna.Lookup.ToASCII(u.Hostname())
	if err != nil || host == "" {
		return u.String(), err
	}
	if port := u.Port(); port != "" {
		u.Host = host + ":" + port
	} else {
		u.Host = host
	}
	return u.String(), nil
}

func mp4Size(data []byte) (int, int) {
	bestW, bestH := 0, 0
	walkBoxes(data, func(kind string, payload []byte) {
		if kind != "tkhd" {
			return
		}
		w, h := tkhdSize(payload)
		if h > bestH || (h == bestH && w > bestW) {
			bestW, bestH = w, h
		}
	})
	return bestW, bestH
}

func moovOffset(data []byte) int64 {
	var offset int64
	for offset+8 <= int64(len(data)) {
		size, kind, header := boxHeader(data[offset:])
		if size < int64(header) {
			return 0
		}
		if kind == "moov" {
			return offset
		}
		if kind == "mdat" && offset+size > int64(len(data)) {
			return offset + size
		}
		if offset+size > int64(len(data)) {
			return 0
		}
		offset += size
	}
	return 0
}

func walkBoxes(data []byte, visit func(kind string, payload []byte)) {
	for offset := 0; offset+8 <= len(data); {
		size, kind, header := boxHeader(data[offset:])
		if size < int64(header) {
			return
		}
		end := offset + int(size)
		if end > len(data) || end < offset {
			end = len(data)
		}
		payload := data[offset+header : end]
		visit(kind, payload)
		if containerBox(kind) && end > offset+header {
			walkBoxes(payload, visit)
		}
		if end <= offset || end > len(data) {
			return
		}
		offset = end
	}
}

func boxHeader(b []byte) (size int64, kind string, header int) {
	if len(b) < 8 {
		return 0, "", 0
	}
	size = int64(binary.BigEndian.Uint32(b[:4]))
	kind = string(b[4:8])
	header = 8
	if size == 1 {
		if len(b) < 16 {
			return 0, kind, 0
		}
		size = int64(binary.BigEndian.Uint64(b[8:16]))
		header = 16
	}
	if size == 0 {
		size = int64(len(b))
	}
	return size, kind, header
}

func containerBox(kind string) bool {
	switch kind {
	case "moov", "trak", "mdia", "minf", "stbl", "edts":
		return true
	default:
		return false
	}
}

func tkhdSize(payload []byte) (int, int) {
	if len(payload) < 4 {
		return 0, 0
	}
	version := payload[0]
	off := 4
	if version == 1 {
		off += 32
	} else {
		off += 20
	}
	off += 8 + 2 + 2 + 2 + 2 + 36
	if off+8 > len(payload) {
		return 0, 0
	}
	w := int(binary.BigEndian.Uint32(payload[off:off+4]) >> 16)
	h := int(binary.BigEndian.Uint32(payload[off+4:off+8]) >> 16)
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	return w, h
}
