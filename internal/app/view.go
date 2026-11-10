package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/mybox/internal/config"
	"github.com/harrylyx/mybox/internal/model"
)

// View draws one frame. The chrome is a dark cinema layout: a narrow
// source sidebar, a hero over the poster wall, and pages that share one
// type scale and one accent.
func (a *App) View(c *ui.Context) {
	th := a.useTheme(c)
	if a.catalog == nil {
		a.viewWelcome(c, th)
	} else {
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			a.viewSidebar(c, th)
			ui.Column(c).Grow(1).Padding(22, 28, 18, 28).Gap(16).Children(func() {
				a.viewTop(c, th)
				switch a.screen {
				case screenIndex:
					a.viewIndex(c, th)
				case screenUnsupported:
					a.viewUnsupported(c, th)
				case screenDetail:
					a.viewDetail(c, th)
				case screenPlay:
					a.viewPlay(c, th)
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
				ui.Text(c, "MYBOX").FontSize(13).Bold().TextColor(th.Accent)
				ui.Text(c, "mybox").FontSize(52).Bold()
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
			ui.Text(c, "TVBox / FongMi 订阅的 URL，或本机 JSON。mybox 不内置任何片源。").TextColor(th.TextMuted).Wrap()
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
			ui.Text(c, "mybox").FontSize(20).Bold()
			ui.Text(c, "自备片源").FontSize(12).TextColor(th.TextMuted)
		})
		side := ui.Sidebar(c, &a.sideSel, func() {
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
		if a.screen == screenBrowse || a.screen == screenLive || a.screen == screenDetail {
			ph := "搜索片名，回车"
			if a.screen == screenLive {
				ph = "过滤频道"
			}
			field := ui.SearchField(c, &a.query).Placeholder(ph).Width(280).Label("搜索")
			if a.screen == screenBrowse && (field.Submitted() || (field.Changed() && strings.TrimSpace(a.query) == "")) {
				a.pageNum = 1
				a.screen = screenBrowse
				a.loadList()
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
	if a.screen == screenLive && a.catalog != nil && a.liveIndex >= 0 && a.liveIndex < len(a.catalog.Lives) {
		return a.catalog.Lives[a.liveIndex].Name
	}
	if m, ok := a.currentMeta(); ok && a.screen != screenIndex {
		return m.Name
	}
	return "mybox"
}

func (a *App) viewBrowse(c *ui.Context, th *ui.Theme) {
	searching := strings.TrimSpace(a.query) != ""
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(18).Children(func() {
			if m, ok := a.currentMeta(); ok && m.Level == config.LevelPartial && m.Reason != "" {
				a.note(c, th, m.Reason, false)
			}
			if searching {
				ui.Text(c, "搜索「"+strings.TrimSpace(a.query)+"」").FontSize(28).Bold()
			} else if len(a.page.Items) > 0 && !a.listLoad {
				a.viewHero(c, th, a.page.Items[0])
			}
			if len(a.cats) > 0 && !searching {
				ui.ScrollBoth(c).MaxHeight(44).Children(func() {
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
				a.skeletonRow(c, th)
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
			ui.Grid(c).ColumnTracks(posterTracks()...).Gap(18).Children(func() {
				for i := range a.page.Items {
					a.posterCard(c, th, a.page.Items[i])
				}
			})
			a.viewPager(c, th)
		})
	})
}

// posterTracks keeps covers at a poster width. Flexible tracks were
// stretching two titles into wide banners.
func posterTracks() []ui.Track {
	out := make([]ui.Track, 5)
	for i := range out {
		out[i] = ui.Fixed(176)
	}
	return out
}

func (a *App) viewHero(c *ui.Context, th *ui.Theme, item model.Item) {
	open := func() { a.openDetail(item) }
	hero := ui.Box(c).FillWidth().Height(280).Radius(22).Clip().OnClick(open).Cursor(ui.CursorPointer)
	hero.Children(func() {
		if bmp := a.poster(item.Pic); bmp != nil {
			ui.Image(c, bmp).FillWidth().Height(280).Fit(ui.Cover)
		} else {
			ui.Box(c).Fill().Gradient(th.SurfaceHover, stageInk(a.light), 120)
		}
		ui.Column(c).Absolute().Left(0).Right(0).Bottom(0).Top(0).Justify(ui.End).Padding(28).Gap(8).
			Gradient(ui.Transparent, ui.RGBA(0, 0, 0, 0.78), 180).
			Children(func() {
				ui.Text(c, "继续看").FontSize(12).Bold().TextColor(ui.Hex("#e4c295"))
				ui.Text(c, item.Title).FontSize(36).Bold().TextColor(ui.Hex("#fafafa"))
				if item.Remarks != "" {
					ui.Text(c, item.Remarks).FontSize(14).TextColor(ui.RGBA(255, 255, 255, 0.78))
				}
				ui.Row(c).Padding(6, 0, 0, 0).Children(func() {
					if ui.PrimaryButton(c.Key("hero:"+item.ID), "查看详情").Clicked() {
						open()
					}
				})
			})
	})
}

func (a *App) posterCard(c *ui.Context, th *ui.Theme, item model.Item) {
	open := func() { a.openDetail(item) }
	card := ui.Column(c.Key("card:" + item.ID)).Gap(8).OnClick(open).Cursor(ui.CursorPointer)
	bg := ui.Transparent
	shadow := ui.RGBA(0, 0, 0, 0)
	if card.Hovered() {
		bg = th.SurfaceHover
		shadow = ui.RGBA(0, 0, 0, 0.35)
	}
	card.Padding(8).Radius(16).Background(bg).
		Shadow(0, 16, 32, 0, shadow).
		Transition(ui.ElementTransition{Duration: 160 * time.Millisecond}).
		Children(func() {
			frame := ui.Column(c).FillWidth().Height(248).Radius(12).Clip().Background(th.Surface)
			frame.Children(func() {
				if bmp := a.poster(item.Pic); bmp != nil {
					ui.Image(c, bmp).FillWidth().Height(248).Fit(ui.Cover).OnClick(open)
					return
				}
				ui.Column(c).Fill().Center().Gradient(th.Surface, th.SurfaceHover, 160).Children(func() {
					ui.Text(c, mark(item.Title)).FontSize(28).Bold().TextColor(th.Accent)
				})
			})
			ui.Text(c, item.Title).FontSize(14).Bold().SingleLine().OnClick(open)
			sub := item.Remarks
			if sub == "" {
				sub = item.Year
			}
			if sub == "" {
				sub = " "
			}
			ui.Text(c, sub).FontSize(12).TextColor(th.TextMuted).SingleLine()
		})
}

func (a *App) skeletonRow(c *ui.Context, th *ui.Theme) {
	ui.Column(c).Gap(14).Children(func() {
		ui.Box(c).FillWidth().Height(180).Radius(22).Background(th.Surface)
		ui.Grid(c).ColumnTracks(posterTracks()...).Gap(18).Children(func() {
			for i := 0; i < 5; i++ {
				ui.Column(c).Width(160).Gap(8).Children(func() {
					ui.Box(c).Width(160).Height(240).Radius(12).Background(th.Surface)
					ui.Box(c).Width(96).Height(12).Radius(6).Background(th.SurfaceHover)
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
		a.skeletonRow(c, th)
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
				a.portrait(c, th, pic, title, 200)
				ui.Column(c).Grow(1).Gap(10).Padding(8, 0, 0, 0).Children(func() {
					ui.Text(c, title).FontSize(34).Bold().Wrap()
					if d != nil {
						meta := joinMeta(d.TypeName, d.Year, d.Area, d.Remarks)
						if meta != "" {
							ui.Text(c, meta).TextColor(th.Accent).Wrap()
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
			ui.Row(c).Gap(8).Children(func() {
				for _, ep := range line.Episodes {
					ep := ep
					label := ep.Name
					if label == "" {
						label = "播放"
					}
					a.choice(c, th, line.Name+"\n"+ep.URL+"\n"+label, label, false, func() {
						a.playEpisode(line.Name, ep)
					})
				}
			})
		})
	})
}

func (a *App) portrait(c *ui.Context, th *ui.Theme, pic, title string, h float32) {
	w := h * 0.7
	frame := ui.Column(c).Width(w).Height(h).Radius(16).Clip().Background(th.Surface).
		Shadow(0, 18, 40, 0, ui.RGBA(0, 0, 0, 0.35))
	frame.Children(func() {
		if bmp := a.poster(pic); bmp != nil {
			ui.Image(c, bmp).Width(w).Height(h).Fit(ui.Cover)
			return
		}
		ui.Column(c).Fill().Center().Gradient(th.SurfaceHover, th.Surface, 160).Children(func() {
			ui.Text(c, mark(title)).FontSize(36).Bold().TextColor(th.Accent)
		})
	})
}

func (a *App) viewPlay(c *ui.Context, th *ui.Theme) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.playTitle).FontSize(20).Bold().Shrink(1)
		ui.Column(c).Grow(1)
		if ui.Button(c, "用 mpv 播放").Clicked() {
			a.openExternal()
		}
	})
	if ui.Checkbox(c, &a.preferExternal, "之后优先使用 mpv").Changed() {
		a.saveSettings()
	}
	if a.playLoading {
		a.loadingRow(c, th, "正在解析播放地址…")
	}
	if a.playErr != "" {
		a.note(c, th, a.playErr, true)
	}
	if a.playNote != "" {
		a.note(c, th, a.playNote, false)
	}
	stage := ui.Column(c).FillWidth().Height(460).Radius(20).Clip().Background(stageInk(a.light)).
		Shadow(0, 20, 50, 0, ui.RGBA(0, 0, 0, 0.4))
	stage.Children(func() {
		if a.player != nil {
			ui.Video(c, a.player).Grow(1).Fit(ui.Contain)
			return
		}
		ui.Column(c).Fill().Center().Gap(8).Children(func() {
			if a.playLoading {
				ui.Spinner(c)
			}
			ui.Text(c, "等待画面").TextColor(ui.Hex("#a1a1aa"))
		})
	})
	if a.player != nil {
		if !a.player.Ready() || a.player.Buffering() {
			a.loadingRow(c, th, "缓冲…")
		}
		if err := a.player.Err(); err != nil {
			a.note(c, th, err.Error(), true)
		}
		ui.Row(c).Gap(12).Padding(12, 14).Radius(16).Background(th.Surface).AlignItems(ui.Center).Children(func() {
			label := "暂停"
			if !a.player.Playing() {
				label = "播放"
			}
			if ui.Button(c.Key("play-toggle"), label).Clicked() {
				if a.player.Playing() {
					a.player.Pause()
				} else {
					a.player.Play()
				}
			}
			dur := a.player.Duration()
			if dur > 0 {
				pos := a.player.Position().Seconds()
				if ui.Slider(c.Key("seek"), &pos, 0, dur.Seconds()).Grow(1).Label("进度").Changed() {
					a.player.Seek(time.Duration(pos * float64(time.Second)))
				}
			} else {
				ui.Text(c, "直播").TextColor(th.TextMuted).Grow(1)
			}
			vol := a.volume
			if ui.Slider(c.Key("volume"), &vol, 0, 1).Width(110).Label("音量").Changed() {
				a.volume = vol
				a.player.SetVolume(vol)
				a.saveSettings()
			}
		})
	}
	if a.playURL != "" {
		ui.Text(c, a.playURL).FontSize(11).TextColor(th.TextMuted).Selectable().SingleLine()
	}
}

func (a *App) viewLive(c *ui.Context, th *ui.Theme) {
	if a.liveLoading && len(a.channels) == 0 {
		a.skeletonRow(c, th)
		return
	}
	if a.liveErr != "" {
		a.note(c, th, a.liveErr, true)
		return
	}
	chs := a.visibleChannels()
	if len(chs) == 0 {
		a.emptyState(c, th, "没有频道", "播放列表是空的，或过滤条件没有命中。")
		return
	}
	ui.Text(c, fmt.Sprintf("%d 个频道", len(chs))).FontSize(13).TextColor(th.TextMuted)
	ui.List(c, &a.chState, len(chs), func(i int) {
		ch := chs[i]
		row := ui.Row(c).FillWidth().Padding(10, 12).Radius(12).Gap(12).AlignItems(ui.Center)
		bg := ui.Transparent
		if row.Hovered() {
			bg = th.SurfaceHover
		}
		row.Background(bg).Children(func() {
			ui.Column(c).Width(36).Height(36).Radius(10).Background(th.Surface).Center().Children(func() {
				ui.Text(c, mark(ch.Name)).FontSize(14).Bold().TextColor(th.Accent)
			})
			ui.Column(c).Grow(1).Gap(1).Children(func() {
				ui.Text(c, ch.Name).SingleLine()
				group := ch.Group
				if group == "" {
					group = "直播"
				}
				ui.Text(c, group).FontSize(12).TextColor(th.TextMuted).SingleLine()
			})
			if ui.Button(c.Key("ch:"+ch.Group+"/"+ch.Name+"/"+ch.URL), "播放").Clicked() {
				a.playChannel(ch)
			}
		})
	}).Grow(1)
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
					ui.Text(c, "mybox 不执行 jar，也不会去加载 DEX。换一个 CMS、JS 或 XBPQ 源即可。").TextColor(th.TextMuted).Wrap()
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
