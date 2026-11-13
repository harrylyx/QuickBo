package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
)

func (a *App) viewHome(c *ui.Context, th *ui.Theme) {
	lanes := a.home
	searching := strings.TrimSpace(a.committed) != ""
	if searching {
		lanes = a.homeHits
	}
	ui.Scroll(c).Grow(1).TrackScroll(&a.browseScroll).Children(func() {
		ui.Column(c).Padding(20, pagePad, 28, pagePad).Gap(32).Children(func() {
			// A reload keeps the old rows on screen until sources report in, so
			// say it at the top where it can be seen, not only below the rows.
			if a.loading && !searching && len(lanes) > 0 {
				a.loadingRow(c, th, a.loadNote)
			}
			if !searching {
				a.homeHero(c, th)
			}
			a.viewContinue(c, th)
			if searching {
				ui.Text(c, "搜索「"+strings.TrimSpace(a.committed)+"」").FontSize(fontHero).Bold()
			}
			if a.homeSearching && len(lanes) == 0 {
				a.loadingRow(c, th, "正在搜索全部片源…")
				return
			}
			if !searching && a.loading && len(lanes) == 0 {
				a.loadingRow(c, th, "正在从各个源收集节目…")
				ui.Text(c, "列出内容的源会逐个出现在下面，打不开的不会进侧栏。").TextColor(th.TextMuted).Wrap()
				return
			}
			if len(lanes) == 0 {
				title, body := "没有节目", "这些源的第一页是空的。"
				actions := []stateAction{
					{label: "重新采集", primary: true, click: a.recollect},
					{label: "管理订阅", click: a.openSubs},
				}
				if searching {
					title, body = "没有结果", "换个关键词，或等源检查完再搜。"
					actions = []stateAction{
						{label: "清空搜索", primary: true, click: func() {
							a.query = ""
							a.committed = ""
							a.homeHits = nil
							a.homeSearching = false
						}},
						{label: "重新采集", click: a.recollect},
					}
				}
				a.emptyState(c, th, title, body, actions...)
				return
			}
			for _, lane := range lanes {
				a.homeRail(c, th, lane)
			}
			if a.loading && !searching {
				a.loadingRow(c, th, a.loadNote)
			}
		})
	})
}

func (a *App) homeRail(c *ui.Context, th *ui.Theme, lane homeLane) {
	row := ui.Column(c.Key("rail:" + lane.Key)).Gap(12)
	row.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, lane.Name).FontSize(fontHeadline).FontWeight(weightTitle).SingleLine()
			ui.Column(c).Grow(1)
			// Rows are keyed by homepage rule; only a row named after a
			// source has a list to open.
			if a.canMore(lane) {
				link := ui.Row(c.Key("all:" + lane.Key)).Gap(4).AlignItems(ui.Center).
					OnClick(func() { a.openMore(lane) }).Cursor(ui.CursorPointer).
					Tooltip("查看更多").Label("查看更多")
				fg := th.TextMuted
				if link.Hovered() {
					fg = th.Text
				}
				link.Transition(ui.ElementTransition{Duration: 150 * time.Millisecond}).Children(func() {
					ui.Text(c, "查看更多").FontSize(fontCaption).TextColor(fg).SingleLine()
					ui.Icon(c, iconChevronRight).Size(12, 12).TextColor(fg)
				})
			}
		})
		// A poster that failed to download or decode is a blank card too, so it
		// leaves the row; a poster still loading stays until it fails. Titles
		// with no poster address were already ranked last by the lane builder
		// and only fill a short row, so they stay.
		shown := make([]model.Item, 0, len(lane.Items))
		for _, it := range lane.Items {
			if pic := model.ResolveURL("", it.Pic); pic != "" && a.failed[pic] {
				continue
			}
			shown = append(shown, it)
		}
		lane.Items = shown
		if len(lane.Items) == 0 {
			return
		}
		ui.ScrollBoth(c).MaxHeight(236).Children(func() {
			ui.Row(c).Gap(12).Children(func() {
				for _, item := range lane.Items {
					a.railCard(c, th, lane.Key, item)
				}
			})
		})
	})
}

