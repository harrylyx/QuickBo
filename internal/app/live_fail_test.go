package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/live"
	"github.com/harrylyx/QuickBo/internal/source"
)

// liveFailedApp reproduces the state after every line of a plain live list
// has failed. The health cache marks both addresses down, so the whole line
// walk runs synchronously and offline.
func liveFailedApp(t *testing.T) *App {
	t.Helper()
	a := New()
	a.settingsPath = ""
	chs := []live.Channel{
		{Name: "CCTV1", URL: "http://a/1", Group: "央视"},
		{Name: "CCTV1", URL: "http://a/2", Group: "央视"},
	}
	entry := &source.LiveEntry{Name: "甲", URL: "http://list/a"}
	entry.Prime(chs)
	a.catalog = &source.Catalog{Lives: []*source.LiveEntry{entry}}
	a.channels = chs
	a.screen = screenLive
	a.sideSel = "live:0"
	a.lastSide = "live:0"
	a.liveIndex = 0
	now := time.Now()
	a.liveHealth = map[string]liveHealthState{
		"live:http://a/1": {ok: false, at: now},
		"live:http://a/2": {ok: false, at: now},
	}
	a.playChannel(chs[0])
	return a
}

func liveStageTexts(tt *ui.Tester) string {
	return strings.Join(tt.Texts(), "\n")
}

func TestAllLiveLinesFailKeepsPlayerWithNotice(t *testing.T) {
	a := liveFailedApp(t)
	if a.playErr != "没有可用的直播线路" {
		t.Fatalf("err %q", a.playErr)
	}
	if a.player != nil || !a.liveStageOpen() || !a.liveStageFailed() {
		t.Fatalf("stage lost: player %v open %v failed %v", a.player, a.liveStageOpen(), a.liveStageFailed())
	}
	if a.screen != screenLive {
		t.Fatalf("screen %q", a.screen)
	}
	tt := ui.NewTester(a.View, 1200, 800)
	texts := liveStageTexts(tt)
	for _, want := range []string{"直播暂时无法播放", "没有可用的直播线路", "重试线路", "返回频道列表"} {
		if !strings.Contains(texts, want) {
			t.Fatalf("missing %q in %q", want, texts)
		}
	}
}

func TestAllAggLiveLinesFailKeepsPlayerWithNotice(t *testing.T) {
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
	if a.playErr != "没有可用的直播线路" || !a.liveStageFailed() {
		t.Fatalf("err %q stage failed %v", a.playErr, a.liveStageFailed())
	}
	tt := ui.NewTester(a.View, 1200, 800)
	texts := liveStageTexts(tt)
	for _, want := range []string{"直播暂时无法播放", "重试线路", "返回频道列表"} {
		if !strings.Contains(texts, want) {
			t.Fatalf("missing %q in %q", want, texts)
		}
	}
}

func TestFailedLiveStageKeepsFullscreenAndCanLeave(t *testing.T) {
	a := liveFailedApp(t)
	a.fullscreen = true
	if !a.liveFullscreen() {
		t.Fatal("fullscreen stage dropped after every line failed")
	}
	tt := ui.NewTester(a.View, 1200, 800)
	if !strings.Contains(liveStageTexts(tt), "直播暂时无法播放") {
		t.Fatalf("fullscreen stage lost the notice: %q", tt.Texts())
	}
	if err := tt.Click("返回频道列表"); err != nil {
		t.Fatal(err)
	}
	if a.liveStageOpen() || a.liveStageFailed() || a.playErr != "" || a.fromLive || a.fullscreen {
		t.Fatalf("stage did not close: open %v failed %v err %q fromLive %v fullscreen %v",
			a.liveStageOpen(), a.liveStageFailed(), a.playErr, a.fromLive, a.fullscreen)
	}
}

func TestSwitchingLiveListClearsFailedStage(t *testing.T) {
	a := liveFailedApp(t)
	if !a.liveStageFailed() {
		t.Fatal("expected a failed stage")
	}
	second := &source.LiveEntry{Name: "乙", URL: "http://list/b"}
	second.Prime([]live.Channel{{Name: "CCTV2", URL: "http://b/1"}})
	a.catalog.Lives = append(a.catalog.Lives, second)
	a.selectLive(1)
	if a.playErr != "" || a.playLoading || a.liveStageOpen() {
		t.Fatalf("failed stage leaked: err %q loading %v open %v", a.playErr, a.playLoading, a.liveStageOpen())
	}
}

func TestClearLiveFailuresForgetsDownVerdicts(t *testing.T) {
	a := liveFailedApp(t)
	a.liveHealth["live:http://a/3"] = liveHealthState{ok: true, at: time.Now()}
	a.clearLiveFailures()
	if len(a.liveHealth) != 1 || !a.liveHealth["live:http://a/3"].ok {
		t.Fatalf("health %+v", a.liveHealth)
	}
}
