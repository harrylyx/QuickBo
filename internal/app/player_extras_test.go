package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

// fakePlay is a Player whose clock the test sets, with mpv controls that
// record what they were asked.
type fakePlay struct {
	pos, dur time.Duration
	ended    bool
	seeks    []time.Duration
	props    map[string]string
	cmds     [][]string
}

func (f *fakePlay) Load(_ context.Context, _ string, _ map[string]string) error { return nil }
func (f *fakePlay) Play() error                                                 { return nil }
func (f *fakePlay) Pause() error                                                { return nil }
func (f *fakePlay) Seek(d time.Duration) error                                  { f.seeks = append(f.seeks, d); f.pos = d; return nil }
func (f *fakePlay) SetRate(float64) error                                       { return nil }
func (f *fakePlay) SetVolume(float64) error                                     { return nil }
func (f *fakePlay) SetMuted(bool) error                                         { return nil }
func (f *fakePlay) SetFullscreen(bool) error                                    { return nil }
func (f *fakePlay) Fullscreen() bool                                            { return false }
func (f *fakePlay) Muted() bool                                                 { return false }
func (f *fakePlay) Ready() bool                                                 { return true }
func (f *fakePlay) Playing() bool                                               { return !f.ended }
func (f *fakePlay) Ended() bool                                                 { return f.ended }
func (f *fakePlay) Buffering() bool                                             { return false }
func (f *fakePlay) Position() time.Duration                                     { return f.pos }
func (f *fakePlay) Duration() time.Duration                                     { return f.dur }
func (f *fakePlay) Buffered() time.Duration                                     { return 0 }
func (f *fakePlay) Err() error                                                  { return nil }
func (f *fakePlay) Close() error                                                { return nil }
func (f *fakePlay) SetProp(name, value string) error {
	if f.props == nil {
		f.props = map[string]string{}
	}
	f.props[name] = value
	return nil
}
func (f *fakePlay) Prop(name string) (string, error) { return f.props[name], nil }
func (f *fakePlay) Command(args ...string) error     { f.cmds = append(f.cmds, args); return nil }

// bingeApp is an app playing episode 1 of 3 of 某剧 from the demo source.
func bingeApp(t *testing.T, f *fakePlay) *App {
	t.Helper()
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.siteKey = "c"
	a.detail = &model.Detail{ID: "1", Title: "某剧", Lines: []model.Line{{Name: "线路一", Episodes: []model.Episode{
		{Name: "第1集", URL: "u1"}, {Name: "第2集", URL: "u2"}, {Name: "第3集", URL: "u3"},
	}}}}
	a.playLine, a.playIndex, a.screen = "线路一", 0, screenPlay
	a.player = f
	return a
}

func TestBingeSkipsIntroThenOutroIntoNextEpisode(t *testing.T) {
	f := &fakePlay{pos: 2 * time.Second, dur: 100 * time.Second}
	a := bingeApp(t, f)
	a.setSkip(skipRule{Intro: 30, Outro: 10})

	a.bingeTick()
	if len(f.seeks) != 1 || f.seeks[0] != 30*time.Second || a.px.notice != "已跳过片头" {
		t.Fatalf("intro must be skipped once: seeks %v notice %q", f.seeks, a.px.notice)
	}
	a.bingeTick()
	if len(f.seeks) != 1 {
		t.Fatalf("intro skipped twice: %v", f.seeks)
	}

	f.pos = 91 * time.Second
	a.bingeTick()
	if a.playIndex != 1 {
		t.Fatalf("outro must move on to episode 2, at %d", a.playIndex)
	}
}

// Resuming past the intro does not jump back to the intro's end.
func TestBingeResumeWinsOverIntro(t *testing.T) {
	f := &fakePlay{pos: 0, dur: 100 * time.Second}
	a := bingeApp(t, f)
	a.setSkip(skipRule{Intro: 30})
	a.resumeAt = 50
	a.bingeTick()
	if len(f.seeks) != 0 {
		t.Fatalf("resume point must win: %v", f.seeks)
	}
}