func (a *App) railCard(c *ui.Context, th *ui.Theme, siteKey string, item model.Item) {
	const w, h float32 = 132, 198
	open := func() { a.openItem(siteKey, item) }
	card := ui.Column(c.Key("rail:" + siteKey + ":" + item.ID)).Width(w).Gap(8).OnClick(open).Cursor(ui.CursorPointer)
	hot := card.Hovered()
	card.Children(func() {
		frame := ui.Column(c.Key("rail-frame:" + siteKey + ":" + item.ID)).Width(w).Height(h).Radius(8).Clip().Background(th.Surface)
		a.animateCard(card, frame, hot, a.light)
		frame.Children(func() {
			a.cover(c, th, item.Pic, item.Title, w, h, open, hot)
			if item.Remarks != "" {
				ui.Column(c).Absolute().Left(8).Top(8).Padding(3, 6).Radius(4).Background(ui.RGBA(0, 0, 0, 0.62)).Children(func() {
					ui.Text(c, item.Remarks).FontSize(fontCaption).TextColor(th.AccentText).SingleLine()
				})
			}
		})
		ui.Text(c, item.Title).FontSize(fontBody).FontWeight(weightStrong).SingleLine()
	})
}

// The home banner rotates through the top of 最新影视.
const (
	heroCount  = 5
	heroHeight = 380
	heroEvery  = 6 * time.Second
	heroFade   = 600 * time.Millisecond
)

// heroItems is what the banner shows: the first titles of 最新影视 (or of
// the first row) that have a poster to fill it.
func (a *App) heroItems() (string, []model.Item) {
	if len(a.home) == 0 {
		return "", nil
	}
	lane := a.home[0]
	for _, l := range a.home {
		if l.Key == latestRuleID {
			lane = l
			break
		}
	}
	var out []model.Item
	for _, it := range lane.Items {
		if pic := model.ResolveURL("", it.Pic); pic != "" && !a.failed[pic] {
			out = append(out, it)
		}
		if len(out) == heroCount {
			break
		}
	}
	return lane.Key, out
}

func (a *App) heroStep(delta, n int) {
	if n <= 0 {
		return
	}
	a.heroPrev = a.heroIdx
	a.heroIdx = ((a.heroIdx+delta)%n + n) % n
	a.heroAt = time.Now()
}

// openItem opens a card's title: the aggregated one across sources for
// home rows, or the source's own detail.
func (a *App) openItem(siteKey string, item model.Item) {
	if strings.HasPrefix(item.ID, titlePrefix) {
		a.openTitle(strings.TrimPrefix(item.ID, titlePrefix))
		return
	}
	a.siteKey = siteKey
	a.openDetail(item)
}

// homeHero is the banner atop the home screen: a title of 最新影视 at a
// time on its own blurred poster, crossfading to the next every heroEvery
// unless the pointer rests on it, with arrows and dots to pick one.
func (a *App) homeHero(c *ui.Context, th *ui.Theme) {
	laneKey, items := a.heroItems()
	n := len(items)
	if n == 0 {
		return
	}
	if a.heroIdx >= n {
		a.heroIdx = 0
	}
	if a.heroPrev >= n {
		a.heroPrev = a.heroIdx
	}
	if a.heroAt.IsZero() {
		a.heroAt = time.Now()
	}
	hero := ui.Box(c.Key("hero")).FillWidth().Height(heroHeight).Radius(20).Clip().Background(ui.Hex("#141418")).
		Shadow(0, 18, 40, -18, ui.RGBA(0, 0, 0, 0.6))
	hovered := hero.Hovered()
	since := time.Since(a.heroAt)
	if n > 1 && !hovered {
		if since >= heroEvery {
			a.heroStep(1, n)
			since = 0
		} else {
			c.After(heroEvery - since)
		}
	}
	cross := float32(1)
	if since < heroFade && a.heroPrev != a.heroIdx {
		cross = float32(since) / float32(heroFade)
		cross = cross * cross * (3 - 2*cross) // smoothstep
		c.After(16 * time.Millisecond)
	}
	cur := items[a.heroIdx]
	// The banner's text is always light on a darkened poster, in either
	// appearance, as a cinema's marquee is.
	white := ui.Hex("#ffffff")
	hero.Children(func() {
		if cross < 1 {
			a.heroBackdrop(c, items[a.heroPrev], 1)
		}
		a.heroBackdrop(c, cur, cross)
		ui.Row(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Padding(40, 56, 48, 56).Gap(40).AlignItems(ui.Center).Children(func() {
			ui.Column(c.Key("hero-copy:" + cur.ID)).Grow(1).Shrink(1).Gap(14).Opacity(0.25 + 0.75*cross).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Row(c).Padding(4, 10).Radius(999).Background(th.Accent).Children(func() {
						ui.Text(c, "最新影视").FontSize(fontCaption).FontWeight(weightTitle).TextColor(th.AccentText).SingleLine()
					})
					if hits := len(a.lookup[strings.TrimPrefix(cur.ID, titlePrefix)]); hits > 1 {
						ui.Text(c, fmt.Sprintf("%d 个片源", hits)).FontSize(fontCaption).TextColor(white.Alpha(0.7)).SingleLine()
					}
				})
				ui.Text(c, cur.Title).FontSize(40).FontWeight(weightHeavy).TextColor(white).MaxLines(2).Wrap()
				meta := joinMeta(cur.Year, cur.Remarks)
				if meta != "" {
					ui.Text(c, meta).FontSize(fontBody).TextColor(white.Alpha(0.72)).SingleLine()
				}
				ui.Row(c).Padding(10, 0, 0, 0).Children(func() {
					if primaryPill(c, th, "hero-open", iconPlay, "查看详情").Clicked() {
						a.openItem(laneKey, cur)
					}
				})
			})
			a.heroPoster(c, th, cur, laneKey)
		})
		if n > 1 {
			if hovered {
				for _, side := range []struct {
					key, tip string
					icon     *ui.SVG
					delta    int
					left     bool
				}{{"hero-prev", "上一部", iconBack, -1, true}, {"hero-next", "下一部", iconChevronRight, 1, false}} {
					side := side
					holder := ui.Column(c).Absolute().Top(heroHeight/2 - 20)
					if side.left {
						holder = holder.Left(12)
					} else {
						holder = holder.Right(12)
					}
					holder.Children(func() {
						glassButton(c, th, side.key, side.icon, side.tip, 40, white, func() { a.heroStep(side.delta, n) })
					})
				}
			}
			ui.Row(c).Absolute().Left(56).Bottom(22).Gap(6).AlignItems(ui.Center).Children(func() {
				for i := range items {
					i := i
					dot := ui.Box(c.Key(fmt.Sprintf("hero-dot:%d", i))).Height(6).Radius(3).Cursor(ui.CursorPointer).
						OnClick(func() {
							if i != a.heroIdx {
								a.heroPrev, a.heroIdx, a.heroAt = a.heroIdx, i, time.Now()
							}
						})
					w, fill := float32(6), white.Alpha(0.35)
					if i == a.heroIdx {
						w, fill = 22, white
					}
					dot.Width(dot.AnimateTo("w", w, ui.Snappy)).Background(dot.AnimateColor("bg", fill, colorEase))
				}
			})
		}
	})
}

