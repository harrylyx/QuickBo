package player

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
)

// checkChunk is how much of an address Check reads: enough for a playlist's
// first segments or a page's <html>, without downloading the video.
const checkChunk = 16 << 10

// ErrCheckTimeout means Check ran out of time before it could tell. A slow
// source is not proof that a stream cannot play.
var ErrCheckTimeout = errors.New("检测超时")

// Check reads the start of a media address and reports why it cannot play,
// or nil if it looks like media. It catches what players fail on later: an
// address that does not open, an empty answer, an HTML page or API JSON
// where a stream should be, and a playlist with nothing in it. Addresses it
// cannot read over HTTP (rtmp://, rtsp://, local files) pass unchecked.
func Check(ctx context.Context, raw string, headers map[string]string) error {
	raw = strings.TrimSpace(raw)
	low := strings.ToLower(raw)
	if raw == "" {
		return errors.New("没有播放地址")
	}
	if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
		return nil
	}
	body, _, err := fetchPart(ctx, raw, headers, 0, checkChunk)
	if err != nil {
		if ctx.Err() != nil {
			return ErrCheckTimeout
		}
		return fmt.Errorf("地址打不开（%v）", err)
	}
	return checkBody(body)
}

func checkBody(body []byte) error {
	b := bytes.TrimSpace(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")))
	if len(b) == 0 {
		return errors.New("地址没有返回内容")
	}
	head := bytes.ToLower(b[:min(len(b), 1024)])
	switch {
	case bytes.HasPrefix(head, []byte("#extm3u")):
		if !bytes.Contains(b, []byte("#EXTINF")) && !bytes.Contains(b, []byte("#EXT-X-STREAM-INF")) {
			return errors.New("播放列表是空的")
		}
		return nil
	case bytes.HasPrefix(head, []byte("<")) &&
		(bytes.Contains(head, []byte("<html")) || bytes.Contains(head, []byte("<!doctype")) || bytes.Contains(head, []byte("<head"))):
		return errors.New("返回的是网页，不是视频")
	case bytes.HasPrefix(head, []byte("{")) || bytes.HasPrefix(head, []byte("[")):
		return errors.New("返回的是接口数据，不是视频")
	}
	return nil
}
