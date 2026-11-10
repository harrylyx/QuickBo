package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/live"
	"github.com/harrylyx/QuickBo/internal/model"
)

// View draws one frame. The chrome is a dark cinema layout: a narrow
// source sidebar, a hero over the poster wall, and pages that share one
// type scale and one accent.
func (a *App) View(c *ui.Context) {
	th := a.useTheme(c)
	a.playerKeys(c)
	if a.catalog == nil {
		a.viewWelcome(c, th)
	} else if a.screen == screenPlay {
		ui.Box(c).Fill().Background(ui.Hex("#000000")).Children(func() {
			a.viewPlay(c, th)
		})
	} else {
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			a.viewSidebar(c, th)
			ui.Column(c).Grow(1).Padding(22, 28, 18, 28).Gap(16).Children(func() {
				a.viewTop(c, th)
				switch a.screen {
				case screenHome:
					a.viewBrowse(c, th)
				case screenHistory:
					a.viewHistory(c, th)
				case screenFav:
					a.viewFavs(c, th)
				case screenIndex:
					a.viewIndex(c, th)
				case screenUnsupported:
					a.viewUnsupported(c, th)
				case screenDetail:
					a.viewDetail(c, th)
				case screenLive:
					a.viewLive(c, th)
				case screenEmpty:
					a.emptyState(c, th, "这份配置是空的", "没有点播站，也没有直播。换一份配置，或往里面加上站点。")
				default:
					a.viewBrowse(c, th)
				}
			})
		})
	}
	a.viewAdd(c, th)
}

func (a *App) viewWelcome(c *ui.Context, th *ui.Theme) {
	ui.Box(c).Fill().Children(func() {
		ui.Box(c).Fill().RadialGradient(ui.RadialGradient{
			Center: ui.Point{X: 0.5, Y: 0.18},
			Radius: 520,
			Stops: []ui.GradientStop{
				{th.Accent.Alpha(0.22), 0},
				{ui.Transparent, 1},
			},
		})
		ui.Column(c).Fill().Center().Children(func() {
			ui.Column(c).Width(560).Gap(14).Children(func() {
				ui.Text(c, "QUICKBO").FontSize(13).Bold().TextColor(th.Accent)
				ui.Text(c, "QuickBo").FontSize(52).Bold()
				ui.Text(c, "一台不带片源的播放器。").FontSize(20).TextColor(th.TextMuted)
				ui.Text(c, "不内置任何片源。把你自己的配置 URL 或本地 JSON 加进来，再浏览、搜索和播放。").TextColor(th.TextMuted).Wrap()
				ui.Text(c, "JSON / XML CMS、一部分 JS（drpy）和 XBPQ 规则可以直接用。JAR/DEX 爬虫会列出来，但本版不执行 jar。").TextColor(th.TextMuted).Wrap()
				if a.loading {
					a.loadingRow(c, th, "正在读取配置…")
				}
				if a.loadErr != "" {
					a.note(c, th, a.loadErr, true)
				}
				ui.Row(c).Gap(10).Padding(8, 0, 0, 0).Children(func() {
					if ui.PrimaryButton(c, "添加配置").Clicked() {
						a.adding = true
					}
				})
			})
		})
	})
}

func (a *App) viewAdd(c *ui.Context, th *ui.Theme) {
	ui.Modal(c, &a.adding, func() {
		ui.Column(c).Width(440).Gap(14).Children(func() {
			ui.Text(c, "配置").FontSize(22).Bold()
			ui.Text(c, "TVBox / FongMi 订阅的 URL，或本机 JSON。QuickBo 不内置任何片源。").TextColor(th.TextMuted).Wrap()
			ui.Text(c, "配置 URL").FontSize(12).Bold().TextColor(th.TextMuted)
			field := ui.TextInput(c, &a.urlDraft).Placeholder("https://example.com/tvbox.json").Label("配置 URL")
			if field.Submitted() {
				a.adding = false
				a.Load(a.urlDraft)
			}
			ui.Text(c, "外观").FontSize(12).Bold().TextColor(th.TextMuted)
			ui.Row(c).Gap(8).Children(func() {
				a.choice(c, th, "外观深色", "深色", !a.light, func() {
					a.light = false
					a.saveSettings()
				})
				a.choice(c, th, "外观浅色", "浅色", a.light, func() {
					a.light = true
					a.saveSettings()
				})
			})
			if ui.Checkbox(c, &a.preferExternal, "优先使用 mpv 播放").Changed() {
				a.saveSettings()
			}
			if a.loadErr != "" && a.catalog != nil {
				a.note(c, th, a.loadErr, true)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.adding = false
				}
				if ui.Button(c, "打开文件").Clicked() {
					a.pickFile()
				}
				if ui.PrimaryButton(c, "加载").Clicked() {
					a.adding = false
					a.Load(a.urlDraft)
				}
			})
		})
	})
}

