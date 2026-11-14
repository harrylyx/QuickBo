package app

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/player"
)

// playerExtras is the player's state beyond playback itself.
type playerExtras struct {
	// A short notice in the middle of the picture: volume, speed, a seek.
	notice      string
	noticeUntil time.Time

	// Picture-in-picture: a frameless window of its own, kept above other
	// apps, while the main window hides.
	pip    bool
	pipWin *mygo.Window

	// The settings panel and the keyboard help.
	panel bool
	keys  bool

	// Picture and subtitles for this session.
	aspect     string
	subScale   float64
	subDelay   float64
	brightness float64
	contrast   float64
	tracks     []player.Track
	tracksAt   time.Time

	// Binge-watching: play on at the end, and the intro/outro to skip per
	// title. The flags are per episode, reset as one starts.
	autoNext       bool
	skips          map[string]skipRule
	introDone      bool
	outroDone      bool
	endDone        bool
	countdownOff   bool
	seekDragging   bool
	seekDragAt     float64
	lastWheelPulse time.Time
}

// skipRule is how much of an episode's start and end to skip, in seconds.
type skipRule struct {
	Intro float64 `json:"intro,omitempty"`
	Outro float64 `json:"outro,omitempty"`
}

func cloneSkips(in map[string]skipRule) map[string]skipRule {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]skipRule, len(in))
	for k, v := range in {
		if v.Intro > 0 || v.Outro > 0 {
			out[k] = v
		}
	}
	return out
}

// rateList are the speeds the player offers, as mature players do: fine
// steps around normal, wider ones above.
var rateList = []float64{0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3}

// countdownLead is how long before an episode ends the next one is
// announced, with a way to cancel it.
const countdownLead = 8.0

// notify shows msg over the picture for a moment.
func (a *App) notify(msg string) {
	a.px.notice = msg
	a.px.noticeUntil = time.Now().Add(1200 * time.Millisecond)
}

// controls returns the player's mpv controls, when it has them.
func (a *App) controls() (player.Controls, bool) {
	if a.player == nil {
		return nil, false
	}
	c, ok := a.player.(player.Controls)
	return c, ok
}

// resetEpisodeExtras clears what belongs to one episode as another starts.
func (a *App) resetEpisodeExtras() {
	a.px.introDone, a.px.outroDone, a.px.endDone, a.px.countdownOff = false, false, false, false
	a.px.tracks, a.px.tracksAt = nil, time.Time{}
	a.px.seekDragging = false
}

// applyPicture re-applies the session's picture and subtitle settings to a
// freshly loaded player.
func (a *App) applyPicture() {
	ctl, ok := a.controls()
	if !ok {
		return
	}
	a.setAspect(ctl, a.px.aspect)
	if a.px.subScale > 0 && a.px.subScale != 1 {
		ctl.SetProp("sub-scale", fmt.Sprintf("%.2f", a.px.subScale))
	}
	if a.px.subDelay != 0 {
		ctl.SetProp("sub-delay", fmt.Sprintf("%.2f", a.px.subDelay))
	}
	if a.px.brightness != 0 {
		ctl.SetProp("brightness", fmt.Sprintf("%.0f", a.px.brightness))
	}
	if a.px.contrast != 0 {
		ctl.SetProp("contrast", fmt.Sprintf("%.0f", a.px.contrast))
	}
}

// --- speed ---

func rateIndex(r float64) int {
	best := 0
	for i, v := range rateList {
		if math.Abs(v-r) < math.Abs(rateList[best]-r) {
			best = i
		}
	}
	return best
}

// stepRate moves the speed one step along rateList.
func (a *App) stepRate(dir int) {
	i := rateIndex(a.rate) + dir
	i = max(0, min(len(rateList)-1, i))
	a.setRate(rateList[i])
}

// --- episodes ---