func TestBingeEndPlaysNextUnlessOffOrCanceled(t *testing.T) {
	f := &fakePlay{pos: 100 * time.Second, dur: 100 * time.Second, ended: true}
	a := bingeApp(t, f)
	a.px.autoNext = false
	a.bingeTick()
	if a.playIndex != 0 {
		t.Fatal("auto-next off must stay on the episode")
	}

	b := bingeApp(t, &fakePlay{pos: 95 * time.Second, dur: 100 * time.Second})
	if left, ok := b.nextIn(); !ok || left != 5 {
		t.Fatalf("countdown 5s before the end: %v %v", left, ok)
	}
	b.px.countdownOff = true // 取消
	b.player.(*fakePlay).ended = true
	b.bingeTick()
	if b.playIndex != 0 {
		t.Fatal("a canceled countdown must not play on")
	}

	// The last episode has nothing after it.
	c := bingeApp(t, &fakePlay{pos: 100 * time.Second, dur: 100 * time.Second, ended: true})
	c.playIndex = 2
	c.bingeTick()
	if _, ok := c.nextIn(); ok || c.playIndex != 2 {
		t.Fatal("the last episode must not count down or move on")
	}
}

func TestSkipsAndAutoNextPersist(t *testing.T) {
	a := New()
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	a.detail = &model.Detail{Title: "某剧"}
	a.setSkip(skipRule{Intro: 88.4, Outro: 120})
	a.px.autoNext = false
	a.saveSettings()

	b := New()
	b.settingsPath = a.settingsPath
	b.RestoreSettings()
	b.detail = &model.Detail{Title: "某剧 "}
	if got := b.currentSkip(); got.Intro != 88 || got.Outro != 120 {
		t.Fatalf("skips %+v", got)
	}
	if b.px.autoNext {
		t.Fatal("auto-next off must persist")
	}
	if !New().px.autoNext {
		t.Fatal("auto-next defaults on")
	}
}

func TestRateStepsAndPictureControls(t *testing.T) {
	f := &fakePlay{}
	a := bingeApp(t, f)
	a.rate = 1
	a.stepRate(1)
	a.stepRate(1)
	if a.rate != 1.5 || a.px.notice != "倍速 1.5×" {
		t.Fatalf("rate %v notice %q", a.rate, a.px.notice)
	}
	a.stepRate(-10)
	if a.rate != 0.5 {
		t.Fatalf("rate must stop at the slowest: %v", a.rate)
	}

	a.setAspect(f, "crop")
	if f.props["panscan"] != "1.0" || f.props["keepaspect"] != "yes" {
		t.Fatalf("crop: %v", f.props)
	}
	a.setAspect(f, "16:9")
	if f.props["video-aspect-override"] != "16:9" || f.props["panscan"] != "0" {
		t.Fatalf("16:9: %v", f.props)
	}
	a.setSubDelay(0.5)
	a.setSubScale(1.2)
	if f.props["sub-delay"] != "0.50" || f.props["sub-scale"] != "1.20" {
		t.Fatalf("subtitles: %v", f.props)
	}
	a.selectTrack("sub", 0, "字幕已关闭")
	if f.props["sid"] != "no" {
		t.Fatalf("subtitles off: %v", f.props)
	}

	// A new episode keeps the session's picture settings.
	g := &fakePlay{}
	a.player = g
	a.applyPicture()
	if g.props["video-aspect-override"] != "16:9" || g.props["sub-delay"] != "0.50" {
		t.Fatalf("picture not carried over: %v", g.props)
	}
}

func TestScreenshotNamesFileAfterTitle(t *testing.T) {
	f := &fakePlay{}
	a := bingeApp(t, f)
	t.Setenv("HOME", t.TempDir())
	a.screenshot()
	if len(f.cmds) != 1 || f.cmds[0][0] != "screenshot-to-file" || !strings.Contains(f.cmds[0][1], filepath.Join("Pictures", "QuickBo", "某剧-")) {
		t.Fatalf("cmds %v", f.cmds)
	}
}

