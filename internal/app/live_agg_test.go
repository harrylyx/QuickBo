package app

import (
	"strings"
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
	if liveBucket("CCTV1") != "央视" || liveBucket("央视新闻") != "央视" {
		t.Fatal("cctv bucket")
	}
	if liveBucket("湖南卫视") != "地方卫视" {
		t.Fatal("satellite bucket")
	}
	if liveBucket("新闻") != "其他" {
		t.Fatal("other bucket")
	}
	names := make([]string, len(got))
	for i, g := range got {
		names[i] = g.Name
	}
	if strings.Join(names, ",") != "CCTV-1,CCTV1,新闻,湖南卫视" {
		t.Fatalf("names sort %v", names)
	}
}

func TestLiveNamesSortByNumber(t *testing.T) {
	got := aggregateLives([]liveSnap{{
		Name: "甲", URL: "http://a",
		Chs: []live.Channel{
			{Name: "CCTV10", URL: "http://10"},
			{Name: "CCTV2", URL: "http://2"},
			{Name: "CCTV1", URL: "http://1"},
			{Name: "湖南卫视", URL: "http://h"},
			{Name: "北京卫视", URL: "http://b"},
		},
	}})
	names := make([]string, len(got))
	for i, g := range got {
		names[i] = g.Name
	}
	if strings.Join(names, ",") != "CCTV1,CCTV2,CCTV10,北京卫视,湖南卫视" {
		t.Fatalf("names %v", names)
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
	if !tt.HasText("央视") || !tt.HasText("地方卫视") || !tt.HasText("其他") {
		t.Fatalf("missing sections: %q", tt.Texts())
	}
	if !tt.HasText("CCTV1") || !tt.HasText("湖南卫视") || !tt.HasText("新闻") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if !tt.HasText("1 份列表没读到") {
		t.Fatalf("missing skip note: %q", tt.Texts())
	}
	texts := tt.Texts()
	if strings.Index(strings.Join(texts, "\n"), "央视") > strings.Index(strings.Join(texts, "\n"), "地方卫视") {
		t.Fatalf("央视 should come before 地方卫视: %q", texts)
	}
}

func TestAutoSwitchAnnouncesTheNextLine(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{}
	a.screen = screenLiveAgg
	a.liveAggs = []liveAgg{{
		Name: "CCTV1", Sources: 2,
		Lines: []liveLine{
			{Source: "甲", Name: "CCTV1", URL: "http://a", Group: "央视"},
			{Source: "乙", Name: "CCTV1", URL: "http://b", Group: "央视频"},
		},
	}}
	now := time.Now()
	a.liveHealth = map[string]liveHealthState{
		"live:http://a": {ok: false, at: now},
		"live:http://b": {ok: false, at: now},
	}
	a.playLiveName("CCTV1")
	if a.playErr != "没有可用的直播线路" {
		t.Fatalf("err %q", a.playErr)
	}
	if !strings.Contains(a.px.notice, "播放超时") || !strings.Contains(a.px.notice, "已换到") {
		t.Fatalf("notice %q", a.px.notice)
	}
	if a.screen != screenLiveAgg {
		t.Fatalf("screen %q", a.screen)
	}
}

func TestLiveLineDrawerListsSources(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{}
	a.screen = screenLiveAgg
	a.liveAggName = "CCTV1"
	a.liveLineAt = 0
	a.fromLive = true
	a.playLoading = true
	a.showDrawer = true
	a.liveAggLines = []live.Channel{{Name: "CCTV1", URL: "http://a"}, {Name: "CCTV1", URL: "http://b"}}
	a.liveAggs = []liveAgg{{
		Name: "CCTV1",
		Lines: []liveLine{
			{Source: "甲", Name: "CCTV1", URL: "http://a", Group: "央视"},
			{Source: "乙", Name: "CCTV1", URL: "http://b", Group: "央视频"},
		},
	}}
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("线路") || !tt.HasText("甲  ·  央视") || !tt.HasText("乙  ·  央视频") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if tt.HasText("测速") {
		t.Fatal("speed test should be gone")
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