// episodeList is the line being played and its episodes.
func (a *App) episodeList() (string, []model.Episode) {
	if a.detail == nil {
		return "", nil
	}
	for _, ln := range a.detail.Lines {
		if a.playLine == "" || ln.Name == a.playLine {
			return ln.Name, ln.Episodes
		}
	}
	if len(a.detail.Lines) > 0 && a.linePick < len(a.detail.Lines) {
		ln := a.detail.Lines[a.linePick]
		return ln.Name, ln.Episodes
	}
	return "", nil
}

// hasEpisode reports whether an episode delta away exists.
func (a *App) hasEpisode(delta int) bool {
	_, eps := a.episodeList()
	next := a.playIndex + delta
	return a.playIndex >= 0 && next >= 0 && next < len(eps)
}

func (a *App) skipKey() string {
	title := a.playTitle
	if a.detail != nil && strings.TrimSpace(a.detail.Title) != "" {
		title = a.detail.Title
	}
	return watchTitle(title)
}

func (a *App) currentSkip() skipRule {
	return a.px.skips[a.skipKey()]
}

func (a *App) setSkip(rule skipRule) {
	key := a.skipKey()
	if key == "" {
		return
	}
	if a.px.skips == nil {
		a.px.skips = map[string]skipRule{}
	}
	rule.Intro = max(0, math.Round(rule.Intro))
	rule.Outro = max(0, math.Round(rule.Outro))
	if rule.Intro == 0 && rule.Outro == 0 {
		delete(a.px.skips, key)
	} else {
		a.px.skips[key] = rule
	}
	a.saveSettings()
}

// bingeTick skips the intro and outro and plays on at the end. It runs in
// every frame of the player (playerKeys).
func (a *App) bingeTick() {
	p := a.player
	if p == nil || a.fromLive || !p.Ready() {
		return
	}
	pos, dur := p.Position().Seconds(), p.Duration().Seconds()
	skip := a.currentSkip()
	if !a.px.introDone {
		a.px.introDone = true
		// Resuming further in wins over the intro.
		if skip.Intro > 0 && a.resumeAt < skip.Intro && pos < skip.Intro-1 {
			p.Seek(time.Duration(skip.Intro * float64(time.Second)))
			a.notify("已跳过片头")
		}
	}
	if dur <= 0 {
		return
	}
	next := a.px.autoNext && a.hasEpisode(1)
	if skip.Outro > 0 && !a.px.outroDone && pos >= dur-skip.Outro && pos > skip.Intro {
		a.px.outroDone = true
		if next && !a.px.countdownOff {
			a.notify("已跳过片尾")
			a.playRelative(1)
			return
		}
	}
	if p.Ended() && !a.px.endDone {
		a.px.endDone = true
		if next && !a.px.countdownOff {
			a.playRelative(1)
		}
	}
}

// nextIn is how many seconds remain before the next episode starts on its
// own, when it will.
func (a *App) nextIn() (float64, bool) {
	p := a.player
	if p == nil || a.fromLive || !a.px.autoNext || a.px.countdownOff || !a.hasEpisode(1) || !p.Ready() {
		return 0, false
	}
	pos, dur := p.Position().Seconds(), p.Duration().Seconds()
	if dur <= 0 {
		return 0, false
	}
	end := dur
	if o := a.currentSkip().Outro; o > 0 {
		end = dur - o
	}
	left := end - pos
	if left > countdownLead || left < 0 {
		return 0, false
	}
	return left, true
}

// --- picture ---

// aspectChoices are the picture shapes offered: as mpv sees them, and the
// label.
var aspectChoices = [][2]string{
	{"", "默认"}, {"16:9", "16:9"}, {"4:3", "4:3"}, {"2.35:1", "21:9"}, {"stretch", "拉伸铺满"}, {"crop", "裁剪铺满"},
}

func (a *App) setAspect(ctl player.Controls, v string) {
	a.px.aspect = v
	keep, aspect, pan := "yes", "-1", "0"
	switch v {
	case "":
	case "stretch":
		keep = "no"
	case "crop":
		pan = "1.0"
	default:
		aspect = v
	}
	ctl.SetProp("keepaspect", keep)
	ctl.SetProp("video-aspect-override", aspect)
	ctl.SetProp("panscan", pan)
}

