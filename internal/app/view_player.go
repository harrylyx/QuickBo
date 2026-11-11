package app

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/player"
)

// rateList is the playback speed cycle, toggled by the rate button.
var rateList = []float64{0.5, 0.75, 1, 1.25, 1.5, 2, 3}

// spaceBoostDelay is how long space must be held before it turns into a
// temporary 2× boost instead of a pause/play toggle on release.
const spaceBoostDelay = 400 * time.Millisecond

// bufferingShowDelay suppresses the buffering spinner for short transients so
// a seek's quick rebuffer does not flash the overlay.
const bufferingShowDelay = 350 * time.Millisecond

func (a *App) playerKeys(c *ui.Context) {
	if a.screen != screenPlay && a.screen != screenLive {
		// Leaving the player ends the app window's fullscreen.
		if a.fullscreen {
			a.setFullscreen(false)
		}
		return
	}
	if a.player == nil {
		return
	}
	// Keep the local fullscreen flag in sync with the real state, so a toggle
	// always flips the right way even after the system (the window's green
	// button) or mpv's own OSC/Escape changed it.
	if a.embedded() {
		if a.win != nil {
			a.fullscreen = a.win.IsFullScreen()
		}
	} else {
		a.fullscreen = a.player.Fullscreen()
	}
	switch {
	case c.Shortcut(0, ui.KeySpace):
		// Live/embedded fallback: no focused element consumes space, so it
		// toggles play here. In the VOD view the stage's HandleInput takes it.
		a.togglePlay()
	case c.Shortcut(0, ui.KeyLeft):
		a.seekBy(-10)
	case c.Shortcut(0, ui.KeyRight):
		a.seekBy(10)
	case c.Shortcut(0, ui.KeyUp):
		a.nudgeVolume(0.08)
	case c.Shortcut(0, ui.KeyDown):
		a.nudgeVolume(-0.08)
	case c.Shortcut(0, ui.KeyF):
		a.toggleFull()
	case c.Shortcut(0, ui.KeyEscape):
		if a.fullscreen {
			a.setFullscreen(false)
			return
		}
		if a.screen == screenPlay {
			a.back()
		}
	}
	if a.resumeAt > 0 && a.player.Ready() {
		a.player.Seek(time.Duration(a.resumeAt * float64(time.Second)))
		a.resumeAt = 0
	}
	// Holding space past spaceBoostDelay turns it into a temporary 2× boost;
	// release restores the chosen rate (see playerInput).
	if !a.spaceDownAt.IsZero() && !a.spaceBoosted && time.Since(a.spaceDownAt) > spaceBoostDelay {
		a.spaceBoosted = true
		a.player.SetRate(2)
		a.player.Play()
	}
	if a.player.Playing() {
		a.noteProgress()
	}
	// Video frames repaint only the video element (paintVideo), not the
	// view. Keep a slow frame loop alive while playing so the progress and
	// buffer bars move, and while space is held so the boost can kick in.
	if a.player.Playing() || a.player.Buffering() || !a.spaceDownAt.IsZero() {
		c.After(250 * time.Millisecond)
	}
}

func (a *App) togglePlay() {
	if a.player == nil {
		return
	}
	if a.player.Playing() {
		a.player.Pause()
		a.noteProgress()
	} else {
		a.player.Play()
	}
	a.armControls()
}

func (a *App) seekBy(sec float64) {
	if a.player == nil {
		return
	}
	next := a.player.Position() + time.Duration(sec*float64(time.Second))
	if next < 0 {
		next = 0
	}
	if d := a.player.Duration(); d > 0 && next > d {
		next = d
	}
	a.player.Seek(next)
	a.noteProgress()
	a.armControls()
}

func (a *App) nudgeVolume(d float64) {
	v := a.volume + d
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	a.volume = v
	if a.player != nil {
		a.player.SetMuted(false)
		a.player.SetVolume(v)
	}
	a.saveSettings()
	a.armControls()
}

func (a *App) toggleMute() {
	if a.player == nil {
		return
	}
	a.player.SetMuted(!a.player.Muted())
	a.armControls()
}

func (a *App) toggleFull() {
	a.setFullscreen(!a.fullscreen)
	a.armControls()
}

