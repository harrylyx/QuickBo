package player

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckTellsMediaFromPages(t *testing.T) {
	bodies := map[string]string{
		"/ok.m3u8":     "#EXTM3U\n#EXTINF:10,\nseg0.ts\n",
		"/master.m3u8": "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1,RESOLUTION=1280x720\nlow.m3u8\n",
		"/empty.m3u8":  "#EXTM3U\n#EXT-X-ENDLIST\n",
		"/page":        "<!DOCTYPE html><html><body>播放器</body></html>",
		"/api":         `{"code":0,"msg":"ok"}`,
		"/blank":       "",
		"/video.mp4":   "\x00\x00\x00\x20ftypisom",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gone" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(bodies[r.URL.Path]))
	}))
	defer srv.Close()
	for path, want := range map[string]string{
		"/ok.m3u8": "", "/master.m3u8": "", "/video.mp4": "",
		"/empty.m3u8": "播放列表是空的", "/page": "网页", "/api": "接口数据",
		"/blank": "没有返回内容", "/gone": "打不开",
	} {
		err := Check(context.Background(), srv.URL+path, nil)
		if want == "" && err != nil {
			t.Errorf("%s: %v, want playable", path, err)
		}
		if want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Errorf("%s: %v, want %q", path, err, want)
		}
	}
	if err := Check(context.Background(), "rtmp://example.com/live", nil); err != nil {
		t.Errorf("rtmp should pass unchecked: %v", err)
	}
}

func TestCheckTimeoutIsNotAVerdict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := Check(ctx, srv.URL+"/slow.m3u8", nil); err != ErrCheckTimeout {
		t.Fatalf("got %v, want ErrCheckTimeout", err)
	}
}