// screenshot saves the frame, with its subtitles, under Pictures/QuickBo.
func (a *App) screenshot() {
	ctl, ok := a.controls()
	if !ok {
		a.notify("当前播放方式不支持截图")
		return
	}
	dir := filepath.Join(userPictures(), "QuickBo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.notify("截图失败：" + err.Error())
		return
	}
	name := safeFileName(a.skipKey())
	if name == "" {
		name = "截图"
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.png", name, time.Now().Format("20060102-150405")))
	if err := ctl.Command("screenshot-to-file", path, "subtitles"); err != nil {
		a.notify("截图失败：" + err.Error())
		return
	}
	a.notify("截图已保存到 图片/QuickBo")
}

func userPictures() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(home, "Pictures")
}

func safeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 32 {
			return '_'
		}
		return r
	}, strings.TrimSpace(s))
	if len([]rune(s)) > 40 {
		s = string([]rune(s)[:40])
	}
	return s
}

// --- tracks ---

// tracks returns the stream's tracks, read again at most every two seconds.
func (a *App) tracks() []player.Track {
	ctl, ok := a.controls()
	if !ok {
		return nil
	}
	if time.Since(a.px.tracksAt) > 2*time.Second {
		a.px.tracks = player.Tracks(ctl)
		a.px.tracksAt = time.Now()
	}
	return a.px.tracks
}

func tracksOf(all []player.Track, kind string) []player.Track {
	var out []player.Track
	for _, t := range all {
		if t.Kind == kind {
			out = append(out, t)
		}
	}
	return out
}

// selectTrack chooses a track of kind ("sub" or "audio"); id 0 turns
// subtitles off.
func (a *App) selectTrack(kind string, id int, label string) {
	ctl, ok := a.controls()
	if !ok {
		return
	}
	prop, v := "aid", fmt.Sprint(id)
	if kind == "sub" {
		prop = "sid"
		if id == 0 {
			v = "no"
		}
	}
	if err := ctl.SetProp(prop, v); err != nil {
		a.notify("切换失败：" + err.Error())
		return
	}
	a.px.tracksAt = time.Time{}
	a.notify(label)
}