// heroBackdrop fills the banner with item's blurred poster, opacity op,
// under a scrim that darkens the left (behind the text) and the bottom.
func (a *App) heroBackdrop(c *ui.Context, item model.Item, op float32) {
	pic := model.ResolveURL("", item.Pic)
	a.poster(pic)
	art, ok := a.artOf(pic)
	layer := ui.Box(c.Key("hero-bg:" + item.ID)).Absolute().Left(0).Right(0).Top(0).Bottom(0).Opacity(op)
	in := fadeIn(layer, ok)
	black := ui.Hex("#000000")
	deep := ui.Hex("#141418")
	if ok {
		deep = art.tint.MixOklab(black, 0.72)
	}
	layer.Children(func() {
		if ok {
			ui.Image(c, art.backdrop).Absolute().Left(0).Right(0).Top(0).Bottom(0).Fit(ui.Cover).Opacity(in)
		}
		ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 90, Oklab: true,
			Stops: []ui.GradientStop{
				{Color: deep.Alpha(0.92), At: 0},
				{Color: deep.Alpha(0.6), At: 0.5},
				{Color: deep.Alpha(0.15), At: 1},
			},
		})
		ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 180,
			Stops: []ui.GradientStop{
				{Color: black.Alpha(0), At: 0.55},
				{Color: black.Alpha(0.45), At: 1},
			},
		})
	})
}

// heroPoster is the banner's sharp poster, lifting a little under the
// pointer and opening the title when clicked.
func (a *App) heroPoster(c *ui.Context, th *ui.Theme, item model.Item, laneKey string) {
	const w, h float32 = 200, 300
	pic := model.ResolveURL("", item.Pic)
	frame := ui.Column(c.Key("hero-poster:"+item.ID)).Width(w).Height(h).Radius(14).Clip().Shrink(0).
		Border(1, ui.RGBA(255, 255, 255, 0.12)).Cursor(ui.CursorPointer).OnClick(func() { a.openItem(laneKey, item) })
	pose := ui.Pose{}
	if frame.Hovered() {
		pose.Y, pose.Scale = -4, 1.02
	}
	frame.AnimatePose(pose, ui.Snappy).Shadow(0, 24, 48, -10, ui.RGBA(0, 0, 0, 0.6))
	frame.Children(func() {
		a.cover(c, th, pic, item.Title, w, h, nil, false)
	})
}