func (a *App) viewSidebar(c *ui.Context, th *ui.Theme) {
	ui.Column(c).Width(236).FillHeight().Background(sidebarInk(a.light)).Padding(18, 12, 12, 12).Gap(14).Children(func() {
		ui.Column(c).Gap(2).Padding(4, 8, 0, 8).Children(func() {
			ui.Text(c, "QuickBo").FontSize(20).Bold()
			ui.Text(c, "自备片源").FontSize(12).TextColor(th.TextMuted)
		})
		side := ui.Sidebar(c, &a.sideSel, func() {
			ui.SidebarSection(c, "资料库", nil, func() {
				ui.SidebarItem(c, "home", nil, "首页")
				ui.SidebarItem(c, "history", nil, "历史")
				ui.SidebarItem(c, "fav", nil, "收藏")
			})
			if a.catalog != nil && len(a.catalog.Sites) > 0 {
				ui.SidebarSection(c, "点播", nil, func() {
					for _, s := range a.catalog.Sites {
						m := s.Meta()
						item := ui.SidebarItem(c, "site:"+m.Key, nil, m.Name)
						if m.Level == config.LevelNo {
							item.Children(func() {
								ui.Text(c, "不可用").FontSize(11).TextColor(th.TextMuted)
							})
						}
					}
				})
			}
			if a.catalog != nil && len(a.catalog.Lives) > 0 {
				ui.SidebarSection(c, "直播", nil, func() {
					for i, lv := range a.catalog.Lives {
						ui.SidebarItem(c, fmt.Sprintf("live:%d", i), nil, lv.Name)
					}
				})
			}
		}).Grow(1)
		if side.Changed() {
			a.onSide()
		}
	})
}

func (a *App) viewTop(c *ui.Context, th *ui.Theme) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		if a.screen == screenDetail || a.screen == screenPlay {
			if ui.Button(c, "返回").Clicked() {
				a.back()
			}
		}
		ui.Column(c).Gap(1).Shrink(1).Children(func() {
			ui.Text(c, a.crumb()).FontSize(12).TextColor(th.TextMuted).SingleLine()
			ui.Text(c, a.headline()).FontSize(22).Bold().SingleLine()
		})
		ui.Column(c).Grow(1)
		if a.screen == screenBrowse || a.screen == screenHome || a.screen == screenLive || a.screen == screenDetail {
			ph := "搜索片名，回车"
			if a.screen == screenLive {
				ph = "过滤频道"
			}
			field := ui.SearchField(c, &a.query).Placeholder(ph).Width(280).Label("搜索")
			if a.screen == screenBrowse || a.screen == screenHome {
				cleared := field.Changed() && strings.TrimSpace(a.query) == "" && a.committed != ""
				if field.Submitted() || cleared {
					a.committed = strings.TrimSpace(a.query)
					a.pageNum = 1
					if a.committed == "" {
						a.screen = screenHome
						a.sideSel = "home"
						a.lastSide = "home"
					} else {
						a.screen = screenBrowse
					}
					a.loadList()
				}
			}
		}
		if ui.Button(c, "配置").Clicked() {
			a.urlDraft = a.cfgRef
			a.adding = true
		}
	})
	if a.loading {
		a.loadingRow(c, th, "正在读取新的配置…")
	}
	if a.loadErr != "" {
		a.note(c, th, a.loadErr, true)
	}
}

func (a *App) crumb() string {
	switch a.screen {
	case screenHome:
		return "首页"
	case screenHistory:
		return "资料库"
	case screenFav:
		return "资料库"
	case screenDetail:
		return "详情"
	case screenPlay:
		return "正在播放"
	case screenLive:
		return "直播"
	case screenUnsupported:
		return "源"
	case screenIndex:
		return "仓库"
	default:
		if strings.TrimSpace(a.query) != "" {
			return "搜索"
		}
		return "首页"
	}
}

