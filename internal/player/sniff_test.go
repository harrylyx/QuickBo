package player

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnwrapPlayerPageAndPlaylist(t *testing.T) {
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/play/abc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<script>const vid = '%s/play/abc/index.m3u8';</script>`, srv.URL)
	})
	mux.HandleFunc("/play/abc/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte("#EXTM3U\n#EXTINF:1,\nseg.ts\n"))
	})
	mux.HandleFunc("/bare", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html>player</html>`))
	})
	mux.HandleFunc("/bare/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#EXTM3U\n"))
	})
	mux.HandleFunc("/live.php", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte("#EXTM3U\n#EXTINF:-1,News\nhttp://cdn/live.ts\n"))
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	u, ok := Unwrap(context.Background(), srv.URL+"/play/abc", nil)
	if !ok || !strings.HasSuffix(u, "/play/abc/index.m3u8") {
		t.Fatalf("page unwrap %q ok %v", u, ok)
	}
	u, ok = Unwrap(context.Background(), srv.URL+"/bare", nil)
	if !ok || !strings.HasSuffix(u, "/bare/index.m3u8") {
		t.Fatalf("sibling unwrap %q ok %v", u, ok)
	}
	u, ok = Unwrap(context.Background(), srv.URL+"/live.php", nil)
	if !ok || u != srv.URL+"/live.php" {
		t.Fatalf("playlist body %q ok %v", u, ok)
	}
	if _, ok := Unwrap(context.Background(), "https://cdn.example/a.m3u8", nil); ok {
		t.Fatal("direct media should not be fetched")
	}
}
