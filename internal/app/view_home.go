package app

import (
	"strings"

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
				ui.Text(c, "搜索「"+strings.TrimSpace(a.committed)+"」").FontSize(26).Bold()
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
				if searching {
					title, body = "没有结果", "换个关键词，或等源检查完再搜。"
				}
				a.emptyState(c, th, title, body)
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
	row.ContextMenu(func(menu *ui.Menu) {
		if menu.Item("删除").Chosen() {
			a.dropSite(lane.Key)
		}
	})
	row.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, lane.Name).FontSize(18).Bold().SingleLine()
			ui.Column(c).Grow(1)
			ui.Text(c.Key("all:"+lane.Key), "全部").FontSize(13).TextColor(th.TextMuted).Cursor(ui.CursorPointer).OnClick(func() {
				a.selectSite(lane.Key)
			})
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
		a.siteKey = siteKey
		a.openDetail(item)
	}
	card := ui.Column(c.Key("rail:" + siteKey + ":" + item.ID)).Width(w).Gap(8).OnClick(open).Cursor(ui.CursorPointer)
	card.Children(func() {
		frame := ui.Column(c).Width(w).Height(h).Radius(8).Clip().Background(th.Surface)
		frame.Children(func() {
			a.cover(c, th, item.Pic, item.Title, w, h, open, card.Hovered())
			if item.Remarks != "" {
				ui.Column(c).Absolute().Left(8).Top(8).Padding(3, 6).Radius(4).Background(ui.RGBA(0, 0, 0, 0.62)).Children(func() {
					ui.Text(c, item.Remarks).FontSize(12).TextColor(ui.Hex("#FAFAFA")).SingleLine()
				})
			}
		})
		ui.Text(c, item.Title).FontSize(14).SingleLine()
	})
}
