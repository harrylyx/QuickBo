package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/live"
	"github.com/harrylyx/QuickBo/internal/source"
)

func TestAggregateLivesMergesSameNames(t *testing.T) {
	got := aggregateLives([]liveSnap{
		{Name: "甲", URL: "http://list/a", Chs: []live.Channel{
			{Name: " CCTV1 ", URL: "http://a/1", Group: "央视"},
			{Name: "湖南卫视", URL: "http://a/h"},
			{Name: "新闻", URL: "http://same"},
			{Name: "CCTV-1", URL: "http://a/dash"},
		}},
		{Name: "乙", URL: "http://list/b", Chs: []live.Channel{
			{Name: "CCTV1", URL: "http://b/1", Group: "央视频"},
			{Name: "新闻", URL: "http://same"},
		}},
	})
	by := map[string]liveAgg{}
	for _, g := range got {
		by[g.Name] = g
	}
	cctv := by["CCTV1"]
	if cctv.Sources != 2 || len(cctv.Lines) != 2 || cctv.Lines[0].URL != "http://a/1" || cctv.Lines[1].URL != "http://b/1" {
		t.Fatalf("CCTV1 %+v", cctv)
	}
	if by["湖南卫视"].Sources != 1 || len(by["湖南卫视"].Lines) != 1 {
		t.Fatalf("湖南卫视 %+v", by["湖南卫视"])
	}
	if by["新闻"].Sources != 1 || len(by["新闻"].Lines) != 1 || by["新闻"].Lines[0].URL != "http://same" {
		t.Fatalf("duplicate address %+v", by["新闻"])
	}
	if by["CCTV-1"].Sources != 1 {
		t.Fatalf("CCTV-1 should stay its own name: %+v", by["CCTV-1"])
	}
	if got[0].Name != "CCTV1" {
		t.Fatalf("the name with more sources comes first: %+v", got)
	}
}

func TestNextAggChannelSkipsOtherNames(t *testing.T) {
	chs := []live.Channel{
		{Name: "CCTV1", URL: "http://a"},
		{Name: "湖南卫视", URL: "http://h"},
		{Name: "CCTV1", URL: "http://b"},
	}
	next, ok := nextAggChannel(chs, chs[0])
	if !ok || next.URL != "http://b" || next.Name != "CCTV1" {
		t.Fatalf("next %+v ok %v", next, ok)
	}
}

func TestAggFailureStaysOnTheSameName(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.screen = screenLiveAgg
	a.liveAggLines = []live.Channel{
		{Name: "CCTV1", URL: "http://a"},
		{Name: "湖南卫视", URL: "http://h"},
		{Name: "CCTV1", URL: "http://b"},
	}
	now := time.Now()
	a.liveHealth = map[string]liveHealthState{
		"live:http://a": {ok: false, at: now},
		"live:http://b": {ok: false, at: now},
	}
	a.switchChannel(a.liveAggLines[0])
	if a.screen != screenLiveAgg {
		t.Fatalf("screen %q", a.screen)
	}
	if a.playTitle != "CCTV1" || a.playURL == "http://h" {
		t.Fatalf("title %q url %q", a.playTitle, a.playURL)
	}
	if a.playErr != "没有可用的直播线路" {
		t.Fatalf("err %q", a.playErr)
	}
}

func TestOpenLiveAggSkipsFailedLists(t *testing.T) {
	a := New()
	a.settingsPath = ""
	first := &source.LiveEntry{Name: "甲", URL: "http://list/a"}
	first.Prime([]live.Channel{
		{Name: "CCTV1", URL: "http://a/1", Group: "央视"},
		{Name: "湖南卫视", URL: "http://a/h"},
		{Name: "新闻", URL: "http://same"},
	})
	second := &source.LiveEntry{Name: "乙", URL: "http://list/b"}
	second.Prime([]live.Channel{
		{Name: "CCTV1", URL: "http://b/1", Group: "央视频"},
		{Name: "新闻", URL: "http://same"},
	})
	bad := &source.LiveEntry{Name: "丙", URL: "http://list/c"}
	bad.Prime(nil)
	a.catalog = &source.Catalog{Lives: []*source.LiveEntry{first, second, bad}}
	a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("直播") {
		t.Fatalf("sidebar missing 直播: %q", tt.Texts())
	}
	a.sideSel = "liveagg"
	a.onSide()
	waitLiveAgg(t, a)
	tt.Frame()
	if a.screen != screenLiveAgg || a.liveAggMiss != 1 {
		t.Fatalf("screen %q miss %d", a.screen, a.liveAggMiss)
	}
	if !tt.HasText("CCTV1 · 2 个源") || !tt.HasText("湖南卫视 · 1 个源") || !tt.HasText("新闻 · 1 个源") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if !tt.HasText("1 份列表没读到") {
		t.Fatalf("missing skip note: %q", tt.Texts())
	}
	if err := tt.Click("CCTV1 · 2 个源"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.liveAggName != "CCTV1" {
		t.Fatalf("opened %q", a.liveAggName)
	}
	if !tt.HasText("甲  ·  央视") || !tt.HasText("乙  ·  央视频") {
		t.Fatalf("lines %q", tt.Texts())
	}
	if err := tt.Click("返回"); err != nil {
		t.Fatal(err)
	}
	if a.liveAggName != "" || a.player != nil || a.fromLive {
		t.Fatalf("back name %q fromLive %v", a.liveAggName, a.fromLive)
	}
}

func waitLiveAgg(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.postMu.Lock()
		busy := a.liveAggLoading
		a.postMu.Unlock()
		if !busy {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("live aggregate did not finish")
}