func TestPlayRelativeAtTheEndsSaysSo(t *testing.T) {
	a := bingeApp(t, &fakePlay{})
	a.playRelative(-1)
	if a.px.notice != "已经是第一集" || a.playIndex != 0 {
		t.Fatalf("notice %q index %d", a.px.notice, a.playIndex)
	}
}

// ratePlay records the speeds asked of it and whether it plays.
type ratePlay struct {
	fakePlay
	rates  []float64
	paused bool
}

func (r *ratePlay) SetRate(v float64) error { r.rates = append(r.rates, v); return nil }
func (r *ratePlay) Play() error             { r.paused = false; return nil }
func (r *ratePlay) Pause() error            { r.paused = true; return nil }
func (r *ratePlay) Playing() bool           { return !r.paused }

func playWindow(t *testing.T) (*App, *ratePlay, *ui.Tester) {
	t.Helper()
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	p := &ratePlay{fakePlay: fakePlay{dur: 100 * time.Second}}
	a.player, a.screen, a.rate = p, screenPlay, 1.25
	tt := ui.NewTester(a.View, 1200, 800)
	tt.Frame()
	return a, p, tt
}

// Holding space plays at 2× and letting go restores the chosen speed
// without pausing; a tap pauses. The window must not claim space before
// the player sees it go down.
func TestHoldSpaceBoostsAndTapPauses(t *testing.T) {
	a, p, tt := playWindow(t)
	tt.KeyDown(ui.KeySpace)
	tt.KeyRepeat(0, ui.KeySpace)
	tt.Frame()
	if !a.spaceBoosted || len(p.rates) != 1 || p.rates[0] != 2 {
		t.Fatalf("hold: boosted %v rates %v", a.spaceBoosted, p.rates)
	}
	if !tt.HasText("▶▶  2× 加速中") {
		t.Fatalf("the boost must show: %q", tt.Texts())
	}
	tt.KeyUp(ui.KeySpace)
	tt.Frame()
	if a.spaceBoosted || p.rates[len(p.rates)-1] != 1.25 || p.paused {
		t.Fatalf("release: boosted %v rates %v paused %v", a.spaceBoosted, p.rates, p.paused)
	}

	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if !p.paused || len(p.rates) != 2 {
		t.Fatalf("a tap must pause without a speed change: paused %v rates %v", p.paused, p.rates)
	}
}

// B boosts the moment it goes down, with no wait, and restores on release.
func TestHoldBBoostsAtOnce(t *testing.T) {
	a, p, tt := playWindow(t)
	tt.KeyDown(ui.KeyB)
	tt.Frame()
	if !a.spaceBoosted || len(p.rates) != 1 || p.rates[0] != 2 {
		t.Fatalf("B down: boosted %v rates %v", a.spaceBoosted, p.rates)
	}
	tt.KeyUp(ui.KeyB)
	tt.Frame()
	if a.spaceBoosted || p.rates[len(p.rates)-1] != 1.25 || p.paused {
		t.Fatalf("B up: boosted %v rates %v paused %v", a.spaceBoosted, p.rates, p.paused)
	}
}

// The floating window's buttons work over the layer that drags the window.
func TestPipWindowControls(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.detail = &model.Detail{Title: "某剧"}
	p := &ratePlay{fakePlay: fakePlay{pos: 30 * time.Second, dur: 100 * time.Second}}
	a.player, a.screen, a.px.pip = p, screenPlay, true
	tt := ui.NewTester(a.PipView, 480, 270)
	tt.Move(240, 135)
	tt.Frame()
	for _, want := range []string{"某剧", "后退 10 秒", "暂停", "回到主窗口（P）"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %s: %q", want, tt.Texts())
		}
	}
	if err := tt.Click("后退 10 秒"); err != nil {
		t.Fatal(err)
	}
	if len(p.seeks) != 1 || p.seeks[0] != 20*time.Second {
		t.Fatalf("seek back over the drag layer: %v", p.seeks)
	}
	if err := tt.Click("暂停"); err != nil {
		t.Fatal(err)
	}
	if !p.paused {
		t.Fatal("pause over the drag layer")
	}
	if err := tt.Click("回到主窗口（P）"); err != nil {
		t.Fatal(err)
	}
	if a.px.pip {
		t.Fatal("restore must leave the floating window")
	}
}