// setFullscreen fullscreens the app window when the video plays inside it,
// or mpv's own window when it plays there.
func (a *App) setFullscreen(on bool) {
	a.fullscreen = on
	if a.player != nil && !a.embedded() {
		a.player.SetFullscreen(on)
		return
	}
	if a.win != nil && a.win.IsFullScreen() != on {
		a.win.SetFullScreen(on)
	}
}

// embedded reports whether the current player draws its video in the app
// window (libmpv) rather than in a window of its own (an mpv process).
func (a *App) embedded() bool {
	_, ok := a.player.(player.VideoSource)
	return ok
}

// paintVideo draws an embedded player's frames. Its Draw callback runs again
// every display frame while the video moves, without building the view: it
// tells the player the size to render at and shows the newest frame.
func (a *App) paintVideo(c *ui.Context, vs player.VideoSource) {
	if a.videoBM == nil {
		a.videoBM = ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	}
	bm := a.videoBM
	pl := a.player
	ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
		s := p.Scale()
		vs.SetVideoSize(int(r.W*s+0.5), int(r.H*s+0.5))
		if img := vs.TakeFrame(); img != nil {
			bm.Update(img)
			a.videoShown = true
		}
		if a.videoShown {
			// The frame is rendered at this element's size, letterboxed by
			// mpv, so it fills the box; Contain absorbs the width rounding.
			p.Image(bm, r, ui.Contain)
		}
		if pl.Playing() || pl.Buffering() || !pl.Ready() || !a.videoShown {
			p.AnimationFrameRate(60)
		} else {
			// Paused: frames still come after a seek or a resize.
			p.After(100 * time.Millisecond)
		}
	})
}

func (a *App) armControls() {
	a.controlsUntil = time.Now().Add(3 * time.Second)
}

func (a *App) setRate(r float64) {
	a.rate = r
	if a.player != nil {
		a.player.SetRate(r)
	}
	a.armControls()
}

// cycleRate steps through rateList, wrapping around, so the rate button is a
// single tap to change speed rather than opening an options row.
func (a *App) cycleRate() {
	i := 0
	for ; i < len(rateList); i++ {
		if rateList[i] == a.rate {
			break
		}
	}
	a.setRate(rateList[(i+1)%len(rateList)])
}

// playerInput takes the space key while the stage has focus, so a short press
// pauses on release and a hold becomes a temporary 2× boost (see playerKeys).
func (a *App) playerInput(ev ui.InputEvent) bool {
	if a.player == nil || ev.Key != ui.KeySpace {
		return false
	}
	switch ev.Kind {
	case ui.InputKeyDown:
		if ev.Repeat {
			return true
		}
		a.spaceDownAt = time.Now()
		a.spaceBoosted = false
		return true
	case ui.InputKeyUp:
		a.spaceDownAt = time.Time{}
		if a.spaceBoosted {
			a.spaceBoosted = false
			a.player.SetRate(a.rate)
		} else {
			a.togglePlay()
		}
		return true
	}
	return false
}

func (a *App) playRelative(delta int) {
	if a.detail == nil {
		return
	}
	var lineName string
	var eps []model.Episode
	for _, ln := range a.detail.Lines {
		if ln.Name != a.playLine && a.playLine != "" {
			continue
		}
		lineName = ln.Name
		eps = append(eps, ln.Episodes...)
		break
	}
	if lineName == "" && len(a.detail.Lines) > 0 {
		ln := a.detail.Lines[a.linePick]
		lineName = ln.Name
		eps = append(eps, ln.Episodes...)
	}
	next := a.playIndex + delta
	if next < 0 || next >= len(eps) {
		return
	}
	a.playEpisode(lineName, eps[next])
}

func (a *App) viewPlay(c *ui.Context, th *ui.Theme) {
	a.paintStage(c, th, false)
}