func (a *App) headline() string {
	switch a.screen {
	case screenHome:
		return "继续看"
	case screenHistory:
		return "观看历史"
	case screenFav:
		return "我的收藏"
	}
	if a.screen == screenLive && a.catalog != nil && a.liveIndex >= 0 && a.liveIndex < len(a.catalog.Lives) {
		return a.catalog.Lives[a.liveIndex].Name
	}
	if m, ok := a.currentMeta(); ok && a.screen != screenIndex {
		return m.Name
	}
	return "QuickBo"
}

func (a *App) viewBrowse(c *ui.Context, th *ui.Theme) {
	searching := strings.TrimSpace(a.committed) != ""
	a.maybeLoadMore()
	cols, cellW, cellH, tracks := a.posterGrid(c)
	ui.Scroll(c).Grow(1).TrackScroll(&a.browseScroll).Children(func() {
		ui.Column(c).Gap(18).Children(func() {
			if a.screen == screenHome {
				a.viewRails(c, th)
			}
			if m, ok := a.currentMeta(); ok && m.Level == config.LevelPartial && m.Reason != "" {
				a.note(c, th, m.Reason, false)
			}
			if searching {
				label := fmt.Sprintf("搜索「%s」", strings.TrimSpace(a.committed))
				if a.page.Total > 0 {
					label += fmt.Sprintf("  ·  %d 条", a.page.Total)
				}
				ui.Text(c, label).FontSize(28).Bold()
			} else if a.listAnchored && len(a.page.Items) > 0 && !a.listLoad {
				a.viewHero(c, th, a.page.Items[0])
			}
			if len(a.cats) > 0 && !searching {
				ui.ScrollBoth(c).MaxHeight(48).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						for _, cat := range a.cats {
							cat := cat
							on := cat.ID == a.catID
							a.choice(c, th, "cat:"+cat.ID, cat.Name, on, func() { a.pickCat(cat.ID) })
						}
					})
				})
			}
			if a.listLoad && len(a.page.Items) == 0 {
				a.skeletonRow(c, th, cols, cellW, cellH)
				return
			}
			if a.listErr != "" {
				a.note(c, th, a.listErr, true)
			}
			n := len(a.page.Items)
			if n == 0 && a.listErr == "" {
				a.emptyState(c, th, "没有结果", "换一个分类，或换个关键词再搜。")
				return
			}
			ui.Grid(c).ColumnTracks(tracks...).Gap(14).Children(func() {
				for i := range a.page.Items {
					a.posterCard(c, th, a.page.Items[i], cellW, cellH)
				}
			})
			if a.appending {
				a.loadingRow(c, th, "正在加载更多…")
			}
			a.viewPager(c, th)
		})
	})
}

// posterGrid sizes a 2:3 cover wall from the window. Fixed tracks keep a
// short page from stretching two titles into banners.
func (a *App) posterGrid(c *ui.Context) (cols int, cellW, cellH float32, tracks []ui.Track) {
	winW, _ := c.Size()
	inner := winW - 236 - 72
	if a.screen == screenFav || a.screen == screenHistory {
		inner = winW - 236 - 72
	}
	if inner < 320 {
		inner = winW - 48
	}
	if inner < 280 {
		inner = 280
	}
	const gap float32 = 14
	cols = int((inner + gap) / (156 + gap))
	if cols < 2 {
		cols = 2
	}
	if cols > 8 {
		cols = 8
	}
	cellW = (inner - gap*float32(cols-1)) / float32(cols)
	if cellW < 120 {
		cellW = 120
	}
	cellH = cellW * 1.5
	tracks = make([]ui.Track, cols)
	for i := range tracks {
		tracks[i] = ui.Fixed(cellW)
	}
	return cols, cellW, cellH, tracks
}