// loadSubtitle asks for a subtitle file and adds it to the stream.
func (a *App) loadSubtitle() {
	if _, ok := a.controls(); !ok {
		return
	}
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  win,
			Title:   "加载字幕",
			Filters: []mygo.FileFilter{{Name: "字幕", Extensions: []string{"srt", "ass", "ssa", "vtt", "sub", "sup"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.post(func() {
			ctl, ok := a.controls()
			if !ok {
				return
			}
			if err := ctl.Command("sub-add", paths[0], "select"); err != nil {
				a.notify("字幕加载失败：" + err.Error())
				return
			}
			a.px.tracksAt = time.Time{}
			a.notify("已加载字幕 " + filepath.Base(paths[0]))
		})
	}()
}

func (a *App) setSubScale(v float64) {
	a.px.subScale = math.Round(max(0.5, min(3, v))*10) / 10
	if ctl, ok := a.controls(); ok {
		ctl.SetProp("sub-scale", fmt.Sprintf("%.2f", a.px.subScale))
	}
	a.notify(fmt.Sprintf("字幕大小 %.0f%%", a.px.subScale*100))
}

func (a *App) setSubDelay(v float64) {
	a.px.subDelay = math.Round(max(-30, min(30, v))*10) / 10
	if ctl, ok := a.controls(); ok {
		ctl.SetProp("sub-delay", fmt.Sprintf("%.2f", a.px.subDelay))
	}
	a.notify(fmt.Sprintf("字幕延迟 %+.1f 秒", a.px.subDelay))
}

func (a *App) setPictureProp(prop string, v float64) {
	v = math.Round(max(-100, min(100, v)))
	if prop == "brightness" {
		a.px.brightness = v
	} else {
		a.px.contrast = v
	}
	if ctl, ok := a.controls(); ok {
		ctl.SetProp(prop, fmt.Sprintf("%.0f", v))
	}
}

// --- picture-in-picture ---

const pipW, pipH = 480, 270

// togglePip moves the video into a small floating window, or back.
func (a *App) togglePip() {
	if a.px.pip {
		a.exitPip()
		return
	}
	a.enterPip()
}

// enterPip opens the floating window: frameless, rounded, above other apps
// and on every desktop, at the bottom-right of where the main window was,
// which hides until the video comes back.
func (a *App) enterPip() {
	if a.win == nil || a.player == nil || a.px.pip {
		return
	}
	if !a.embedded() {
		a.notify("在独立 mpv 窗口播放时不支持小窗")
		return
	}
	if a.fullscreen {
		a.setFullscreen(false)
	}
	a.showDrawer, a.px.panel, a.px.keys = false, false, false
	r := a.win.Bounds()
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:             "QuickBo 小窗",
		Width:             pipW,
		Height:            pipH,
		MinWidth:          256,
		MinHeight:         144,
		Frameless:         true,
		CornerRadius:      12,
		AlwaysOnTop:       true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		BackgroundColor:   "#000000",
		Content:           ui.View(a.PipView),
	})
	w.SetBounds(mygo.Rectangle{X: r.X + r.Width - pipW - 24, Y: r.Y + r.Height - pipH - 24, Width: pipW, Height: pipH})
	w.SetVisibleOnAllWorkspaces(true)
	w.OnClosed(func() {
		// Closed by the system (Cmd+W): bring the main window back.
		a.post(func() {
			if a.px.pipWin == w {
				a.px.pipWin = nil
				a.exitPip()
			}
		})
	})
	a.px.pip, a.px.pipWin = true, w
	a.win.Hide()
	w.Show()
	// The click that opened the small window orders the main window front
	// again once the mouse event finishes, which left the live page on
	// screen. Hide it once that event is over.
	w.Update(func() {
		if a.px.pip && a.win != nil {
			a.win.Hide()
			if pw := a.px.pipWin; pw != nil {
				pw.Focus()
			}
		}
	})
}

// exitPip closes the floating window and shows the main one again.
func (a *App) exitPip() {
	if !a.px.pip {
		return
	}
	a.px.pip = false
	w := a.px.pipWin
	a.px.pipWin = nil
	if a.win != nil {
		a.win.Show()
		a.win.Focus()
		a.win.Invalidate()
	}
	if w != nil {
		w.Close()
	}
}

// PipView is the floating window's content.
func (a *App) PipView(c *ui.Context) {
	a.drawFrame(func() {
		th := a.useTheme(c)
		// The main window is hidden, so the failover watch has to run here.
		a.watchLivePlayback()
		a.playerKeys(c)
		if !a.px.pip || a.player == nil {
			ui.Box(c).Fill().Background(ui.Hex("#000000"))
			return
		}
		a.viewPip(c, th)
	})
}

