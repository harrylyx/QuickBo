package app

import (
	"fmt"
	"image"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/harrylyx/QuickBo/internal/player"
)

// rateList is the playback speed cycle, toggled by the rate button.

// spaceBoostDelay is how long space must be held before it turns into a
// temporary 2× boost instead of a pause/play toggle on release.
const spaceBoostDelay = 400 * time.Millisecond

// bufferingShowDelay suppresses the buffering spinner for short transients so
// a seek's quick rebuffer does not flash the overlay.
const bufferingShowDelay = 350 * time.Millisecond

func (a *App) playerKeys(c *ui.Context) {
	if a.screen != screenPlay && a.screen != screenLive && !(a.screen == screenLiveAgg && a.fromLive) {
		// Leaving the player ends the app window's fullscreen, and the
		// floating window.
		if a.fullscreen {
			a.setFullscreen(false)
		}
		a.exitPip()
		return
	}
	if a.player == nil {
		// While the stream is still resolving (or failed to open) there is no
		// player, but Escape must still leave the play screen, and a failed
		// live stage must be able to give the channel list back.
		if c.Shortcut(0, ui.KeyEscape) {
			switch {
			case a.screen == screenPlay:
				a.back()
			case a.liveStageOpen():
				a.leaveLiveStage()
			}
		}
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
	// On the play screen and in the floating window the focused stage takes
	// space itself (playerInput), to tell a tap from a hold. A Shortcut for
	// space claims the key for the window, and the stage would then never
	// see it go down: register it only where no stage takes it (live).
	spaceHere := a.screen != screenPlay && !a.px.pip
	switch {
	case spaceHere && c.Shortcut(0, ui.KeySpace):
		a.togglePlay()
	case c.Shortcut(0, ui.KeyLeft):
		a.seekBy(-10)
	case c.Shortcut(0, ui.KeyRight):
		a.seekBy(10)
	case c.Shortcut(ui.Shift, ui.KeyLeft):
		a.seekBy(-60)
	case c.Shortcut(ui.Shift, ui.KeyRight):
		a.seekBy(60)
	case c.Shortcut(0, ui.KeyUp):
		a.nudgeVolume(0.08)
	case c.Shortcut(0, ui.KeyDown):
		a.nudgeVolume(-0.08)
	case c.Shortcut(0, ui.KeyF):
		a.toggleFull()
	case c.Shortcut(0, ui.KeyEscape):
		switch {
		case a.px.keys:
			a.px.keys = false
		case a.px.panel:
			a.px.panel = false
		case a.px.pip:
			a.exitPip()
		case a.fullscreen:
			a.setFullscreen(false)
		case a.screen == screenPlay:
			a.back()
		}
		return
	case !a.playerOwnsKeys():
		// The live view keeps a filter field next to the picture: letters
		// typed there must not drive the player.
	case c.Shortcut(0, ui.KeyM):
		a.toggleMute()
	case c.Shortcut(0, ui.KeyBracketLeft):
		a.stepRate(-1)
	case c.Shortcut(0, ui.KeyBracketRight):
		a.stepRate(1)
	case c.Shortcut(0, ui.KeyBackslash):
		a.setRate(1)
	case c.Shortcut(0, ui.KeyN), c.Shortcut(0, ui.KeyPageDown):
		if !a.fromLive {
			a.playRelative(1)
		}
	case c.Shortcut(0, ui.KeyPageUp):
		if !a.fromLive {
			a.playRelative(-1)
		}
	case c.Shortcut(0, ui.KeyP):
		a.togglePip()
	case c.Shortcut(0, ui.KeyS):
		a.screenshot()
	case c.Shortcut(ui.Shift, ui.KeySlash), c.Shortcut(0, ui.KeySlash):
		a.px.keys = !a.px.keys
	}
	if a.player != nil {
		if a.resumeAt > 0 && a.player.Ready() {
			a.player.Seek(time.Duration(a.resumeAt * float64(time.Second)))
			a.resumeAt = 0
		}
		// Holding space past spaceBoostDelay turns it into a temporary 2× boost;
		// release restores the chosen rate (see playerInput).
		if !a.spaceDownAt.IsZero() && !a.spaceBoosted && time.Since(a.spaceDownAt) > spaceBoostDelay {
			a.startBoost()
		}
		if a.player.Playing() {
			a.noteProgress()
		}
		a.bingeTick()
		// Video frames repaint only the video element (paintVideo), not the
		// view. Keep a slow frame loop alive while playing so the progress and
		// buffer bars move, and while space is held so the boost can kick in.
		if a.player.Playing() || a.player.Buffering() || !a.spaceDownAt.IsZero() {
			c.After(250 * time.Millisecond)
		}
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
	if a.player == nil || a.fromLive {
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
	verb := "快进"
	if sec < 0 {
		verb = "后退"
	}
	a.notify(fmt.Sprintf("%s %.0f 秒  ·  %s", verb, math.Abs(sec), clock(next.Seconds())))
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
	a.notify(fmt.Sprintf("音量 %.0f%%", v*100))
}

func (a *App) toggleMute() {
	if a.player == nil {
		return
	}
	muted := !a.player.Muted()
	a.player.SetMuted(muted)
	a.armControls()
	if muted {
		a.notify("已静音")
	} else {
		a.notify(fmt.Sprintf("音量 %.0f%%", a.volume*100))
	}
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
// liveRenderSide is the longest side a live frame is drawn at. The
// software renderer copies every pixel; a 超清 frame at the window's full
// retina size does not finish before the next one, so the picture drops
// frames. The GPU scales this up to the window.
const liveRenderSide = 1920

func capLiveRender(w, h int) (int, int) {
	if w <= 0 || h <= 0 {
		return w, h
	}
	if m := max(w, h); m > liveRenderSide {
		w = w * liveRenderSide / m
		h = h * liveRenderSide / m
	}
	if w < 16 {
		w = 16
	}
	if h < 16 {
		h = 16
	}
	return w, h
}

func (a *App) paintVideo(c *ui.Context, vs player.VideoSource) {
	if a.videoBM == nil {
		a.videoBM = ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	}
	bm := a.videoBM
	pl := a.player
	live := a.fromLive
	ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
		s := p.Scale()
		w, h := int(r.W*s+0.5), int(r.H*s+0.5)
		if live {
			w, h = capLiveRender(w, h)
		}
		vs.SetVideoSize(w, h)
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
	a.notify("倍速 " + trimRate(r))
}

// playerInput takes the space key while the stage has focus, so a short press
// pauses on release and a hold becomes a temporary 2× boost (see playerKeys).
func (a *App) playerInput(ev ui.InputEvent) bool {
	if a.player == nil {
		return false
	}
	if ev.Kind == ui.InputScroll {
		// The wheel over the picture sets the volume, as in most players.
		// A trackpad sends many small steps; a wheel a few large ones.
		if d := -ev.DY / 400; d != 0 && !a.px.panel && !a.showDrawer {
			a.nudgeVolume(float64(max(-0.1, min(0.1, d))))
			return true
		}
		return false
	}
	switch ev.Key {
	case ui.KeySpace:
		return a.spaceKey(ev)
	case ui.KeyB:
		return a.boostKey(ev)
	}
	return false
}

// spaceKey: a tap pauses or plays on release, a hold plays at 2× until let
// go. The hold shows by the keyboard's auto-repeat, or spaceBoostDelay.
func (a *App) spaceKey(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputKeyDown:
		if ev.Repeat {
			// The keyboard repeating the key is the system saying it is
			// held: boost now rather than wait for the timer.
			if !a.spaceDownAt.IsZero() && !a.spaceBoosted {
				a.startBoost()
			}
			return true
		}
		a.spaceDownAt = time.Now()
		a.spaceBoosted = false
		return true
	case ui.InputKeyUp:
		a.spaceDownAt = time.Time{}
		if a.spaceBoosted {
			a.stopBoost()
		} else {
			a.togglePlay()
		}
		return true
	}
	return false
}

// boostKey: B plays at 2× for as long as it is held, at once, as it has no
// tap to tell apart.
func (a *App) boostKey(ev ui.InputEvent) bool {
	if ev.Mods != 0 {
		return false
	}
	switch ev.Kind {
	case ui.InputKeyDown:
		if !a.spaceBoosted {
			a.startBoost()
		}
		return true
	case ui.InputKeyUp:
		if a.spaceBoosted && a.spaceDownAt.IsZero() {
			a.stopBoost()
		}
		return true
	}
	return false
}

func (a *App) playRelative(delta int) {
	lineName, eps := a.episodeList()
	next := a.playIndex + delta
	if a.playIndex < 0 || next < 0 || next >= len(eps) {
		if delta > 0 {
			a.notify("已经是最后一集")
		} else {
			a.notify("已经是第一集")
		}
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
	show := a.showDrawer || a.px.panel || a.player == nil || a.playLoading || paused || time.Now().Before(a.controlsUntil)
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
		if vs, ok := a.player.(player.VideoSource); ok && !a.px.pip {
			a.paintVideo(c, vs)
			// A click on the picture plays or pauses; a double click goes
			// fullscreen. The controls drawn over it take their own clicks.
			hit := ui.Box(c.Key("video-hit")).Absolute().Left(0).Right(0).Top(0).Bottom(0)
			if hit.DoubleClicked() {
				a.toggleFull()
			} else if hit.Clicked() && !a.showDrawer && !a.px.panel {
				a.togglePlay()
			}
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
			if a.liveStageFailed() {
				ui.Column(c).Absolute().Left(24).Right(24).Top(0).Bottom(0).Center().Children(func() {
					ui.Column(c).FillWidth().Padding(20, 24).Radius(14).Gap(10).
						Background(ui.RGBA(0, 0, 0, 0.72)).Children(func() {
						ui.Text(c, "直播暂时无法播放").FontSize(fontTitle).Bold().TextColor(th.AccentText)
						ui.Text(c, a.playErr+"。已试过可用线路，仍无法打开画面。可以重试、换一个频道，或检查订阅里的直播地址。").
							FontSize(fontBody).TextColor(th.AccentText.Alpha(0.8)).Wrap()
						a.actionRow(c, []stateAction{
							{label: "重试线路", primary: true, click: a.retryLiveChannel},
							{label: "返回频道列表", click: a.leaveLiveStage},
						})
					})
				})
			} else {
				ui.Column(c).Absolute().Left(24).Right(24).Bottom(108).Children(func() {
					a.noteWithActions(c, th, a.playErr, true,
						stateAction{label: "重试", primary: true, click: a.retryPlay})
				})
			}
		}
		a.noticeLayer(c, th)
		if a.showDrawer {
			a.episodeDrawer(c, th)
		}
		if a.px.panel {
			a.settingsPanel(c, th)
		}
		if a.px.keys {
			a.keysOverlay(c, th)
		}
	})
}

func (a *App) thinProgress(c *ui.Context, th *ui.Theme) {
	// A live stream has no length, so the hairline must not creep along.
	if a.fromLive {
		return
	}
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
	// Keyed so a sibling inserted in front — the video layer, its click
	// target, the buffering spinner — cannot take this bar's place.
	// chromeFade animates position, and the bar would otherwise be drawn
	// where that sibling was and slide back over the fade.
	ui.Column(c.Key("chrome-top")).Absolute().Left(0).Right(0).Top(0).Padding(20, 28).Gap(2).
		Gradient(ui.RGBA(0, 0, 0, 0.78), ui.Transparent, 180).
		Transition(chromeFade(-8)).DragWindow().Children(func() {
		// The native title bar being hidden, the bar over the picture
		// doubles as one: its empty reaches drag the window.
		ui.Row(c).Gap(12).AlignItems(ui.Center).DragWindow().Children(func() {
			a.iconBtn(c, "back", iconBack, "返回", 44, 26, white, func() {
				if a.liveStageFailed() {
					a.leaveLiveStage()
					return
				}
				a.back()
			})
			ui.Column(c).Gap(2).Shrink(1).Children(func() {
				ui.Text(c, title).FontSize(fontTitle).Bold().TextColor(white).SingleLine().PassThrough()
				if episode != "" {
					ui.Text(c, episode).FontSize(fontCaption).TextColor(th.AccentText.Alpha(0.78)).SingleLine().PassThrough()
				}
			})
			ui.Box(c).Grow(1).PassThrough()
		})
	})
	ui.Column(c.Key("chrome-bottom")).Absolute().Left(0).Right(0).Bottom(0).Padding(20, 28, 18, 28).Gap(12).
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
			if !a.fromLive {
				a.iconBtn(c, "prev", iconPrev, "上一集", 44, 26, white, func() { a.playRelative(-1) })
				a.iconBtn(c, "rew", iconRew, "后退 10 秒", 44, 26, white, func() { a.seekBy(-10) })
				a.iconBtn(c, "fwd", iconFwd, "快进 10 秒", 44, 26, white, func() { a.seekBy(10) })
				a.iconBtn(c, "next", iconNext, "下一集", 44, 26, white, func() { a.playRelative(1) })
			}
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
			if a.fromLive && a.screen == screenLiveAgg && len(a.liveAggLines) > 0 {
				a.iconBtn(c, "eps", iconList, "线路", 44, 26, white, func() {
					a.showDrawer = !a.showDrawer
					a.px.panel = false
				})
			} else if !a.fromLive && a.detail != nil && len(a.detail.Lines) > 0 {
				a.iconBtn(c, "eps", iconList, "选集", 44, 26, white, func() {
					a.showDrawer = !a.showDrawer
					a.px.panel = false
					if a.showDrawer {
						a.syncDrawerLine()
					}
				})
			}
			a.iconBtn(c, "panel", iconSliders, "播放设置：字幕、画面、跳过片头片尾", 44, 24, white, func() {
				a.px.panel = !a.px.panel
				a.showDrawer = false
			})
			if a.embedded() {
				a.iconBtn(c, "pip", iconPip, "悬浮小窗（P）", 44, 24, white, a.togglePip)
			}
			a.iconBtn(c, "full", iconFull, "全屏（F）", 44, 26, white, a.toggleFull)
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

// rateBtn cycles the speed and shows the current one. It stays on the bar
// because changing speed is a frequent tap, not a trip into the panel.
func (a *App) rateBtn(c *ui.Context, th *ui.Theme) {
	label := trimRate(a.rate)
	b := ui.Column(c.Key("speed")).Height(44).Padding(0, 10).Radius(8).Center().
		OnClick(a.cycleRate).Cursor(ui.CursorPointer).Tooltip("倍速").Label("倍速")
	bg := ui.Transparent
	if b.Hovered() {
		bg = th.AccentText.Alpha(0.14)
	}
	b.Background(bg).Children(func() {
		ui.Text(c, label).FontSize(fontBody).Bold().TextColor(th.AccentText)
	})
}

// cycleRate steps through rateList, wrapping around.
func (a *App) cycleRate() {
	i := 0
	for ; i < len(rateList); i++ {
		if rateList[i] == a.rate {
			break
		}
	}
	a.setRate(rateList[(i+1)%len(rateList)])
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
	if a.fromLive && a.liveAggName != "" {
		title = a.liveAggName
		episode = a.liveLineLabelAt(a.playURL)
	} else if a.detail != nil && !a.fromLive && strings.TrimSpace(a.detail.Title) != "" {
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
	if a.fromLive {
		a.liveMark(c, th)
		return
	}
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
		// Dragging along the bar previews where it will land, and seeks there
		// on release.
		px0, _, _ := hit.PointerPosition()
		if _, _, held := hit.Dragged(); held && dur > 0 && w > 1 {
			a.px.seekDragging = true
			a.px.seekDragAt = max(0, min(dur, float64(px0)/float64(w)*dur))
			a.armControls()
		} else if a.px.seekDragging {
			a.px.seekDragging = false
			if a.player != nil {
				a.player.Seek(time.Duration(a.px.seekDragAt * float64(time.Second)))
				a.noteProgress()
			}
		}
		if a.px.seekDragging && dur > 0 {
			frac = a.px.seekDragAt / dur
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

// liveMark stands in for the seek bar. The playhead of a live stream only
// counts how long this player has been open, so the bar stays still and
// shows that it is live.
func (a *App) liveMark(c *ui.Context, th *ui.Theme) {
	label := "直播"
	if a.player != nil && (a.player.Buffering() || !a.player.Ready()) {
		label = "缓冲中"
	}
	ui.Row(c).Height(28).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(fontCaption).TextColor(th.AccentText)
	})
}

func (a *App) episodeDrawer(c *ui.Context, th *ui.Theme) {
	if a.fromLive && a.screen == screenLiveAgg {
		a.liveLineDrawer(c, th)
		return
	}
	ui.Column(c).Absolute().Top(0).Right(0).Bottom(0).Width(300).Padding(20, 18).Gap(12).
		Background(stageInk(false)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "选集").FontSize(fontTitle).Bold().TextColor(th.AccentText).Grow(1)
			a.iconBtn(c, "close-eps", iconBack, "关闭", 44, 22, th.AccentText, func() { a.showDrawer = false })
		})
		if a.detail == nil {
			ui.Text(c, "没有剧集列表").TextColor(th.AccentText.Alpha(0.7))
			return
		}
		if len(a.detail.Lines) > 1 {
			ui.ScrollBoth(c).MaxHeight(52).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					for i, ln := range a.detail.Lines {
						i := i
						a.stageChoice(c, th, "drawer-line:"+ln.Name, lineLabel(ln.Name, i), i == a.linePick, func() { a.linePick = i })
					}
				})
			})
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				if a.linePick < 0 || a.linePick >= len(a.detail.Lines) {
					a.linePick = 0
				}
				ln := a.detail.Lines[a.linePick]
				for i, ep := range ln.Episodes {
					i, ep, name := i, ep, ln.Name
					label := ep.Name
					if label == "" {
						label = fmt.Sprintf("第 %d 集", i+1)
					}
					on := i == a.playIndex && name == a.playLine
					a.stageChoice(c, th, "drawer-"+name+"-"+label, label, on, func() {
						a.playEpisode(name, ep)
						a.showDrawer = false
					})
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

func (a *App) syncDrawerLine() {
	if a.detail == nil {
		return
	}
	for i, ln := range a.detail.Lines {
		if ln.Name == a.playLine {
			a.linePick = i
			return
		}
	}
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

// playerOwnsKeys reports whether letter and punctuation keys belong to the
// player: on its own screen, in fullscreen, or as the floating window.
func (a *App) playerOwnsKeys() bool {
	return a.screen == screenPlay || a.fullscreen || a.px.pip
}

// startBoost plays at 2× while space is held; letting go restores the
// chosen speed (playerInput).
func (a *App) startBoost() {
	if a.player == nil {
		return
	}
	a.spaceBoosted = true
	a.player.SetRate(2)
	a.player.Play()
	a.armControls()
}

// stopBoost goes back to the chosen speed.
func (a *App) stopBoost() {
	a.spaceBoosted = false
	if a.player != nil {
		a.player.SetRate(a.rate)
	}
}
