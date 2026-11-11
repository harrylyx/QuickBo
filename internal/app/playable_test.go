package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

// streamFake resolves every episode to the episode's own URL.
type streamFake struct{ cmsFake }

func (streamFake) Resolve(_ context.Context, _ string, ep model.Episode) (model.Playable, error) {
	return model.Playable{URL: ep.URL}, nil
}

func detailWith(urls ...string) model.Detail {
	var d model.Detail
	for i, u := range urls {
		d.Lines = append(d.Lines, model.Line{Name: string(rune('A' + i)), Episodes: []model.Episode{{Name: "1", URL: u}}})
	}
	return d
}

func TestMeasureOfferTellsWhetherASourcePlays(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.m3u8":
			w.Write([]byte("#EXTM3U\n#EXTINF:10,\nseg0.ts\n"))
		case "/page.m3u8":
			w.Write([]byte("<!DOCTYPE html><html><body>广告页</body></html>"))
		case "/slow.m3u8":
			time.Sleep(400 * time.Millisecond)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	a := New()
	a.settingsPath = ""
	probe := func(timeout time.Duration, d model.Detail) offerProbe {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return a.measureOffer(ctx, streamFake{}, d, nil)
	}
	if p := probe(5*time.Second, detailWith(srv.URL+"/page.m3u8", srv.URL+"/ok.m3u8")); p.unplayable {
		t.Fatalf("one good line must keep the source playable: %+v", p)
	}
	p := probe(5*time.Second, detailWith(srv.URL+"/page.m3u8", srv.URL+"/gone.m3u8"))
	if !p.unplayable || !strings.Contains(p.reason, "网页") {
		t.Fatalf("all lines dead: %+v", p)
	}
	if p := probe(5*time.Second, model.Detail{}); !p.unplayable || p.reason != "没有可播放的剧集" {
		t.Fatalf("no episodes: %+v", p)
	}
	if p := probe(100*time.Millisecond, detailWith(srv.URL+"/slow.m3u8")); p.unplayable {
		t.Fatalf("a timeout must not mark a source unplayable: %+v", p)
	}
}

func TestUnplayableOffersSortLastAndCannotBePicked(t *testing.T) {
	a := New()
	a.settingsPath = ""
	good := model.Detail{Title: "好源"}
	a.offers = []playOffer{
		{SiteKey: "bad", SiteName: "坏源", Clarity: 50, Unplayable: true, Reason: "返回的是网页，不是视频"},
		{SiteKey: "ok", SiteName: "好源", Clarity: 20, Detail: &good},
	}
	a.sortOffers()
	if a.offers[0].SiteKey != "ok" {
		t.Fatalf("unplayable sorted first: %+v", a.offers)
	}
	a.offerPick = 0
	a.applyOffer()
	a.pickOffer(1)
	if a.offerPick != 0 || a.siteKey != "ok" {
		t.Fatalf("picked an unplayable source: pick=%d site=%s", a.offerPick, a.siteKey)
	}
	// Even when it is the pick, applyOffer shows a playable source.
	a.offerPick = 1
	a.applyOffer()
	if a.siteKey != "ok" {
		t.Fatalf("applyOffer showed the unplayable source")
	}
}

func TestDetailPageGreysOutUnplayableSources(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.screen = screenDetail
	good := model.Detail{Title: "示例影片", Lines: []model.Line{{Name: "线路", Episodes: []model.Episode{{Name: "1", URL: "https://x/1.m3u8"}}}}}
	a.detail = &good
	a.offers = []playOffer{
		{SiteKey: "ok", SiteName: "好源", Clarity: 40, Detail: &good},
		{SiteKey: "bad", SiteName: "坏源", Clarity: 40, Unplayable: true, Reason: "地址打不开（HTTP 404）"},
	}
	tt := ui.NewTester(a.View, 1200, 900)
	if !tt.HasText("无法播放") || !tt.HasText("坏源") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if !tt.HasText("1 个无法播放") {
		t.Fatalf("missing summary: %q", tt.Texts())
	}
	_ = tt.Click("坏源")
	if a.offerPick != 0 || a.siteKey == "bad" {
		t.Fatalf("clicking an unplayable source picked it")
	}
}
