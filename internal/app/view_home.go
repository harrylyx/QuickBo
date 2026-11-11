package app

import (
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
		ui.Column(c).Padding(8, pagePad, 28, pagePad).Gap(28).Children(func() {
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
			ui.Text(c, lane.Name).FontSize(fontTitle).Bold().SingleLine()
			ui.Column(c).Grow(1)
			// Rows are keyed by homepage rule; only a row named after a
			// source has a list to open.
			if a.catalog != nil && a.catalog.Find(lane.Key) != nil {
				link := ui.Row(c.Key("all:" + lane.Key)).Gap(4).AlignItems(ui.Center).
					OnClick(func() { a.selectSite(lane.Key) }).Cursor(ui.CursorPointer).
					Tooltip("查看全部分类").Label("全部")
				fg := th.TextMuted
				if link.Hovered() {
					fg = th.Text
				}
				link.Transition(ui.ElementTransition{Duration: 150 * time.Millisecond}).Children(func() {
					ui.Text(c, "全部").FontSize(fontCaption).TextColor(fg).SingleLine()
					ui.Icon(c, iconChevronRight).Size(12, 12).TextColor(fg)
				})
			}
		})
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
	open := func() {
		if strings.HasPrefix(item.ID, titlePrefix) {
			a.openTitle(strings.TrimPrefix(item.ID, titlePrefix))
			return
		}
		a.siteKey = siteKey
		a.openDetail(item)
	}
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
		ui.Text(c, item.Title).FontSize(fontBody).SingleLine()
	})
}