func (a *App) paintStage(c *ui.Context, th *ui.Theme, embed bool) {
	stage := ui.Column(c)
	if embed {
		stage = stage.FillWidth().Grow(1).MinHeight(320)
	} else {
		stage = stage.Fill()
	}
	// In the full-screen VOD view the stage takes the keyboard focus so space's
	// press/release reach playerInput (mpv renders in its own window and the
	// window-level shortcuts only see key-down). The live view is embedded next
	// to a search field, so it must not steal focus there.
	if a.player != nil && !embed {
		stage = stage.Focusable().FocusRing(false).Focus().HandleInput(a.playerInput)
	}
	x, y, over := stage.PointerPosition()
	if over {
		dx, dy := x-a.ptrX, y-a.ptrY
		if !a.ptrSeen || dx*dx+dy*dy > 4 {
			a.ptrSeen = true
			a.ptrX, a.ptrY = x, y
			a.armControls()
		}
	} else {
		a.ptrSeen = false
	}
	if stage.Pressed() {
		a.armControls()
	}
	if remain := time.Until(a.controlsUntil); remain > 0 {
		c.After(remain + 40*time.Millisecond)
	}
	paused := a.player != nil && a.player.Ready() && !a.player.Playing() && !a.player.Ended()
	show := a.showDrawer || a.player == nil || a.playLoading || paused || time.Now().Before(a.controlsUntil)
	if a.player != nil && a.playErr == "" {
		if err := a.player.Err(); err != nil {
			a.playErr = "播放失败：" + err.Error()
		}
	}
	// Debounce the buffering overlay so a seek's brief rebuffer does not flash.
	buffering := a.player != nil && (!a.player.Ready() || a.player.Buffering())
	showBuffering := false
	if buffering {
		if a.bufferingSince.IsZero() {
			a.bufferingSince = time.Now()
		}
		showBuffering = time.Since(a.bufferingSince) > bufferingShowDelay
	} else {
		a.bufferingSince = time.Time{}
	}
	radius := float32(0)
	if embed {
		radius = 18
	}
	stage.Radius(radius).Clip().Background(ui.Hex("#000000")).Children(func() {
		if vs, ok := a.player.(player.VideoSource); ok {
			a.paintVideo(c, vs)
		} else if a.player != nil {
			ui.Column(c).Fill().Center().Children(func() {
				ui.Text(c, "视频在 mpv 窗口中播放").FontSize(fontCaption).TextColor(th.AccentText.Alpha(0.55))
			})
		} else {
			ui.Column(c).Fill().Center().Children(func() {
				if a.playLoading {
					ui.Spinner(c)
				}
			})
		}
		if showBuffering {
			ui.Column(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Center().Children(func() {
				ui.Column(c).Padding(16).Radius(16).Background(ui.RGBA(0, 0, 0, 0.45)).Children(func() {
					ui.Spinner(c)
				})
			})
		}
		if !show && a.player != nil {
			a.thinProgress(c, th)
		}
		if show {
			a.playerChrome(c, th)
		}
		if a.playErr != "" {
			ui.Column(c).Absolute().Left(24).Right(24).Bottom(108).Children(func() {
				a.noteWithActions(c, th, a.playErr, true,
					stateAction{label: "重试", primary: true, click: a.retryPlay})
			})
		}
		if a.showDrawer {
			a.episodeDrawer(c, th)
		}
	})
}

func (a *App) thinProgress(c *ui.Context, th *ui.Theme) {
	var pos, dur float64
	if a.player != nil {
		pos = a.player.Position().Seconds()
		dur = a.player.Duration().Seconds()
	}
	if dur <= 0 {
		return
	}
	frac := pos / dur
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(2).Background(th.AccentText.Alpha(0.18)).Children(func() {
		ui.Box(c).Height(2).WidthPercent(float32(frac) * 100).Background(th.Accent)
	})
}

func (a *App) playerChrome(c *ui.Context, th *ui.Theme) {
	white := th.AccentText
	title, episode := a.overlayTitle()
	ui.Column(c).Absolute().Left(0).Right(0).Top(0).Padding(20, 28).Gap(2).
		Gradient(ui.RGBA(0, 0, 0, 0.78), ui.Transparent, 180).
		Transition(chromeFade(-8)).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			a.iconBtn(c, "back", iconBack, "返回", 44, 26, white, func() { a.back() })
			ui.Column(c).Gap(2).Shrink(1).Children(func() {
				ui.Text(c, title).FontSize(fontTitle).Bold().TextColor(white).SingleLine()
				if episode != "" {
					ui.Text(c, episode).FontSize(fontCaption).TextColor(th.AccentText.Alpha(0.78)).SingleLine()
				}
			})
		})
	})
	ui.Column(c).Absolute().Left(0).Right(0).Bottom(0).Padding(20, 28, 18, 28).Gap(12).
		Gradient(ui.Transparent, ui.RGBA(0, 0, 0, 0.88), 180).
		Transition(chromeFade(8)).Children(func() {
		a.seekBar(c, th)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			glyph := iconPlay
			tip := "播放"
			if a.player != nil && a.player.Playing() {
				glyph = iconPause
				tip = "暂停"
			}
			a.iconBtn(c, "pp", glyph, tip, 64, 34, white, a.togglePlay)
			a.iconBtn(c, "prev", iconPrev, "上一集", 44, 26, white, func() { a.playRelative(-1) })
			a.iconBtn(c, "rew", iconRew, "后退 10 秒", 44, 26, white, func() { a.seekBy(-10) })
			a.iconBtn(c, "fwd", iconFwd, "快进 10 秒", 44, 26, white, func() { a.seekBy(10) })
			a.iconBtn(c, "next", iconNext, "下一集", 44, 26, white, func() { a.playRelative(1) })
			vol := iconVol
			vt := "静音"
			if a.player != nil && a.player.Muted() {
				vol = iconMute
				vt = "取消静音"
			}
			a.iconBtn(c, "vol", vol, vt, 44, 26, white, a.toggleMute)
			slider := ui.Slider(c, &a.volume, 0, 1).Width(120)
			if a.player != nil && a.volume != a.appliedVolume {
				if a.volume > 0 {
					a.player.SetMuted(false)
				}
				a.player.SetVolume(a.volume)
				a.appliedVolume = a.volume
			}
			if !slider.Pressed() {
				a.rememberVolume()
			}
			ui.Column(c).Grow(1)
			a.rateBtn(c, th)
			a.iconBtn(c, "eps", iconList, "选集", 44, 26, white, func() { a.showDrawer = !a.showDrawer })
			a.iconBtn(c, "full", iconFull, "全屏", 44, 26, white, a.toggleFull)
		})
	})
}