// viewPip is the floating window: the picture edge to edge; the pointer over
// it fades in a title bar to drag it by, a play button in the middle and a
// progress bar, as the system's own picture-in-picture does.
func (a *App) viewPip(c *ui.Context, th *ui.Theme) {
	white := th.AccentText
	stage := ui.Box(c).Fill().Background(ui.Hex("#000000")).Focusable().FocusRing(false).Focus().HandleInput(a.playerInput)
	size := stage.Bounds()
	_, _, over := stage.PointerPosition()
	if over {
		c.After(150 * time.Millisecond)
	}
	paused := !a.player.Playing() && a.player.Ready()
	show := over || paused
	fade := ui.ElementTransition{Duration: 180 * time.Millisecond, ExitDuration: 240 * time.Millisecond, Ease: ui.EaseOut,
		Enter: &ui.Motion{Opacity: 0, Scale: 1}, Exit: &ui.Motion{Opacity: 0, Scale: 1}}
	stage.Children(func() {
		if vs, ok := a.player.(player.VideoSource); ok {
			a.paintVideo(c, vs)
		}
		// The whole picture moves the window; the buttons over it take their
		// own clicks.
		ui.Box(c.Key("pip-drag")).Absolute().Left(0).Right(0).Top(0).Bottom(0).DragWindow()
		a.noticeLayer(c, th)
		if !show {
			a.thinProgress(c, th)
			return
		}
		ui.Box(c.Key("pip-scrim")).Absolute().Left(0).Right(0).Top(0).Bottom(0).IgnoreInput(true).
			Background(ui.RGBA(0, 0, 0, 0.28)).Transition(fade)
		// The title bar drags the window too; its buttons take their clicks.
		ui.Row(c.Key("pip-top")).Absolute().Left(0).Right(0).Top(0).Padding(8, 8, 0, 12).Gap(2).AlignItems(ui.Center).
			DragWindow().Transition(fade).Children(func() {
			title, episode := a.overlayTitle()
			if episode != "" {
				title += "  ·  " + episode
			}
			ui.Text(c, title).FontSize(fontCaption).FontWeight(weightStrong).TextColor(white).SingleLine().Grow(1).Shrink(1).IgnoreInput(true)
			a.iconBtn(c, "pip-restore", iconExternal, "回到主窗口（P）", 28, 15, white, a.exitPip)
			a.iconBtn(c, "pip-close", iconClose, "关闭小窗并停止播放", 28, 15, white, func() {
				a.exitPip()
				a.back()
			})
		})
		// Only as large as its buttons, centred: a row across the window
		// would cover the drag layer and the title bar.
		ui.Row(c.Key("pip-mid")).Absolute().Left(size.W/2).Top(size.H/2).TranslatePercent(-50, -50).
			AlignItems(ui.Center).Gap(18).Transition(fade).Children(func() {
			glyph, tip := iconPlay, "播放"
			if a.player.Playing() {
				glyph, tip = iconPause, "暂停"
			}
			if !a.fromLive {
				a.iconBtn(c, "pip-rew", iconRew, "后退 10 秒", 36, 20, white, func() { a.seekBy(-10) })
			}
			a.pipPlayButton(c, th, glyph, tip)
			if !a.fromLive {
				a.iconBtn(c, "pip-fwd", iconFwd, "快进 10 秒", 36, 20, white, func() { a.seekBy(10) })
				if a.hasEpisode(1) {
					a.iconBtn(c, "pip-next", iconNext, "下一集", 36, 20, white, func() { a.playRelative(1) })
				}
			}
		})
		ui.Column(c.Key("pip-bottom")).Absolute().Left(10).Right(10).Bottom(8).Gap(2).Transition(fade).Children(func() {
			a.pipProgress(c, th)
		})
	})
}

// pipPlayButton is the large play/pause disc in the middle of the floating
// window.
func (a *App) pipPlayButton(c *ui.Context, th *ui.Theme, glyph *ui.SVG, tip string) {
	b := ui.Column(c.Key("pip-pp")).Size(48, 48).Radius(24).Center().OnClick(a.togglePlay).
		Cursor(ui.CursorPointer).Tooltip(tip).Label(tip)
	bg := ui.RGBA(255, 255, 255, 0.18)
	if b.Hovered() {
		bg = ui.RGBA(255, 255, 255, 0.3)
	}
	b.Background(b.AnimateColor("bg", bg, colorEase))
	pressPose(b, 0.9)
	b.Children(func() {
		ui.Icon(c, glyph).Size(24, 24).TextColor(th.AccentText)
	})
}