func (a *App) viewHero(c *ui.Context, th *ui.Theme, item model.Item) {
	open := func() { a.openDetail(item) }
	_, winH := c.Size()
	h := winH * 0.38
	if h < 240 {
		h = 240
	}
	if h > 380 {
		h = 380
	}
	posterH := h - 48
	hero := ui.Box(c).FillWidth().Height(h).Radius(22).Clip().OnClick(open).Cursor(ui.CursorPointer)
	hero.Children(func() {
		if bmp := a.poster(item.Pic); bmp != nil {
			ui.Image(c, bmp).FillWidth().Height(h + 40).Fit(ui.Cover).Blur(28)
		} else {
			ui.Box(c).Fill().Gradient(th.SurfaceHover, stageInk(a.light), 120)
		}
		ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Background(ui.RGBA(0, 0, 0, 0.28))
		ui.Row(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Padding(24).Gap(22).AlignItems(ui.End).
			Gradient(ui.Transparent, ui.RGBA(0, 0, 0, 0.72), 180).
			Children(func() {
				a.portrait(c, th, item.Pic, item.Title, posterH)
				ui.Column(c).Grow(1).Gap(8).Padding(0, 0, 8, 0).Children(func() {
					ui.Text(c, "精选").FontSize(12).Bold().TextColor(ui.Hex("#e4c295"))
					ui.Text(c, item.Title).FontSize(34).Bold().TextColor(ui.Hex("#fafafa"))
					if item.Remarks != "" {
						ui.Text(c, item.Remarks).FontSize(14).TextColor(ui.RGBA(255, 255, 255, 0.82))
					}
					ui.Row(c).Padding(6, 0, 0, 0).Children(func() {
						if ui.PrimaryButton(c.Key("hero:"+item.ID), "查看详情").Clicked() {
							open()
						}
					})
				})
			})
	})
}

func (a *App) posterCard(c *ui.Context, th *ui.Theme, item model.Item, cellW, cellH float32) {
	open := func() { a.openDetail(item) }
	card := ui.Column(c.Key("card:" + item.ID)).Width(cellW).Gap(8).OnClick(open).Cursor(ui.CursorPointer)
	lift := float32(0)
	shadow := ui.RGBA(0, 0, 0, 0)
	if card.Hovered() {
		lift = -6
		shadow = ui.RGBA(228, 194, 149, 0.28)
	}
	card.Translate(0, lift).
		Shadow(0, 18, 36, 0, shadow).
		Transition(ui.ElementTransition{Duration: 160 * time.Millisecond}).
		Children(func() {
			frame := ui.Column(c).Width(cellW).Height(cellH).Radius(12).Clip().Background(th.Surface)
			frame.Children(func() {
				a.cover(c, th, item.Pic, item.Title, cellW, cellH, open)
				if item.Remarks != "" {
					ui.Column(c).Absolute().Left(8).Top(8).Padding(3, 7).Radius(6).Background(ui.RGBA(0, 0, 0, 0.66)).Children(func() {
						ui.Text(c, item.Remarks).FontSize(11).TextColor(ui.Hex("#fafafa")).SingleLine()
					})
				}
			})
			ui.Text(c, item.Title).FontSize(14).Bold().SingleLine().OnClick(open)
		})
}

func (a *App) cover(c *ui.Context, th *ui.Theme, pic, title string, w, h float32, click func()) {
	pic = model.ResolveURL("", pic)
	if bmp := a.poster(pic); bmp != nil {
		img := ui.Image(c, bmp).Width(w).Height(h).Fit(ui.Cover)
		if click != nil {
			img.OnClick(click)
		}
		return
	}
	if pic != "" && !a.failed[pic] {
		ui.Box(c).Width(w).Height(h).Background(th.SurfaceHover)
		return
	}
	ui.Column(c).Width(w).Height(h).Center().Gradient(th.Surface, th.SurfaceHover, 160).Children(func() {
		ui.Text(c, mark(title)).FontSize(28).Bold().TextColor(th.Accent)
	})
}

func (a *App) skeletonRow(c *ui.Context, th *ui.Theme, cols int, cellW, cellH float32) {
	if cols < 2 {
		cols = 5
	}
	tracks := make([]ui.Track, cols)
	for i := range tracks {
		tracks[i] = ui.Fixed(cellW)
	}
	ui.Column(c).Gap(14).Children(func() {
		ui.Box(c).FillWidth().Height(220).Radius(22).Background(th.Surface)
		ui.Grid(c).ColumnTracks(tracks...).Gap(14).Children(func() {
			for i := 0; i < cols; i++ {
				ui.Column(c).Width(cellW).Gap(8).Children(func() {
					ui.Box(c).Width(cellW).Height(cellH).Radius(12).Background(th.Surface)
					ui.Box(c).Width(cellW * 0.6).Height(12).Radius(6).Background(th.SurfaceHover)
				})
			}
		})
		a.loadingRow(c, th, "正在加载…")
	})
}

func (a *App) viewPager(c *ui.Context, th *ui.Theme) {
	if a.pageNum <= 1 && !a.page.HasNext() {
		return
	}
	label := fmt.Sprintf("第 %d 页", a.pageNum)
	if a.page.PageCount > 0 {
		label = fmt.Sprintf("第 %d / %d 页", a.pageNum, a.page.PageCount)
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, "上一页").Disabled(a.pageNum <= 1).Clicked() && a.pageNum > 1 {
			a.pageNum--
			a.loadList()
		}
		ui.Text(c, label).TextColor(th.TextMuted)
		if ui.Button(c, "下一页").Disabled(!a.page.HasNext()).Clicked() && a.page.HasNext() {
			a.pageNum++
			a.loadList()
		}
	})
}

