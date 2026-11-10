package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
)

func (a *App) playerKeys(c *ui.Context) {
	if a.screen != screenPlay && a.screen != screenLive {
		return
	}
	if a.player == nil {
		return
	}
	switch {
	case c.Shortcut(0, ui.KeySpace):
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
		if a.win != nil && a.win.IsFullScreen() {
			a.win.SetFullScreen(false)
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
	if a.player.Playing() {
		a.noteProgress()
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
	if a.win != nil {
		a.win.ToggleFullScreen()
	}
	a.armControls()
}

func (a *App) armControls() {
	a.controlsUntil = time.Now().Add(3 * time.Second)
}

func (a *App) setRate(r float64) {
	a.rate = r
	if a.player != nil {
		a.player.SetRate(r)
	}
	a.showRates = false
	a.armControls()
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
	a.paintStage(c, th, true)
}

func (a *App) paintStage(c *ui.Context, th *ui.Theme, docked bool) {
	stage := ui.Box(c).FillWidth()
	if docked {
		stage = stage.Grow(1).MinHeight(420)
	} else {
		stage = stage.Height(280)
	}
	_, _, over := stage.PointerPosition()
	if over {
		a.armControls()
		c.After(3 * time.Second)
	}
	show := over || a.showDrawer || a.showRates || a.player == nil || a.playLoading || time.Now().Before(a.controlsUntil)
	if a.player != nil && !a.player.Playing() {
		show = true
	}
	stage.Radius(18).Clip().Background(stageInk(a.light)).Children(func() {
		if a.player != nil {
			ui.Video(c, a.player).Grow(1).Fit(ui.Contain)
		} else {
			ui.Column(c).Fill().Center().Children(func() {
				if a.playLoading {
					ui.Spinner(c)
				}
			})
		}
		if a.player != nil && (!a.player.Ready() || a.player.Buffering()) {
			ui.Column(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Center().Children(func() {
				ui.Column(c).Padding(16).Radius(16).Background(ui.RGBA(0, 0, 0, 0.45)).Children(func() {
					ui.Spinner(c)
				})
			})
		}
		if show {
			a.playerChrome(c, th, stage)
		}
		if a.showDrawer {
			a.episodeDrawer(c, th)
		}
	})
	if a.playErr != "" {
		a.note(c, th, a.playErr, true)
	}
}

func (a *App) playerChrome(c *ui.Context, th *ui.Theme, stage ui.Element) {
	ui.Column(c).Absolute().Left(0).Right(0).Top(0).Padding(16, 18).
		Gradient(ui.RGBA(0, 0, 0, 0.72), ui.Transparent, 180).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			a.iconBtn(c, "back", iconBack, "返回", func() { a.back() })
			ui.Text(c, a.playTitle).FontSize(16).Bold().TextColor(ui.Hex("#fafafa")).Shrink(1)
		})
	})
	ui.Column(c).Absolute().Left(0).Right(0).Bottom(0).Padding(12, 16, 14, 16).Gap(8).
		Gradient(ui.Transparent, ui.RGBA(0, 0, 0, 0.82), 180).Children(func() {
		a.seekBar(c, th, stage)
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			glyph := iconPlay
			tip := "播放"
			if a.player != nil && a.player.Playing() {
				glyph = iconPause
				tip = "暂停"
			}
			a.iconBtn(c, "pp", glyph, tip, a.togglePlay)
			a.iconBtn(c, "prev", iconPrev, "上一集", func() { a.playRelative(-1) })
			a.iconBtn(c, "rew", iconRew, "后退 10 秒", func() { a.seekBy(-10) })
			a.iconBtn(c, "fwd", iconFwd, "快进 10 秒", func() { a.seekBy(10) })
			a.iconBtn(c, "next", iconNext, "下一集", func() { a.playRelative(1) })
			vol := iconVol
			vt := "音量"
			if a.player != nil && a.player.Muted() {
				vol = iconMute
				vt = "取消静音"
			}
			a.iconBtn(c, "vol", vol, vt, a.toggleMute)
			ui.Column(c).Grow(1)
			a.rateBtn(c)
			a.iconBtn(c, "eps", iconList, "选集", func() { a.showDrawer = !a.showDrawer })
			a.iconBtn(c, "full", iconFull, "全屏", a.toggleFull)
			a.iconBtn(c, "mpv", iconPlay, "用 mpv 播放", a.openExternal)
		})
		if a.showRates {
			ui.Row(c).Gap(6).Children(func() {
				for _, r := range []float64{0.5, 0.75, 1, 1.25, 1.5, 2} {
					r := r
					label := trimRate(r)
					a.choice(c, th, "rate-"+label, label, a.rate == r, func() { a.setRate(r) })
				}
			})
		}
		if a.detail != nil && len(a.detail.Lines) > 1 {
			ui.Row(c).Gap(6).Children(func() {
				for i, ln := range a.detail.Lines {
					i, name := i, ln.Name
					a.choice(c, th, "route-"+name, name, name == a.playLine, func() {
						a.linePick = i
						if len(ln.Episodes) > 0 {
							a.playEpisode(name, ln.Episodes[0])
						}
					})
				}
			})
		}
	})
}