// pipProgress is a slim seek bar with the times at its ends.
func (a *App) pipProgress(c *ui.Context, th *ui.Theme) {
	if a.fromLive {
		ui.Text(c, "直播").FontSize(11).TextColor(th.AccentText.Alpha(0.85))
		return
	}
	pos, dur := a.player.Position().Seconds(), a.player.Duration().Seconds()
	if dur <= 0 {
		return
	}
	frac := float32(max(0, min(1, pos/dur)))
	hit := ui.Box(c.Key("pip-seek")).FillWidth().Height(14).Cursor(ui.CursorPointer)
	w := hit.Bounds().W
	if hit.Clicked() && w > 1 {
		x, _, _ := hit.PointerPosition()
		a.player.Seek(time.Duration(float64(x) / float64(w) * dur * float64(time.Second)))
		a.noteProgress()
	}
	hit.Children(func() {
		ui.Box(c).Absolute().Left(0).Right(0).Top(5).Height(4).Radius(2).Background(th.AccentText.Alpha(0.3))
		ui.Box(c).Absolute().Left(0).Top(5).Height(4).Width(w * frac).Radius(2).Background(th.Accent)
	})
	ui.Row(c).Children(func() {
		ui.Text(c, clock(pos)).FontSize(11).TextColor(th.AccentText.Alpha(0.85)).Grow(1)
		ui.Text(c, clock(dur)).FontSize(11).TextColor(th.AccentText.Alpha(0.85))
	})
}

// --- overlays ---

// noticeLayer shows the current notice, the 2× boost while space is held,
// and the next-episode countdown.
func (a *App) noticeLayer(c *ui.Context, th *ui.Theme) {
	white := th.AccentText
	pill := func(key, text string) {
		ui.Row(c.Key(key)).Padding(8, 16).Radius(18).Background(ui.RGBA(0, 0, 0, 0.62)).Gap(8).AlignItems(ui.Center).
			Transition(ui.ElementTransition{Duration: 160 * time.Millisecond, ExitDuration: 220 * time.Millisecond,
				Enter: &ui.Motion{Opacity: 0, Scale: 0.92}, Exit: &ui.Motion{Opacity: 0, Scale: 0.96}}).
			Children(func() {
				ui.Text(c, text).FontSize(fontBody).Bold().TextColor(white).SingleLine()
			})
	}
	ui.Column(c).Absolute().Left(0).Right(0).Top(72).AlignItems(ui.Center).Gap(8).IgnoreInput(true).Children(func() {
		if a.spaceBoosted {
			pill("boost", "▶▶  2× 加速中")
		}
		if remain := time.Until(a.px.noticeUntil); remain > 0 && a.px.notice != "" {
			pill("notice:"+a.px.notice, a.px.notice)
			c.After(remain + 30*time.Millisecond)
		}
	})
	if left, ok := a.nextIn(); ok && !a.px.pip {
		ui.Row(c).Absolute().Right(28).Bottom(120).Padding(10, 14).Radius(12).Background(ui.RGBA(0, 0, 0, 0.72)).
			Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("%.0f 秒后播放下一集", math.Ceil(left))).TextColor(white).SingleLine()
			if ui.Button(c, "立即播放").Clicked() {
				a.playRelative(1)
			}
			if ui.Button(c, "取消").Clicked() {
				a.px.countdownOff = true
			}
		})
		c.After(250 * time.Millisecond)
	}
}

// keysHelp lists the player's keys.
var keysHelp = [][2]string{
	{"空格", "播放 / 暂停"},
	{"按住空格 或 按住 B", "临时 2 倍速，松开恢复"},
	{"← / →", "后退 / 快进 10 秒"},
	{"Shift + ← / →", "后退 / 快进 60 秒"},
	{"↑ / ↓  或滚轮", "音量"},
	{"M", "静音"},
	{"[ / ]", "减速 / 加速"},
	{"\\", "恢复 1 倍速"},
	{"N / PageDown", "下一集"},
	{"PageUp", "上一集"},
	{"F 或双击画面", "全屏"},
	{"P", "悬浮小窗"},
	{"S", "截图"},
	{"Esc", "退出全屏 / 返回"},
	{"?", "显示 / 隐藏快捷键"},
}

