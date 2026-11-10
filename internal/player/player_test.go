package player

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harrylyx/mybox/internal/config"
)

func TestIsDirect(t *testing.T) {
	if !IsDirect("http://a/b.m3u8?token=1") || !IsDirect("rtmp://live/a") || !IsDirect("http://a/v.mp4") {
		t.Fatal("expected direct")
	}
	if IsDirect("http://site/play/123") || IsDirect("") {
		t.Fatal("expected not direct")
	}
}

func TestApplyParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("url") != "http://page/1" {
			http.Error(w, "bad", 400)
			return
		}
		w.Write([]byte(`{"code":200,"data":{"url":"http://cdn/a.m3u8"},"header":{"Referer":"http://page/"}}`))
	}))
	defer srv.Close()
	parses := []config.ParseItem{
		{Name: "嗅探", Type: 0, URL: "http://unused"},
		{Name: "json", Type: 1, URL: srv.URL + "?url="},
	}
	u, hdr, ok := ApplyParses(context.Background(), "http://page/1", parses)
	if !ok || u != "http://cdn/a.m3u8" {
		t.Fatalf("got %s ok %v", u, ok)
	}
	if hdr["Referer"] != "http://page/" {
		t.Fatalf("header %+v", hdr)
	}
	if _, _, ok := ApplyParses(context.Background(), "http://cdn/a.m3u8", parses); ok {
		t.Fatal("direct url should not be parsed")
	}
}