func chromeFade(y float32) ui.ElementTransition {
	return ui.ElementTransition{
		Duration:     200 * time.Millisecond,
		ExitDuration: 160 * time.Millisecond,
		Ease:         ui.EaseOut,
		Enter:        &ui.Motion{Y: y, Opacity: 0, Scale: 1},
		Exit:         &ui.Motion{Y: y, Opacity: 0, Scale: 1},
	}
}

func (a *App) rateBtn(c *ui.Context, th *ui.Theme) {
	label := trimRate(a.rate)
	b := ui.Column(c.Key("speed")).Padding(6, 8).Radius(8).OnClick(a.cycleRate).Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(th.AccentText.Alpha(0.14))
	}
	b.Tooltip("倍速").Label("倍速").Children(func() {
		ui.Text(c, label).FontSize(fontCaption).Bold().TextColor(th.AccentText)
	})
}

func trimRate(r float64) string {
	if r == 1 {
		return "1×"
	}
	return fmt.Sprintf("%g×", r)
}

func (a *App) overlayTitle() (string, string) {
	title := strings.TrimSpace(a.playTitle)
	episode := ""
	if a.detail != nil && !a.fromLive && strings.TrimSpace(a.detail.Title) != "" {
		title = a.detail.Title
		episode = currentEpisodeName(a)
		if a.playLine != "" {
			label := lineLabel(a.playLine, a.linePick)
			if episode != "" {
				episode = label + "  ·  " + episode
			} else {
				episode = label
			}
		}
	}
	if title == "" {
		title = "正在播放"
	}
	return title, episode
}