func (a *App) viewDetail(c *ui.Context, th *ui.Theme) {
	if a.detailLoading && a.detail == nil {
		_, cellW, cellH, _ := a.posterGrid(c)
		a.skeletonRow(c, th, 5, cellW, cellH)
	}
	if a.detailErr != "" {
		a.note(c, th, a.detailErr, true)
	}
	d := a.detail
	title := a.detailTitle
	if d != nil && d.Title != "" {
		title = d.Title
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(22).Children(func() {
			ui.Row(c).Gap(22).AlignItems(ui.Start).Children(func() {
				pic := ""
				if d != nil {
					pic = d.Pic
				}
				_, winH := c.Size()
				ph := winH * 0.46
				if ph < 220 {
					ph = 220
				}
				if ph > 460 {
					ph = 460
				}
				a.portrait(c, th, pic, title, ph)
				ui.Column(c).Grow(1).Gap(10).Padding(8, 0, 0, 0).Children(func() {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, title).FontSize(34).Bold().Shrink(1)
						if d != nil && a.linePick >= 0 && a.linePick < len(d.Lines) && len(d.Lines[a.linePick].Episodes) > 0 {
							ep := d.Lines[a.linePick].Episodes[0]
							ln := d.Lines[a.linePick].Name
							a.iconBtn(c, "detail-play", iconPlay, "播放", 48, 26, th.Text, func() { a.playEpisode(ln, ep) })
						}
						if d != nil {
							on := a.lib != nil && a.lib.hasFav(mediaKey(a.siteKey, d.ID))
							heart := iconHeart
							tip := "收藏"
							if on {
								tip = "取消收藏"
							}
							a.iconBtn(c, "fav-detail", heart, tip, 48, 26, th.Text, func() {
								if a.lib == nil {
									return
								}
								a.lib.toggleFav(favItem{
									Key: mediaKey(a.siteKey, d.ID), SiteKey: a.siteKey, VodID: d.ID,
									Title: d.Title, Pic: d.Pic, Remarks: d.Remarks, Year: d.Year,
								})
							})
						}
					})
					if d != nil {
						meta := joinMeta(d.TypeName, d.Year, d.Area, d.Remarks)
						if meta != "" {
							ui.Text(c, meta).TextColor(th.Accent).Wrap()
						}
						who := joinMeta(prefix("导演 ", d.Director), prefix("主演 ", d.Actors))
						if who != "" {
							ui.Text(c, who).TextColor(th.TextMuted).Wrap()
						}
						if d.Content != "" {
							ui.Text(c, d.Content).TextColor(th.TextMuted).Wrap()
						}
					}
				})
			})
			if d == nil {
				return
			}
			if len(d.Lines) == 0 {
				a.emptyState(c, th, "没有剧集", "这个详情页没有返回可播放的地址。")
				return
			}
			if a.linePick >= len(d.Lines) {
				a.linePick = 0
			}
			if len(d.Lines) > 1 {
				ui.ScrollBoth(c).MaxHeight(44).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						for i, line := range d.Lines {
							i := i
							a.choice(c, th, "line:"+line.Name, line.Name, i == a.linePick, func() { a.linePick = i })
						}
					})
				})
			} else {
				ui.Text(c, d.Lines[0].Name).FontSize(13).Bold().TextColor(th.TextMuted)
			}
			line := d.Lines[a.linePick]
			ui.Text(c, fmt.Sprintf("%d 集", len(line.Episodes))).FontSize(12).TextColor(th.TextMuted)
			ui.Row(c).Gap(8).Wrap().Children(func() {
				for _, ep := range line.Episodes {
					ep := ep
					label := ep.Name
					if label == "" {
						label = "播放"
					}
					on := a.playLine == line.Name && label == currentEpisodeName(a)
					a.choice(c, th, line.Name+"\n"+ep.URL+"\n"+label, label, on, func() {
						a.playEpisode(line.Name, ep)
					})
				}
			})
		})
	})
}