func (a *App) keysOverlay(c *ui.Context, th *ui.Theme) {
	white := th.AccentText
	ui.Column(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Center().Background(ui.RGBA(0, 0, 0, 0.45)).
		OnClick(func() { a.px.keys = false }).Children(func() {
		ui.Column(c).Width(420).Padding(20, 24).Radius(14).Background(stageInk(false)).Gap(10).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "快捷键").FontSize(fontTitle).Bold().TextColor(white).Grow(1)
				a.iconBtn(c, "keys-close", iconClose, "关闭", 44, 22, white, func() { a.px.keys = false })
			})
			for _, k := range keysHelp {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, k[0]).FontSize(fontCaption).Bold().TextColor(white).Width(130).SingleLine()
					ui.Text(c, k[1]).FontSize(fontCaption).TextColor(white.Alpha(0.8)).SingleLine()
				})
			}
		})
	})
}

// settingsPanel is the player's side panel: picture, subtitles, audio, and
// binge-watching. Speed stays on the bottom bar.
func (a *App) settingsPanel(c *ui.Context, th *ui.Theme) {
	white := th.AccentText
	muted := white.Alpha(0.6)
	section := func(title string) {
		ui.Text(c, title).FontSize(fontCaption).Bold().TextColor(muted).Padding(8, 0, 0, 0)
	}
	chips := func(fn func()) { ui.Row(c).Gap(6).Wrap().Children(fn) }
	ctl, hasCtl := a.controls()
	ui.Column(c).Absolute().Top(0).Right(0).Bottom(0).Width(340).Padding(20, 18).Gap(10).
		Background(stageInk(false)).TextColor(white).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "播放设置").FontSize(fontTitle).Bold().TextColor(white).Grow(1)
			a.iconBtn(c, "keys-open", iconList, "快捷键（?）", 44, 22, white, func() { a.px.keys = true; a.px.panel = false })
			a.iconBtn(c, "panel-close", iconClose, "关闭", 44, 22, white, func() { a.px.panel = false })
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(10).Padding(0, 4, 0, 0).Children(func() {
				if hasCtl {
					section("画面比例")
					chips(func() {
						for _, ch := range aspectChoices {
							a.stageChoice(c, th, "aspect:"+ch[1], ch[1], ch[0] == a.px.aspect, func() {
								a.setAspect(ctl, ch[0])
								a.notify("画面比例 " + ch[1])
							})
						}
					})

					all := a.tracks()
					subs := tracksOf(all, "sub")
					section("字幕")
					chips(func() {
						off := true
						for _, t := range subs {
							if t.Selected {
								off = false
							}
						}
						a.stageChoice(c, th, "sub:0", "关闭", off, func() { a.selectTrack("sub", 0, "字幕已关闭") })
						for _, t := range subs {
							a.stageChoice(c, th, fmt.Sprint("sub:", t.ID), t.Label(), t.Selected, func() {
								a.selectTrack("sub", t.ID, "字幕 "+t.Label())
							})
						}
					})
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						if stageButton(c, th, "加载字幕文件…").Clicked() {
							a.loadSubtitle()
						}
					})
					a.stepRow(c, th, "大小", fmt.Sprintf("%.0f%%", subScaleOr1(a.px.subScale)*100),
						func() { a.setSubScale(subScaleOr1(a.px.subScale) - 0.1) },
						func() { a.setSubScale(subScaleOr1(a.px.subScale) + 0.1) })
					a.stepRow(c, th, "延迟", fmt.Sprintf("%+.1f 秒", a.px.subDelay),
						func() { a.setSubDelay(a.px.subDelay - 0.5) },
						func() { a.setSubDelay(a.px.subDelay + 0.5) })

					if audio := tracksOf(all, "audio"); len(audio) > 1 {
						section("音轨")
						chips(func() {
							for _, t := range audio {
								a.stageChoice(c, th, fmt.Sprint("aid:", t.ID), t.Label(), t.Selected, func() {
									a.selectTrack("audio", t.ID, "音轨 "+t.Label())
								})
							}
						})
					}

					section("画面")
					a.stepRow(c, th, "亮度", fmt.Sprintf("%+.0f", a.px.brightness),
						func() { a.setPictureProp("brightness", a.px.brightness-5) },
						func() { a.setPictureProp("brightness", a.px.brightness+5) })
					a.stepRow(c, th, "对比度", fmt.Sprintf("%+.0f", a.px.contrast),
						func() { a.setPictureProp("contrast", a.px.contrast-5) },
						func() { a.setPictureProp("contrast", a.px.contrast+5) })
					ui.Row(c).Gap(8).Children(func() {
						if stageButton(c, th, "恢复画面").Clicked() {
							a.setPictureProp("brightness", 0)
							a.setPictureProp("contrast", 0)
							a.setAspect(ctl, "")
						}
						if stageButton(c, th, "截图（S）").Clicked() {
							a.screenshot()
						}
					})
				} else {
					ui.Text(c, "字幕、音轨、画面比例和截图需要窗口内播放（libmpv）。").FontSize(fontCaption).TextColor(muted).Wrap()
				}

				if !a.fromLive {
					section("追剧")
					auto := a.px.autoNext
					if ui.Checkbox(c, &auto, "播完自动播放下一集").Changed() {
						a.px.autoNext = auto
						a.saveSettings()
					}
					skip := a.currentSkip()
					pos, dur := 0.0, 0.0
					if a.player != nil {
						pos, dur = a.player.Position().Seconds(), a.player.Duration().Seconds()
					}
					ui.Text(c, "跳过片头片尾按这部剧记住，换集、换源都生效。").FontSize(fontCaption).TextColor(muted).Wrap()
					a.stepRow(c, th, "片头", skipLabel(skip.Intro),
						func() { skip.Intro = max(0, skip.Intro-5); a.setSkip(skip) },
						func() { skip.Intro += 5; a.setSkip(skip) })
					a.stepRow(c, th, "片尾", skipLabel(skip.Outro),
						func() { skip.Outro = max(0, skip.Outro-5); a.setSkip(skip) },
						func() { skip.Outro += 5; a.setSkip(skip) })
					ui.Row(c).Gap(8).Wrap().Children(func() {
						if stageButton(c, th, "片头到此为止").Disabled(pos <= 0).Clicked() {
							skip.Intro = pos
							a.setSkip(skip)
							a.notify("片头设为 " + clock(pos))
						}
						if stageButton(c, th, "片尾从此开始").Disabled(dur <= 0 || pos <= 0).Clicked() {
							skip.Outro = dur - pos
							a.setSkip(skip)
							a.notify("片尾设为最后 " + clock(dur-pos))
						}
						if stageButton(c, th, "清除").Disabled(skip.Intro == 0 && skip.Outro == 0).Clicked() {
							a.setSkip(skipRule{})
						}
					})
				}
			})
		})
	})
}

func subScaleOr1(v float64) float64 {
	if v <= 0 {
		return 1
	}
	return v
}

func skipLabel(sec float64) string {
	if sec <= 0 {
		return "不跳过"
	}
	return clock(sec)
}

// stepRow is a labelled value with − and + buttons.
func (a *App) stepRow(c *ui.Context, th *ui.Theme, label, value string, down, up func()) {
	white := th.AccentText
	ui.Row(c.Key("step:" + label)).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(fontCaption).TextColor(white.Alpha(0.8)).Width(56)
		if stageButton(c, th, "−").Label(label + "减少").Clicked() {
			down()
		}
		ui.Text(c, value).FontSize(fontCaption).Bold().TextColor(white).Width(78).SingleLine()
		if stageButton(c, th, "+").Label(label + "增加").Clicked() {
			up()
		}
	})
}