func (a *App) rateBtn(c *ui.Context) {
	label := trimRate(a.rate)
	b := ui.Column(c.Key("speed")).Padding(6, 8).Radius(8).OnClick(func() { a.showRates = !a.showRates }).Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(ui.RGBA(255, 255, 255, 0.14))
	}
	b.Tooltip("倍速").Label("倍速").Children(func() {
		ui.Text(c, label).FontSize(13).Bold().TextColor(ui.Hex("#fafafa"))
	})
}

func trimRate(r float64) string {
	if r == 1 {
		return "1×"
	}
	return fmt.Sprintf("%g×", r)
}

func (a *App) seekBar(c *ui.Context, th *ui.Theme, _ ui.Element) {
	var pos, dur float64
	if a.player != nil {
		pos = a.player.Position().Seconds()
		dur = a.player.Duration().Seconds()
	}
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, clock(pos)).FontSize(12).TextColor(ui.Hex("#fafafa"))
		bar := ui.Box(c.Key("seek")).Height(18).Grow(1).Radius(9).Background(ui.RGBA(255, 255, 255, 0.22))
		w := bar.Bounds().W
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
		bar.Children(func() {
			fill := w * float32(frac)
			if a.player != nil && a.player.Buffering() && w > 0 {
				buf := fill + w*0.08
				if buf > w {
					buf = w
				}
				ui.Box(c).Absolute().Left(0).Top(6).Height(6).Width(buf).Radius(3).Background(ui.RGBA(255, 255, 255, 0.35))
			}
			ui.Box(c).Absolute().Left(0).Top(6).Height(6).Width(fill).Radius(3).Background(th.Accent)
		})
		if bar.Clicked() && dur > 0 && w > 1 {
			x, _, _ := bar.PointerPosition()
			a.player.Seek(time.Duration((float64(x) / float64(w)) * dur * float64(time.Second)))
			a.noteProgress()
			a.armControls()
		}
		total := dur
		if total <= 0 {
			ui.Text(c, "直播").FontSize(12).TextColor(ui.Hex("#fafafa"))
		} else {
			ui.Text(c, clock(total)).FontSize(12).TextColor(ui.Hex("#fafafa"))
		}
	})
}

func (a *App) episodeDrawer(c *ui.Context, th *ui.Theme) {
	ui.Column(c).Absolute().Top(0).Right(0).Bottom(0).Width(280).Padding(16).Gap(8).
		Background(ui.RGBA(12, 12, 14, 0.92)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "选集").Bold().TextColor(ui.Hex("#fafafa")).Grow(1)
			a.iconBtn(c, "close-eps", iconBack, "关闭", func() { a.showDrawer = false })
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

func (a *App) iconBtn(c *ui.Context, key string, glyph *ui.SVG, tip string, click func()) {
	b := ui.Column(c.Key(key)).Size(36, 36).Radius(18).Center().OnClick(click).Cursor(ui.CursorPointer).Tooltip(tip).Label(tip)
	if b.Hovered() {
		b.Background(ui.RGBA(255, 255, 255, 0.16))
	}
	b.Children(func() {
		ui.Icon(c, glyph).Size(18, 18).TextColor(ui.Hex("#fafafa"))
	})
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