func (a *App) portrait(c *ui.Context, th *ui.Theme, pic, title string, h float32) {
	w := h * 2 / 3
	frame := ui.Column(c).Width(w).Height(h).Radius(16).Clip().Background(th.Surface).
		Shadow(0, 18, 40, 0, ui.RGBA(0, 0, 0, 0.35))
	frame.Children(func() {
		a.cover(c, th, pic, title, w, h, nil)
	})
}

func (a *App) viewLive(c *ui.Context, th *ui.Theme) {
	if a.liveLoading && len(a.channels) == 0 {
		_, cellW, cellH, _ := a.posterGrid(c)
		a.skeletonRow(c, th, 4, cellW, cellH)
		return
	}
	if a.liveErr != "" {
		a.note(c, th, a.liveErr, true)
		return
	}
	chs := a.visibleChannels()
	if len(chs) == 0 {
		if strings.TrimSpace(a.query) != "" && len(a.channels) > 0 {
			a.emptyState(c, th, "没有频道符合「"+strings.TrimSpace(a.query)+"」", "清空搜索框可以看到全部频道。")
			return
		}
		a.emptyState(c, th, "没有频道", "这份播放列表是空的。")
		return
	}
	groups := liveGroups(chs)
	shown := chs
	if a.liveGroup != "" {
		shown = nil
		for _, ch := range chs {
			if liveGroupName(ch) == a.liveGroup {
				shown = append(shown, ch)
			}
		}
	}
	ui.Row(c).Grow(1).Gap(12).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(160).Gap(6).Children(func() {
			ui.Text(c, "分组").FontSize(12).Bold().TextColor(th.TextMuted)
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					a.choice(c, th, "grp-all", "全部", a.liveGroup == "", func() { a.liveGroup = "" })
					for _, g := range groups {
						g := g
						a.choice(c, th, "grp-"+g, g, a.liveGroup == g, func() { a.liveGroup = g })
					}
				})
			})
		})
		ui.Column(c).Width(280).Children(func() {
			ui.Text(c, fmt.Sprintf("%d 个频道", len(shown))).FontSize(12).TextColor(th.TextMuted)
			ui.List(c, &a.chState, len(shown), func(i int) {
				ch := shown[i]
				row := ui.Row(c).FillWidth().Padding(8, 10).Radius(12).Gap(8).AlignItems(ui.Center)
				bg := ui.Transparent
				if row.Hovered() || ch.Name == a.playTitle {
					bg = th.SurfaceHover
				}
				row.Background(bg).Children(func() {
					ui.Column(c).Width(32).Height(32).Radius(8).Background(th.Surface).Center().Children(func() {
						ui.Text(c, mark(ch.Name)).FontSize(13).Bold().TextColor(th.Accent)
					})
					ui.Text(c, ch.Name).Grow(1).SingleLine()
					a.iconBtn(c, "ch:"+ch.Group+"/"+ch.Name+"/"+ch.URL, iconPlay, "播放", 40, 22, th.Text, func() { a.playChannel(ch) })
				})
			}).Grow(1)
		})
		ui.Column(c).Grow(1).Children(func() {
			if a.player != nil && a.fromLive || a.playLoading && a.fromLive {
				a.paintStage(c, th, true)
			} else {
				a.emptyState(c, th, "选一个频道", "分组在左，频道在中，画面在这里播放。")
			}
		})
	})
}

func (a *App) viewUnsupported(c *ui.Context, th *ui.Theme) {
	m, ok := a.currentMeta()
	if !ok {
		a.emptyState(c, th, "没有选中的站点", "")
		return
	}
	ui.Column(c).Grow(1).Center().Children(func() {
		ui.Column(c).Width(480).Padding(32).Gap(12).Radius(24).Background(th.Surface).
			Shadow(0, 24, 60, 0, ui.RGBA(0, 0, 0, 0.35)).
			Children(func() {
				ui.Column(c).Width(56).Height(56).Radius(16).Background(th.SurfaceHover).Center().Children(func() {
					ui.Text(c, "JAR").FontSize(13).Bold().TextColor(th.Accent)
				})
				ui.Text(c, m.Name).FontSize(28).Bold()
				ui.Text(c, config.KindLabel(m.Kind)).FontSize(12).TextColor(th.TextMuted)
				reason := m.Reason
				if reason == "" {
					reason = "这个站点本版不支持。"
				}
				ui.Text(c, reason).Wrap()
				if m.Kind == config.KindJar {
					ui.Text(c, "QuickBo 不执行 jar，也不会去加载 DEX。换一个 CMS、JS 或 XBPQ 源即可。").TextColor(th.TextMuted).Wrap()
				}
				if m.API != "" {
					ui.Text(c, m.API).FontSize(12).TextColor(th.TextMuted).Selectable().Wrap()
				}
				if m.Kind == config.KindRemote && (strings.HasPrefix(m.API, "http://") || strings.HasPrefix(m.API, "https://")) {
					if ui.Button(c, "当作配置打开").Clicked() {
						a.Load(m.API)
					}
				}
			})
	})
}