func (a *App) seekBar(c *ui.Context, th *ui.Theme) {
	var pos, dur, buffered float64
	buffering := false
	if a.player != nil {
		pos = a.player.Position().Seconds()
		dur = a.player.Duration().Seconds()
		buffered = a.player.Buffered().Seconds()
		buffering = a.player.Buffering()
	}
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, clock(pos)).FontSize(fontCaption).TextColor(th.AccentText)
		// The hit target is taller than the painted track so the tooltip
		// and the click both land on the same element.
		hit := ui.Box(c.Key("seek")).Height(28).Grow(1)
		w := hit.Bounds().W
		frac := 0.0
		if dur > 0 {
			frac = pos / dur
			if frac < 0 {
				frac = 0
			}
			if frac > 1 {
				frac = 1
			}
		}
		// buffered is the real loaded range mpv reports: seconds of media ahead
		// of the playhead, derived from demuxer-cache-state's seekable-ranges.
		bufFrac := 0.0
		if dur > 0 && buffered > 0 {
			bufFrac = (pos + buffered) / dur
			if bufFrac < 0 {
				bufFrac = 0
			}
			if bufFrac > 1 {
				bufFrac = 1
			}
		}
		hit.Children(func() {
			ui.Box(c).Absolute().Left(0).Right(0).Top(12).Height(4).Radius(2).Background(th.AccentText.Alpha(0.22))
			if bufFrac > frac && w > 0 {
				ui.Box(c).Absolute().Left(0).Top(12).Height(4).Width(w * float32(bufFrac)).Radius(2).Background(th.AccentText.Alpha(0.38))
			}
			ui.Box(c).Absolute().Left(0).Top(12).Height(4).Width(w * float32(frac)).Radius(2).Background(th.Accent)
			px, _, hover := hit.PointerPosition()
			if hover && dur > 0 && w > 1 {
				at := float64(px) / float64(w) * dur
				tipW := float32(58)
				left := px - tipW/2
				if left < 0 {
					left = 0
				}
				if left > w-tipW {
					left = w - tipW
				}
				ui.Column(c).Absolute().Left(left).Top(0).Width(tipW).Height(18).Radius(4).Background(ui.RGBA(0, 0, 0, 0.82)).Center().Children(func() {
					ui.Text(c, clock(at)).FontSize(fontCaption).TextColor(th.AccentText)
				})
			}
		})
		if hit.Clicked() && dur > 0 && w > 1 && a.player != nil {
			x, _, _ := hit.PointerPosition()
			a.player.Seek(time.Duration((float64(x) / float64(w)) * dur * float64(time.Second)))
			a.noteProgress()
			a.armControls()
		}
		if dur <= 0 {
			label := "直播"
			if !a.fromLive {
				label = "00:00"
			}
			if buffering || (a.player != nil && !a.player.Ready()) {
				label = "缓冲中"
			}
			ui.Text(c, label).FontSize(fontCaption).TextColor(th.AccentText)
		} else {
			label := clock(dur)
			if buffering {
				label = "缓冲中"
			}
			ui.Text(c, label).FontSize(fontCaption).TextColor(th.AccentText)
		}
	})
}

func (a *App) episodeDrawer(c *ui.Context, th *ui.Theme) {
	ui.Column(c).Absolute().Top(0).Right(0).Bottom(0).Width(300).Padding(20, 18).Gap(12).
		Background(stageInk(false)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "选集").FontSize(fontTitle).Bold().TextColor(th.AccentText).Grow(1)
			a.iconBtn(c, "close-eps", iconBack, "关闭", 44, 22, th.AccentText, func() { a.showDrawer = false })
		})
		if a.detail == nil {
			ui.Text(c, "没有剧集列表").TextColor(th.TextMuted)
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				for _, ln := range a.detail.Lines {
					if a.playLine != "" && ln.Name != a.playLine {
						continue
					}
					for i, ep := range ln.Episodes {
						i, ep, name := i, ep, ln.Name
						label := ep.Name
						if label == "" {
							label = fmt.Sprintf("第 %d 集", i+1)
						}
						on := i == a.playIndex && name == a.playLine
						a.choice(c, th, "drawer-"+name+"-"+label, label, on, func() {
							a.playEpisode(name, ep)
							a.showDrawer = false
						})
					}
				}
			})
		})
	})
}

func (a *App) iconBtn(c *ui.Context, key string, glyph *ui.SVG, tip string, box, glyphPx float32, fg ui.Color, click func()) {
	b := ui.Column(c.Key(key)).Size(box, box).Radius(box / 2).Center().OnClick(click).Cursor(ui.CursorPointer).Tooltip(tip).Label(tip)
	// A soft disc under the pointer, and a give as it is pressed.
	bg := ui.Transparent
	if b.Hovered() {
		bg = fg.Alpha(0.14)
	}
	b.Background(b.AnimateColor("bg", bg, colorEase))
	pressPose(b, 0.88)
	b.Children(func() {
		ui.Icon(c, glyph).Size(glyphPx, glyphPx).TextColor(fg)
	})
}

func (a *App) rememberVolume() {
	if a.volume == a.savedVolume {
		return
	}
	a.savedVolume = a.volume
	a.saveSettings()
}

func clock(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	s := int(sec + 0.5)
	h, m := s/3600, (s%3600)/60
	s = s % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