func TestCapLiveRenderKeepsUltraHDSmall(t *testing.T) {
	w, h := capLiveRender(3840, 2160)
	if w != 1920 || h != 1080 {
		t.Fatalf("4K live render %dx%d", w, h)
	}
	w, h = capLiveRender(1280, 720)
	if w != 1280 || h != 720 {
		t.Fatalf("720p should stay %dx%d", w, h)
	}
}

func TestSeekByLeavesALiveStream(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.fromLive = true
	p := &fakePlay{pos: 10 * time.Second, dur: time.Hour}
	a.player = p
	a.seekBy(10)
	a.seekBy(-10)
	if len(p.seeks) != 0 {
		t.Fatalf("live seek %v", p.seeks)
	}
}

func TestMainWindowYieldsToThePip(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{}
	a.screen = screenLiveAgg
	a.fromLive = true
	a.px.pip = true
	a.player = &fakePlay{}
	tt := ui.NewTester(a.View, 1280, 800)
	tt.Frame()
	if tt.HasText("首页") || tt.HasText("分类") || tt.HasText("过滤频道") {
		t.Fatalf("main window still drew the app: %q", tt.Texts())
	}
}

func TestLivePipHasNoSeek(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.fromLive = true
	a.playTitle = "CCTV1"
	a.screen = screenLiveAgg
	a.px.pip = true
	a.detail = &model.Detail{Title: "某剧", Lines: []model.Line{{Name: "l", Episodes: []model.Episode{{Name: "1"}, {Name: "2"}}}}}
	a.playIndex = 0
	p := &fakePlay{pos: 30 * time.Second, dur: 100 * time.Second}
	a.player = p
	tt := ui.NewTester(a.PipView, 480, 270)
	tt.Move(240, 135)
	tt.Frame()
	if tt.HasText("后退 10 秒") || tt.HasText("快进 10 秒") || tt.HasText("下一集") {
		t.Fatalf("live pip offered a seek: %q", tt.Texts())
	}
	if !tt.HasText("CCTV1") || !tt.HasText("暂停") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if len(p.seeks) != 0 {
		t.Fatal("opening the live pip sought")
	}
}

// A live stream's playhead only counts how long the player has been open.
// The bar stays a still "直播" mark, while a recording keeps its times.
func TestLivePlaybackSkipsTheTimeline(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.fromLive = true
	p := &fakePlay{pos: 12 * time.Minute, dur: 40 * time.Minute}
	a.player = p
	live := ui.NewTester(func(c *ui.Context) {
		th := a.useTheme(c)
		a.seekBar(c, th)
		a.thinProgress(c, th)
		a.pipProgress(c, th)
	}, 800, 240)
	if live.HasText("12:00") || live.HasText("40:00") {
		t.Fatalf("live timeline is moving: %q", live.Texts())
	}
	if !live.HasText("直播") {
		t.Fatalf("texts %q", live.Texts())
	}
	if len(p.seeks) != 0 {
		t.Fatal("the live mark sought")
	}

	a.fromLive = false
	vod := ui.NewTester(func(c *ui.Context) {
		th := a.useTheme(c)
		a.seekBar(c, th)
	}, 800, 120)
	if !vod.HasText("12:00") || !vod.HasText("40:00") {
		t.Fatalf("vod timeline missing: %q", vod.Texts())
	}
}