func (a *App) viewIndex(c *ui.Context, th *ui.Theme) {
	ui.Text(c, "这是一份仓库索引，还不是点播配置。").FontSize(22).Bold()
	ui.Text(c, "选一个地址继续加载。").TextColor(th.TextMuted)
	if a.catalog == nil {
		return
	}
	ui.Column(c).Gap(10).Children(func() {
		for _, u := range a.catalog.Index {
			name := u.Name
			if name == "" {
				name = u.URL
			}
			row := ui.Column(c.Key(u.URL)).Padding(16).Gap(4).Radius(14).Background(th.Surface).OnClick(func() { a.Load(u.URL) }).Cursor(ui.CursorPointer)
			if row.Hovered() {
				row.Background(th.SurfaceHover)
			}
			row.Children(func() {
				ui.Text(c, name).Bold()
				ui.Text(c, u.URL).FontSize(12).TextColor(th.TextMuted).Selectable()
			})
		}
	})
}

func (a *App) choice(c *ui.Context, th *ui.Theme, key, label string, on bool, click func()) {
	p := ui.Column(c.Key(key)).Padding(8, 14).Radius(999).OnClick(click).Cursor(ui.CursorPointer)
	bg := th.Surface
	fg := th.Text
	if on {
		bg = th.Accent
		fg = th.AccentText
	} else if p.Hovered() {
		bg = th.SurfaceHover
	}
	p.Background(bg).Transition(ui.ElementTransition{Duration: 120 * time.Millisecond}).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(fg).SingleLine()
	})
}

func (a *App) note(c *ui.Context, th *ui.Theme, text string, bad bool) {
	bg := th.Surface
	fg := th.TextMuted
	if bad {
		bg = th.Danger.Alpha(0.14)
		fg = th.Danger
	}
	ui.Column(c).FillWidth().Padding(12, 14).Radius(12).Background(bg).Children(func() {
		ui.Text(c, text).TextColor(fg).Wrap()
	})
}

func (a *App) emptyState(c *ui.Context, th *ui.Theme, title, body string) {
	ui.Column(c).FillWidth().Padding(36).Gap(8).Radius(20).Background(th.Surface).Center().Children(func() {
		ui.Text(c, title).FontSize(18).Bold()
		if body != "" {
			ui.Text(c, body).TextColor(th.TextMuted).Wrap()
		}
	})
}

func (a *App) loadingRow(c *ui.Context, th *ui.Theme, text string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Spinner(c)
		ui.Text(c, text).TextColor(th.TextMuted)
	})
}

func joinMeta(parts ...string) string {
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "   ·   ")
}

func (a *App) viewRails(c *ui.Context, th *ui.Theme) {
	ui.Text(c, "继续观看").FontSize(18).Bold()
	watches := a.lib.watches()
	if len(watches) == 0 {
		ui.Text(c, "还没有观看记录。打开一部片子，进度会留在这里。").TextColor(th.TextMuted)
	} else {
		ui.ScrollBoth(c).MaxHeight(210).Children(func() {
			ui.Row(c).Gap(12).Children(func() {
				for _, w := range watches {
					w := w
					card := ui.Column(c.Key("w-" + w.Key)).Width(148).Gap(6).OnClick(func() {
						a.resumeAt = w.Position
						a.siteKey = w.SiteKey
						a.playLine = w.Line
						a.startPlay(w.Title+"  ·  "+w.Episode, w.URL, nil, false)
					}).Cursor(ui.CursorPointer)
					card.Children(func() {
						ui.Column(c).Width(148).Height(96).Radius(10).Clip().Background(th.Surface).Children(func() {
							a.cover(c, th, w.Pic, w.Title, 148, 96, nil)
						})
						ui.Text(c, w.Title).FontSize(13).Bold().SingleLine()
						frac := float32(0)
						if w.Duration > 0 {
							frac = float32(w.Position / w.Duration)
						}
						ui.Box(c).FillWidth().Height(3).Radius(2).Background(th.SurfaceHover).Children(func() {
							ui.Box(c).Height(3).WidthPercent(frac * 100).Background(th.Accent)
						})
					})
				}
			})
		})
	}
	favs := a.lib.favs()
	if len(favs) > 0 {
		ui.Text(c, "收藏").FontSize(18).Bold()
		ui.Row(c).Gap(12).Children(func() {
			for _, f := range favs {
				f := f
				ui.Column(c.Key("f-" + f.Key)).Width(120).Gap(4).Children(func() {
					ui.Column(c).Width(120).Height(168).Radius(10).Clip().Background(th.Surface).Children(func() {
						a.cover(c, th, f.Pic, f.Title, 120, 168, nil)
					})
					ui.Text(c, f.Title).FontSize(12).SingleLine()
				})
			}
		})
	}
}

func (a *App) viewHistory(c *ui.Context, th *ui.Theme) {
	items := a.lib.watches()
	if len(items) == 0 {
		a.emptyState(c, th, "还没有历史", "播放过的片子会出现在这里，并记住进度。")
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			for _, w := range items {
				w := w
				row := ui.Row(c.Key("hist-" + w.Key)).Padding(10).Gap(12).Radius(14).Background(th.Surface).AlignItems(ui.Center).OnClick(func() {
					a.resumeAt = w.Position
					a.siteKey = w.SiteKey
					a.playLine = w.Line
					a.startPlay(w.Title, w.URL, nil, false)
				}).Cursor(ui.CursorPointer)
				row.Children(func() {
					ui.Column(c).Width(72).Height(96).Radius(8).Clip().Background(th.SurfaceHover).Children(func() {
						a.cover(c, th, w.Pic, w.Title, 72, 96, nil)
					})
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, w.Title).Bold().SingleLine()
						sub := w.Episode
						if w.Line != "" {
							sub = w.Line + "  ·  " + sub
						}
						ui.Text(c, sub).FontSize(12).TextColor(th.TextMuted).SingleLine()
						frac := float32(0)
						if w.Duration > 0 {
							frac = float32(w.Position / w.Duration)
						}
						ui.Box(c).FillWidth().Height(4).Radius(2).Background(th.SurfaceHover).Children(func() {
							ui.Box(c).Height(4).WidthPercent(frac * 100).Background(th.Accent)
						})
						ui.Text(c, clock(w.Position)+" / "+clock(w.Duration)).FontSize(11).TextColor(th.TextMuted)
					})
					a.iconBtn(c, "resume-"+w.Key, iconPlay, "继续播放", 42, 24, th.Text, func() {
						a.resumeAt = w.Position
						a.startPlay(w.Title, w.URL, nil, false)
					})
				})
			}
		})
	})
}

func (a *App) viewFavs(c *ui.Context, th *ui.Theme) {
	items := a.lib.favs()
	if len(items) == 0 {
		a.emptyState(c, th, "还没有收藏", "在详情页点心形图标，片子会留在这里。")
		return
	}
	_, cellW, cellH, tracks := a.posterGrid(c)
	ui.Grid(c).ColumnTracks(tracks...).Gap(14).Children(func() {
		for _, f := range items {
			a.posterCard(c, th, model.Item{ID: f.VodID, Title: f.Title, Pic: f.Pic, Remarks: f.Remarks, Year: f.Year}, cellW, cellH)
		}
	})
}

func currentEpisodeName(a *App) string {
	if a.detail == nil {
		return ""
	}
	for _, ln := range a.detail.Lines {
		if ln.Name == a.playLine && a.playIndex >= 0 && a.playIndex < len(ln.Episodes) {
			n := ln.Episodes[a.playIndex].Name
			if n == "" {
				return "播放"
			}
			return n
		}
	}
	return ""
}

func prefix(p, v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	return p + v
}

func liveGroupName(ch live.Channel) string {
	if strings.TrimSpace(ch.Group) == "" {
		return "未分组"
	}
	return ch.Group
}

func liveGroups(chs []live.Channel) []string {
	var out []string
	seen := map[string]bool{}
	for _, ch := range chs {
		g := liveGroupName(ch)
		if seen[g] {
			continue
		}
		seen[g] = true
		out = append(out, g)
	}
	return out
}

func mark(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "·"
	}
	r := []rune(title)
	if len(r) > 1 {
		return string(r[:1])
	}
	return string(r)
}
