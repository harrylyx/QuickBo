package app

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/live"
	"github.com/harrylyx/QuickBo/internal/model"
)

const (
	sideWidth float32 = 224
	pagePad   float32 = 28
)

var cardMotion = ui.Animation{Duration: 200 * time.Millisecond, Ease: ui.EaseOut}

func (a *App) animateCard(card, frame ui.Element, hot, light bool) {
	pose := ui.Pose{}
	if hot {
		pose.Y = -2
	}
	card.AnimatePose(pose, cardMotion)
	y, blur, ink := cardShadow(light, hot)
	frame.Shadow(0, y, blur, 0, ink).
		Transition(ui.ElementTransition{Duration: 200 * time.Millisecond, Ease: ui.EaseOut, Shadows: true})
}

// View draws one frame. The chrome is a cinema shell: a quiet source
// rail, a full-bleed hero, and a poster wall. Red is reserved for the
// primary action and for progress.
func (a *App) View(c *ui.Context) {
	th := a.useTheme(c)
	a.playerKeys(c)
	if a.catalog == nil {
		a.viewWelcome(c, th)
	} else if a.screen == screenPlay || a.screen == screenLive && a.fullscreen && a.fromLive && a.embedded() {
		// A live channel in fullscreen takes the whole window, as a VOD does.
		ui.Box(c).Fill().Background(ui.Hex("#000000")).Children(func() {
			a.viewPlay(c, th)
		})
	} else {
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			a.viewSidebar(c, th)
			main := ui.Box(c.Key("main")).Grow(1).FillHeight().Background(th.Background).Clip()
			main.Children(func() {
				// The detail page sits on its poster: blurred, tinted with the
				// poster's color, fading into the page (detailBackdrop).
				if a.screen == screenDetail {
					a.detailBackdrop(c, th)
				}
				// Positioned too, after the backdrop, so it paints over it:
				// mygo paints positioned elements over the flow.
				ui.Column(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Children(func() {
					a.viewTop(c, th)
					if a.screen != screenDetail {
						ui.Box(c).FillWidth().Height(1).Background(th.Border)
					}
					ui.Column(c).Grow(1).Children(func() {
						if a.screen == screenHome {
							a.viewHome(c, th)
							return
						}
						if a.screen == screenBrowse {
							a.viewBrowse(c, th)
							return
						}
						ui.Column(c).Grow(1).Padding(20, pagePad, 16, pagePad).Gap(16).Children(func() {
							switch a.screen {
							case screenHistory:
								a.viewHistory(c, th)
							case screenFav:
								a.viewFavs(c, th)
							case screenSubs, screenSources, screenBlacklist, screenRules:
								a.viewManage(c, th)
							case screenMore:
								a.viewMore(c, th)
							case screenIndex:
								a.viewIndex(c, th)
							case screenUnsupported:
								a.viewUnsupported(c, th)
							case screenDetail:
								a.viewDetail(c, th)
							case screenLive:
								a.viewLive(c, th)
							case screenEmpty:
								a.emptyState(c, th, "这份配置是空的", "没有点播站，也没有直播。换一份配置，或往里面加上站点。",
									stateAction{label: "添加订阅", primary: true, click: a.openSubs},
									stateAction{label: "打开本地文件", click: a.pickFile})
							default:
								a.viewBrowse(c, th)
							}
						})
					})
				})
			})
		})
	}
	a.viewSubs(c, th)
	a.viewSettings(c, th)
}

func (a *App) viewWelcome(c *ui.Context, th *ui.Theme) {
	welcomeTransition := ui.ElementTransition{Duration: 520 * time.Millisecond, Ease: ui.EaseOut}
	if !a.welcomeEntered {
		welcomeTransition.Enter = &ui.Motion{Y: 16, Opacity: 0, Scale: 1}
	}
	ui.Box(c).Fill().Children(func() {
		ui.Box(c).Fill().RadialGradient(ui.RadialGradient{
			Center: ui.Point{X: 0.0, Y: 1},
			Radius: 720,
			Stops: []ui.GradientStop{
				{Color: th.Accent.Alpha(0.16), At: 0},
				{Color: ui.Transparent, At: 0.72},
			},
		})
		ui.Column(c).Fill().Padding(48, 72).Children(func() {
			ui.Column(c).Grow(1)
			ui.Column(c.Key("welcome-hero")).Width(520).Gap(16).Transition(welcomeTransition).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(18).Height(2).Radius(1).Background(th.Accent)
					ui.Text(c, "TVBox / FongMi 播放器").FontSize(fontCaption).TextColor(th.TextMuted)
				})
				ui.Text(c, "QuickBo").FontSize(60).Bold()
				ui.Text(c, "添加订阅，开始播放。").FontSize(fontHeadline)
				ui.Text(c, "不内置任何片源。把你自己的配置 URL 或本地 JSON 加进来，再浏览、搜索和播放。JSON / XML CMS、一部分 JS（drpy）和 XBPQ 规则可以直接用。JAR/DEX 爬虫会列出来，但本版不执行 jar。").TextColor(th.TextMuted).Wrap()
				if a.loading {
					a.loadingRow(c, th, "正在读取配置…")
				}
				if a.loadErr != "" {
					a.noteWithActions(c, th, a.loadErr, true, stateAction{label: "重试订阅", primary: true, click: a.retrySubs})
				}
				ui.Row(c).Gap(10).Padding(8, 0, 0, 0).Children(func() {
					if ui.PrimaryButton(c, "添加订阅").Clicked() {
						a.openSubs()
					}
					if ui.Button(c, "打开本地文件").Clicked() {
						a.pickFile()
					}
				})
			})
		})
	})
	a.welcomeEntered = true
}

func (a *App) viewSubs(c *ui.Context, th *ui.Theme) {
	ui.Modal(c, &a.adding, func() {
		ui.Column(c).Width(640).Gap(14).Children(func() {
			a.subsPanel(c, th, false)
		})
	})
}

// subsPanel is the subscription list with its add field: in a dialog before
// any configuration is loaded (page false), and as the 订阅 tab of the
// management page after.
func (a *App) subsPanel(c *ui.Context, th *ui.Theme, page bool) {
	if !page {
		ui.Text(c, "订阅").FontSize(fontHeadline).Bold()
	}
	ui.Text(c, "TVBox / FongMi 订阅的 URL，或本机 JSON。多份源会合并；停用或删除后立即重新加载。").TextColor(th.TextMuted).Wrap()

	sortSel := a.subSort
	if sortSel == "" {
		sortSel = "名称"
	}
	batchLabel := "批量管理"
	if a.subBatch {
		batchLabel = "完成"
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.SearchField(c, &a.subFilter).Placeholder("搜索订阅").Width(200).Label("搜索订阅")
		ui.Column(c).Grow(1)
		ui.Select(c, &sortSel, subSortOptions).Label("排序").Width(124)
		if ui.Button(c, batchLabel).Clicked() {
			a.subBatch = !a.subBatch
			if !a.subBatch {
				a.subSel = nil
			}
		}
	})
	a.subSort = sortSel

	subs := a.filteredSubs()
	counts := a.subSourceCounts()
	stop := false
	list := ui.Scroll(c).FillWidth()
	if page {
		list.Grow(1).MinHeight(160)
	} else {
		list.Height(240)
	}
	list.Children(func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			if len(subs) == 0 {
				if len(a.subs) == 0 {
					ui.Text(c, "还没有订阅").TextColor(th.TextMuted)
				} else {
					ui.Text(c, "没有匹配「"+strings.TrimSpace(a.subFilter)+"」的订阅").TextColor(th.TextMuted)
				}
				return
			}
			for i := range subs {
				sub := subs[i]
				ref := sub.Ref
				ui.Row(c.Key("sub:"+ref)).FillWidth().Padding(10, 12).Radius(8).Background(th.Surface).AlignItems(ui.Center).Gap(8).Children(func() {
					if a.subBatch {
						checked := a.subSel[ref]
						if ui.Checkbox(c, &checked, "").Changed() {
							if a.subSel == nil {
								a.subSel = map[string]bool{}
							}
							if checked {
								a.subSel[ref] = true
							} else {
								delete(a.subSel, ref)
							}
						}
					}
					ui.Column(c).Grow(1).Gap(2).Shrink(1).Children(func() {
						ui.Text(c, sub.Name).FontSize(fontBody).SingleLine()
						ui.Text(c, ref).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
						if !sub.Enabled {
							ui.Text(c, "已停用").FontSize(fontCaption).TextColor(th.TextMuted)
						}
					})
					if n := counts[subLabel(sub)]; n > 0 {
						ui.Text(c, fmt.Sprintf("%d 个源", n)).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
					}
					if !a.subBatch {
						label, glyph := "停用", iconPause
						if !sub.Enabled {
							label, glyph = "启用", iconPlay
						}
						a.iconBtn(c, "sub-toggle:"+ref, glyph, label, 30, 16, th.TextMuted, func() {
							a.toggleSubscription(ref)
							stop = true
						})
						a.iconBtn(c, "sub-drop:"+ref, iconTrash, "删除订阅", 30, 16, th.Danger, func() {
							a.removeSubscription(ref)
							stop = true
						})
					}
				})
				if stop {
					return
				}
			}
		})
	})
	if stop {
		return
	}

	if a.subBatch {
		all := ui.CheckOff
		selCount := 0
		for _, s := range subs {
			if a.subSel[s.Ref] {
				selCount++
			}
		}
		switch {
		case selCount == len(subs) && len(subs) > 0:
			all = ui.CheckOn
		case selCount > 0:
			all = ui.CheckMixed
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.CheckboxState(c, &all, "全选").Changed() {
				if a.subSel == nil {
					a.subSel = map[string]bool{}
				}
				if all == ui.CheckOn {
					for _, s := range subs {
						a.subSel[s.Ref] = true
					}
				} else {
					a.subSel = nil
				}
			}
			ui.Column(c).Grow(1)
			selected := func() []string {
				var out []string
				for r, on := range a.subSel {
					if on {
						out = append(out, r)
					}
				}
				return out
			}
			if ui.Button(c, "启用").Disabled(selCount == 0).Clicked() && selCount > 0 {
				a.setSubscriptionsEnabled(selected(), true)
				stop = true
			}
			if ui.Button(c, "停用").Disabled(selCount == 0).Clicked() && selCount > 0 {
				a.setSubscriptionsEnabled(selected(), false)
				stop = true
			}
			if ui.Button(c, "删除").Disabled(selCount == 0).Clicked() && selCount > 0 {
				a.deleteSubscriptions(selected())
				stop = true
			}
		})
		if stop {
			return
		}
	}

	ui.Text(c, "配置 URL").FontSize(fontCaption).TextColor(th.TextMuted)
	field := ui.TextInput(c, &a.urlDraft).Placeholder("https://example.com/tvbox.json").Label("配置 URL")
	if field.Submitted() {
		ref := a.urlDraft
		a.urlDraft = ""
		a.adding = false
		a.AddSubscription(ref)
		return
	}
	if a.loadErr != "" && a.catalog != nil {
		a.noteWithActions(c, th, a.loadErr, true, stateAction{label: "重试订阅", primary: true, click: a.retrySubs})
	}
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		if !page && ui.Button(c, "取消").Clicked() {
			a.adding = false
			a.subBatch = false
			a.subSel = nil
			a.subFilter = ""
		}
		if ui.Button(c, "打开文件").Clicked() {
			a.pickFile()
		}
		if ui.PrimaryButton(c, "添加订阅").Clicked() {
			ref := a.urlDraft
			a.urlDraft = ""
			a.adding = false
			a.AddSubscription(ref)
			return
		}
	})
}

func (a *App) viewSettings(c *ui.Context, th *ui.Theme) {
	ui.Modal(c, &a.settingsOpen, func() {
		ui.Column(c).Width(480).Gap(16).Children(func() {
			ui.Text(c, "设置").FontSize(fontHeadline).Bold()
			ui.Text(c, "外观偏好只影响这台设备。订阅、片源、黑名单和首页分类在侧栏「管理」中。").TextColor(th.TextMuted).Wrap()

			ui.Text(c, "外观").FontSize(fontCaption).TextColor(th.TextMuted)
			ui.Row(c).Gap(8).Children(func() {
				for _, opt := range [][2]string{{appearanceSystem, "跟随系统"}, {appearanceLight, "浅色"}, {appearanceDark, "深色"}} {
					cur := a.appearance
					if !validAppearance(cur) {
						cur = appearanceSystem
					}
					a.choice(c, th, "外观"+opt[1], opt[1], cur == opt[0], func() {
						a.appearance = opt[0]
						a.saveSettings()
					})
				}
			})

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.PrimaryButton(c, "完成").Clicked() {
					a.settingsOpen = false
				}
			})
		})
	})
}

// openSubs shows the subscriptions: the 订阅 tab of the management page
// once a configuration is loaded, a dialog on the welcome screen before.
func (a *App) openSubs() {
	a.urlDraft = ""
	a.settingsOpen = false
	if a.catalog != nil {
		a.adding = false
		a.goManage(screenSubs)
		return
	}
	a.adding = true
}

func (a *App) openSettings() {
	a.adding = false
	a.settingsOpen = true
}

// hoverAction builds a row's hover-only button at all times and only toggles
// whether it shows. Building it on hover alone made the row taller while
// hovered, so after a delete the row sliding under the pointer grew, moved
// the rows below, and the sidebar shook.
func (a *App) hoverAction(c *ui.Context, show bool, build func()) {
	slot := ui.Row(c).Shrink(0)
	if !show {
		slot = slot.Invisible()
	}
	slot.Children(build)
}

func (a *App) viewSidebar(c *ui.Context, th *ui.Theme) {
	ink := sidebarInk(a.light)
	col := ui.Themed(c, sidebarTheme(th, ink)).Width(sideWidth).FillHeight().Background(ink).Padding(20, 12, 16, 12).Gap(18)
	col.Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(2, 8, 0, 8).Children(func() {
			ui.Column(c).Size(30, 30).Radius(9).Center().
				LinearGradient(ui.LinearGradient{From: th.Accent.MixOklab(ui.Hex("#ffffff"), 0.18), To: th.Accent.MixOklab(ui.Hex("#000000"), 0.2), Angle: 160, Oklab: true}).
				Shadow(0, 4, 12, -2, th.Accent.Alpha(0.45)).Children(func() {
				ui.Icon(c, iconPlay).Size(12, 12).TextColor(th.AccentText)
			})
			ui.Text(c, "QuickBo").FontSize(fontTitle).FontWeight(weightTitle)
		})
		side := ui.Sidebar(c, &a.sideSel, func() {
			ui.SidebarSection(c, "导航", nil, func() {
				ui.SidebarItem(c, "home", iconHome, "首页")
				ui.SidebarItem(c, "manage", iconSliders, "管理")
			})
			ui.SidebarSection(c, "资料库", nil, func() {
				ui.SidebarItem(c, "history", iconClock, "历史")
				ui.SidebarItem(c, "fav", iconHeart, "收藏")
			})
			for _, g := range a.sideGroups() {
				g := g
				ui.SidebarSection(c, sideGroupTitle(g), a.sidebarOpenState(g.name, a.groupHasSelection(g)), func() {
					for _, s := range g.sites {
						m := s.Meta()
						key := m.Key
						item := ui.SidebarItem(c, "site:"+key, iconScreen, m.Name)
						hovered := item.Hovered()
						item.ContextMenu(func(menu *ui.Menu) {
							if menu.Item("删除").Chosen() {
								a.dropSite(key)
							}
						})
						item.Children(func() {
							a.hoverAction(c, hovered, func() {
								a.iconBtn(c, "drop-site:"+key, iconClose, "移除源", 18, 11, th.TextMuted, func() {
									a.dropSite(key)
								})
							})
						})
					}
					for _, li := range g.lives {
						lv, idx := li.entry, li.idx
						item := ui.SidebarItem(c, fmt.Sprintf("live:%d", idx), iconSignal, lv.Name)
						hovered := item.Hovered()
						item.ContextMenu(func(menu *ui.Menu) {
							if menu.Item("删除").Chosen() {
								a.dropLive(lv.URL)
							}
						})
						item.Children(func() {
							a.hoverAction(c, hovered, func() {
								a.iconBtn(c, "drop-live:"+lv.URL, iconClose, "移除直播源", 18, 11, th.TextMuted, func() {
									a.dropLive(lv.URL)
								})
							})
						})
					}
				})
			}
		}).Grow(1)
		if side.Changed() {
			a.onSide()
		}
		ui.Box(c).FillWidth().Height(1).Background(th.Border)
		entry := ui.Row(c.Key("settings-entry")).FillWidth().Padding(10, 10).Radius(8).AlignItems(ui.Center).Gap(10).
			OnClick(a.openSettings).Cursor(ui.CursorPointer).Tooltip("打开设置").Label("设置")
		switch {
		case a.settingsOpen:
			entry.Background(th.Accent.Alpha(0.12))
		case entry.Hovered():
			entry.Background(th.SurfaceHover)
		}
		entry.Transition(ui.ElementTransition{Duration: 150 * time.Millisecond}).Children(func() {
			ui.Icon(c, iconGear).Size(18, 18).TextColor(th.TextMuted)
			ui.Text(c, "设置").FontSize(fontBody).TextColor(th.Text)
		})
	})
}

func (a *App) viewTop(c *ui.Context, th *ui.Theme) {
	ui.Row(c).Padding(12, pagePad).Gap(12).AlignItems(ui.Center).Children(func() {
		if a.screen == screenDetail {
			// The title is the page's own, in large: the bar only takes
			// you back, as a disc over the backdrop.
			glassButton(c, th, "detail-back", iconBack, "返回", 36, th.Text, a.back)
		} else {
			if a.screen == screenPlay {
				if ui.Button(c, "返回").Clicked() {
					a.back()
				}
			}
			ui.Column(c).Gap(1).Shrink(1).Children(func() {
				ui.Text(c, a.crumb()).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
				ui.Text(c, a.headline()).FontSize(fontTitle).FontWeight(weightTitle).SingleLine()
			})
		}
		ui.Column(c).Grow(1)
		if a.screen == screenBrowse || a.screen == screenHome || a.screen == screenLive {
			ph := "搜索片名，回车"
			if a.screen == screenHome {
				ph = "搜索全部片源，回车"
			}
			if a.screen == screenLive {
				ph = "过滤频道"
			}
			field := ui.SearchField(c, &a.query).Placeholder(ph).Width(260).Label("搜索")
			if a.screen == screenBrowse || a.screen == screenHome {
				cleared := field.Changed() && strings.TrimSpace(a.query) == "" && a.committed != ""
				if field.Submitted() || cleared {
					a.committed = strings.TrimSpace(a.query)
					a.pageNum = 1
					if a.screen == screenHome {
						if a.committed == "" {
							a.homeHits = nil
							a.homeSearching = false
						} else {
							a.searchHome()
						}
					} else {
						if a.committed == "" {
							a.screen = screenHome
							a.sideSel = "home"
							a.lastSide = "home"
						} else if a.siteKey != "" {
							a.sideSel = "site:" + a.siteKey
							a.lastSide = a.sideSel
						}
						a.loadList()
					}
				}
			}
		}
		if !isManage(a.screen) && ui.Button(c, "订阅").Clicked() {
			a.openSubs()
		}
	})
	if a.loading || a.loadErr != "" {
		ui.Column(c).Padding(0, pagePad, 10, pagePad).Gap(8).Children(func() {
			if a.loading {
				msg := "正在读取新的配置…"
				if a.loadNote != "" {
					msg = a.loadNote
				}
				a.loadingRow(c, th, msg)
			}
			if a.loadErr != "" {
				a.noteWithActions(c, th, a.loadErr, true, stateAction{label: "重试订阅", primary: true, click: a.retrySubs})
			}
		})
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
	case screenSubs, screenSources, screenBlacklist, screenRules:
		return "管理"
	case screenMore:
		return "首页"
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
		return "全部片源"
	case screenHistory:
		return "观看历史"
	case screenFav:
		return "我的收藏"
	case screenSubs:
		return "订阅"
	case screenSources:
		return "片源"
	case screenBlacklist:
		return "黑名单"
	case screenRules:
		return "首页分类"
	case screenMore:
		return a.moreRow.Name
	case screenDetail:
		if a.detailTitle != "" {
			return a.detailTitle
		}
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
		ui.Column(c).Gap(20).Children(func() {
			ui.Column(c).Padding(8, pagePad, 0, pagePad).Gap(18).Children(func() {
				if m, ok := a.currentMeta(); ok && m.Level == config.LevelPartial && m.Reason != "" {
					a.note(c, th, m.Reason, false)
				}
				if searching {
					label := fmt.Sprintf("搜索「%s」", strings.TrimSpace(a.committed))
					if a.page.Total > 0 && a.page.Total < 500 {
						label += fmt.Sprintf("  ·  %d 条", a.page.Total)
					}
					ui.Text(c, label).FontSize(fontHero).Bold()
				}
				if len(a.cats) > 0 && !searching {
					a.catTabs(c, th)
				}
				if a.listLoad && len(a.page.Items) == 0 {
					a.skeletonRow(c, th, cols, cellW, cellH)
					return
				}
				if a.listErr != "" {
					a.noteWithActions(c, th, a.listErr, true, stateAction{label: "重试", primary: true, click: a.retryList})
				}
				n := len(a.page.Items)
				if n == 0 && a.listErr == "" {
					actions := []stateAction{
						{label: "重新加载", primary: true, click: a.retryList},
						{label: "返回首页", click: a.goHome},
					}
					if searching {
						actions[0] = stateAction{label: "清空搜索", primary: true, click: func() {
							a.query = ""
							a.committed = ""
							a.pageNum = 1
							a.retryList()
						}}
					}
					a.emptyState(c, th, "没有结果", "换一个分类，或换个关键词再搜。", actions...)
					return
				}
				ui.Grid(c).ColumnTracks(tracks...).Gap(16).Children(func() {
					for i := 0; i < n; i++ {
						a.posterCard(c, th, a.siteKey, a.page.Items[i], cellW, cellH)
					}
				})
				if a.appending {
					a.loadingRow(c, th, "正在加载更多…")
				}
				a.viewPager(c, th)
			})
		})
	})
}

// posterGrid sizes a 2:3 cover wall from the window. Fixed tracks keep a
// short page from stretching two titles into banners.
func (a *App) posterGrid(c *ui.Context) (cols int, cellW, cellH float32, tracks []ui.Track) {
	winW, _ := c.Size()
	inner := winW - sideWidth - pagePad*2
	if inner < 320 {
		inner = winW - 48
	}
	if inner < 280 {
		inner = 280
	}
	const gap float32 = 16
	cols = int((inner + gap) / (164 + gap))
	if cols < 2 {
		cols = 2
	}
	if cols > 7 {
		cols = 7
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

func portraitHeight(winH float32) float32 {
	ph := winH * 0.46
	if ph < 220 {
		return 220
	}
	if ph > 460 {
		return 460
	}
	return ph
}

// choiceTipRunes is how long a choice's label may be before it gets a
// tooltip with the whole of it.
const choiceTipRunes = 5

// episodeCell is how wide an episode button is for names up to longest
// runes: narrow for 第12集, wider for a variety show's dated, titled ones.
func episodeCell(longest int) float32 {
	switch {
	case longest <= 5:
		return 104
	case longest <= 9:
		return 160
	}
	return 232
}

func longestEpisode(eps []model.Episode) int {
	n := 0
	for _, ep := range eps {
		n = max(n, utf8.RuneCountInString(ep.Name))
	}
	return n
}

func (a *App) episodeTracks(c *ui.Context, longest int) []ui.Track {
	winW, winH := c.Size()
	available := winW - sideWidth - pagePad*2 - portraitHeight(winH)*2/3 - 22
	const gap float32 = 8
	cell := episodeCell(longest)
	cols := int((available + gap) / (cell + gap))
	if cols < 2 {
		cols = 2
	}
	if cols > 8 {
		cols = 8
	}
	tracks := make([]ui.Track, cols)
	for i := range tracks {
		tracks[i] = ui.Fixed(cell)
	}
	return tracks
}

func (a *App) catTabs(c *ui.Context, th *ui.Theme) {
	ui.ScrollBoth(c).MaxHeight(48).Children(func() {
		ui.Row(c).Gap(20).AlignItems(ui.End).Children(func() {
			for _, cat := range a.cats {
				cat := cat
				a.tab(c, th, "cat:"+cat.ID, cat.Name, cat.ID == a.catID, func() { a.pickCat(cat.ID) })
			}
		})
	})
}

func (a *App) tab(c *ui.Context, th *ui.Theme, key, label string, on bool, click func()) {
	p := ui.Column(c.Key(key)).Gap(8).Padding(10, 2).OnClick(click).Cursor(ui.CursorPointer)
	hovered := p.Hovered()
	fg := th.TextMuted
	if on || hovered {
		fg = th.Text
	}
	p.Children(func() {
		ui.Text(c, label).FontSize(fontBody).TextColor(fg).SingleLine()
		bar := ui.Box(c).Height(2).Radius(1)
		switch {
		case on:
			bar.Background(th.Accent)
		case hovered:
			bar.Background(th.Border)
		}
	})
}

func (a *App) viewContinue(c *ui.Context, th *ui.Theme) {
	if a.lib == nil || a.screen != screenHome {
		return
	}
	watches := a.lib.watches()
	if len(watches) == 0 {
		return
	}
	if len(watches) > 8 {
		watches = watches[:8]
	}
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, "继续观看").FontSize(fontTitle).Bold()
		ui.ScrollBoth(c).MaxHeight(210).Children(func() {
			ui.Row(c).Gap(12).Children(func() {
				for _, w := range watches {
					w := w
					card := ui.Column(c.Key("w-" + w.Key)).Width(104).Gap(8).OnClick(func() {
						a.playHistory(w)
					}).Cursor(ui.CursorPointer)
					hot := card.Hovered()
					card.Children(func() {
						frame := ui.Column(c.Key("continue-frame:" + w.Key)).Width(104).Height(156).Radius(8).Clip().Background(th.Surface)
						a.animateCard(card, frame, hot, a.light)
						frame.Children(func() {
							a.cover(c, th, w.Pic, w.Title, 104, 156, nil, hot)
							if hot {
								a.removeBadge(c, th, "remove-continue:"+w.Key, "从继续观看删除", func() {
									a.lib.removeWatch(w.Key)
								})
							}
						})
						ui.Text(c, w.Title).FontSize(fontCaption).SingleLine()
						frac := float32(0)
						if w.Duration > 0 {
							frac = float32(w.Position / w.Duration)
						}
						ui.Box(c).FillWidth().Height(2).Radius(1).Background(th.SurfaceHover).Children(func() {
							ui.Box(c).Height(2).WidthPercent(frac * 100).Background(th.Accent)
						})
					})
				}
			})
		})
	})
}

func (a *App) posterCard(c *ui.Context, th *ui.Theme, siteKey string, item model.Item, cellW, cellH float32) {
	a.posterCardWithRemove(c, th, siteKey, item, cellW, cellH, nil)
}

func (a *App) posterCardWithRemove(c *ui.Context, th *ui.Theme, siteKey string, item model.Item, cellW, cellH float32, remove func()) {
	open := func() {
		if siteKey != "" {
			a.siteKey = siteKey
		}
		a.openDetail(item)
	}
	card := ui.Column(c.Key("card:" + siteKey + ":" + item.ID)).Width(cellW).Gap(8).OnClick(open).Cursor(ui.CursorPointer)
	hot := card.Hovered()
	card.Children(func() {
		frame := ui.Column(c.Key("poster-frame:" + siteKey + ":" + item.ID)).Width(cellW).Height(cellH).Radius(8).Clip().Background(th.Surface)
		a.animateCard(card, frame, hot, a.light)
		frame.Children(func() {
			a.cover(c, th, item.Pic, item.Title, cellW, cellH, open, hot)
			if item.Remarks != "" {
				ui.Column(c).Absolute().Left(8).Top(8).Padding(3, 6).Radius(4).Background(ui.RGBA(0, 0, 0, 0.62)).Children(func() {
					ui.Text(c, item.Remarks).FontSize(fontCaption).TextColor(th.AccentText).SingleLine()
				})
			}
			if hot && remove != nil {
				a.removeBadge(c, th, "remove-poster:"+siteKey+":"+item.ID, "删除收藏", remove)
			}
		})
		ui.Text(c, item.Title).FontSize(fontBody).SingleLine().OnClick(open)
	})
}

func (a *App) removeBadge(c *ui.Context, th *ui.Theme, key, label string, remove func()) {
	badge := ui.Column(c.Key(key)).Absolute().Right(8).Top(8).Size(24, 24).Radius(12).Center().
		Background(ui.RGBA(0, 0, 0, 0.72)).OnClick(remove).Cursor(ui.CursorPointer).Tooltip(label).Label(label)
	if badge.Hovered() {
		badge.Background(th.Accent)
	}
	badge.Transition(ui.ElementTransition{Duration: 150 * time.Millisecond}).Children(func() {
		ui.Icon(c, iconClose).Size(14, 14).TextColor(th.AccentText)
	})
}

// coverPreload is how far outside the viewport, in DIPs, a cover starts
// loading, so posters are there by the time they scroll in.
const coverPreload = 600

func (a *App) cover(c *ui.Context, th *ui.Theme, pic, title string, w, h float32, click func(), zoom bool) {
	pic = model.ResolveURL("", pic)
	if pic != "" && !a.failed[pic] {
		// One element while loading and after, so the poster fades in over
		// its placeholder once it arrives instead of popping in.
		box := ui.Box(c.Key("cover:" + pic)).Width(w).Height(h)
		// Only covers near the viewport load and count as drawn. Otherwise a
		// long grid marks every poster as just drawn each frame, the cache
		// evicts ones on screen, they reload and evict others: posters blink
		// and new ones never get a download slot.
		var bmp *ui.Bitmap
		if box.InViewMargin(coverPreload) {
			bmp = a.poster(pic)
		} else {
			bmp = a.cachedPoster(pic)
			// InViewMargin answers from the frame before; a cover laid out for
			// the first time needs one more frame to know whether it shows.
			if laid := a.screen + "|" + pic; !a.coverLaid[laid] {
				if a.coverLaid == nil || len(a.coverLaid) > 20000 {
					a.coverLaid = map[string]bool{}
				}
				a.coverLaid[laid] = true
				c.After(50 * time.Millisecond)
			}
		}
		in := fadeIn(box, bmp != nil)
		if in < 1 {
			box.Background(th.SurfaceHover)
		}
		box.Children(func() {
			if bmp == nil || in <= 0 {
				return
			}
			img := ui.Image(c, bmp).Width(w).Height(h).Fit(ui.Cover).Transition(ui.ElementTransition{Duration: 180 * time.Millisecond})
			if in < 1 {
				img.Opacity(in)
			}
			if zoom {
				img.Scale(1.03, 1.03)
			}
			if click != nil {
				img.OnClick(click)
			}
		})
		return
	}
	ui.Column(c).Width(w).Height(h).Center().Gradient(th.Surface, th.SurfaceHover, 160).Children(func() {
		ui.Text(c, mark(title)).FontSize(fontHero).Bold().TextColor(th.TextMuted)
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
	ui.Column(c).Gap(16).Children(func() {
		ui.Grid(c).ColumnTracks(tracks...).Gap(16).Children(func() {
			for i := 0; i < cols; i++ {
				ui.Column(c).Width(cellW).Gap(8).Children(func() {
					ui.Box(c).Width(cellW).Height(cellH).Radius(8).Background(th.Surface)
					ui.Box(c).Width(cellW * 0.62).Height(12).Radius(4).Background(th.SurfaceHover)
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
	if a.detailErr != "" {
		a.noteWithActions(c, th, a.detailErr, true, stateAction{label: "重试", primary: true, click: a.retryDetail})
	}
	d := a.detail
	title := a.detailTitle
	if d != nil && d.Title != "" {
		title = d.Title
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(22).Children(func() {
			ui.Row(c).Gap(32).AlignItems(ui.Start).Padding(8, 0, 6, 0).Children(func() {
				pic := ""
				if d != nil {
					pic = d.Pic
				}
				_, winH := c.Size()
				a.portrait(c, th, pic, title, portraitHeight(winH))
				ui.Column(c).Grow(1).Shrink(1).Gap(14).Padding(6, 0, 0, 0).Children(func() {
					ui.Text(c, title).FontSize(34).FontWeight(weightHeavy).Wrap()
					if d == nil {
						ui.Column(c).Gap(10).Children(func() {
							ui.Box(c).Width(240).Height(12).Radius(6).Background(th.SurfaceHover)
							ui.Box(c).Width(180).Height(12).Radius(6).Background(th.SurfaceHover)
							ui.Box(c).Width(208).Height(12).Radius(6).Background(th.SurfaceHover)
						})
						return
					}
					ui.Row(c).Gap(6).Wrap().Children(func() {
						if d.Remarks != "" {
							metaPill(c, th, d.Remarks, true)
						}
						for _, m := range []string{d.Year, d.Area, d.TypeName} {
							if strings.TrimSpace(m) != "" {
								metaPill(c, th, strings.TrimSpace(m), false)
							}
						}
					})
					who := joinMeta(prefix("导演 ", d.Director), prefix("主演 ", d.Actors))
					if who != "" {
						ui.Text(c, who).FontSize(fontCaption).TextColor(th.TextMuted).Wrap()
					}
					if d.Content != "" {
						ui.Text(c, d.Content).FontSize(fontBody).LineHeight(1.6).TextColor(th.Text.Alpha(0.78)).MaxWidth(760).MaxLines(5).Wrap()
					}
					ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(8, 0, 0, 0).Children(func() {
						if a.linePick >= 0 && a.linePick < len(d.Lines) && len(d.Lines[a.linePick].Episodes) > 0 {
							ep := d.Lines[a.linePick].Episodes[0]
							ln := d.Lines[a.linePick].Name
							label := "播放"
							if ep.Name != "" {
								label = "播放 " + ep.Name
							}
							if primaryPill(c, th, "detail-play", iconPlay, label).Clicked() {
								a.playEpisode(ln, ep)
							}
						}
						on := a.lib != nil && a.lib.hasFav(mediaKey(a.siteKey, d.ID))
						tip, fg := "收藏", th.Text
						if on {
							tip, fg = "取消收藏", th.Accent
						}
						glassButton(c, th, "fav-detail", iconHeart, tip, 44, fg, func() {
							if a.lib == nil {
								return
							}
							a.lib.toggleFav(favItem{
								Key: mediaKey(a.siteKey, d.ID), SiteKey: a.siteKey, VodID: d.ID,
								Title: d.Title, Pic: d.Pic, Remarks: d.Remarks, Year: d.Year,
							})
						})
					})
				})
			})
			if d == nil {
				if a.detailLoading {
					msg := "正在读取影片详情…"
					if len(a.offers) > 0 {
						msg = "正在读取各片源的线路和清晰度…"
					}
					a.loadingRow(c, th, msg)
				} else if a.detailErr == "" {
					a.emptyState(c, th, "暂时没有详情", "返回列表，或换一个片源再试。",
						stateAction{label: "返回", primary: true, click: a.back})
				}
				if len(a.offers) > 0 {
					a.viewOffers(c, th)
				}
				return
			}
			if len(a.offers) > 0 {
				a.viewOffers(c, th)
			}
			if len(d.Lines) == 0 {
				a.emptyState(c, th, "没有剧集", "这个详情页没有返回可播放的地址。",
					stateAction{label: "返回", primary: true, click: a.back})
				return
			}
			if a.linePick >= len(d.Lines) {
				a.linePick = 0
			}
			if len(d.Lines) > 1 {
				ui.ScrollBoth(c).MaxHeight(52).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						for i, line := range d.Lines {
							i := i
							a.choice(c, th, "line:"+line.Name, lineLabel(line.Name, i), i == a.linePick, func() { a.linePick = i })
						}
					})
				})
			} else if !slugLine(d.Lines[0].Name) {
				ui.Text(c, d.Lines[0].Name).FontSize(fontCaption).Bold().TextColor(th.TextMuted)
			}
			line := d.Lines[a.linePick]
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "剧集").FontSize(fontTitle).Bold()
				ui.Text(c, fmt.Sprintf("%d 集", len(line.Episodes))).FontSize(fontCaption).TextColor(th.TextMuted)
			})
			tracks := a.episodeTracks(c, longestEpisode(line.Episodes))
			ui.ScrollBoth(c).MaxHeight(260).Children(func() {
				ui.Grid(c).ColumnTracks(tracks...).Gap(8).Children(func() {
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
	})
}

func (a *App) portrait(c *ui.Context, th *ui.Theme, pic, title string, h float32) {
	w := h * 2 / 3
	frame := ui.Column(c).Width(w).Height(h).Radius(14).Clip().Background(th.Surface).
		Border(1, th.Border).Shadow(0, 24, 48, -12, ui.RGBA(0, 0, 0, 0.55))
	frame.Children(func() {
		a.cover(c, th, pic, title, w, h, nil, false)
	})
}

func (a *App) viewLive(c *ui.Context, th *ui.Theme) {
	if a.liveLoading && len(a.channels) == 0 {
		_, cellW, cellH, _ := a.posterGrid(c)
		a.skeletonRow(c, th, 4, cellW, cellH)
		return
	}
	if a.liveErr != "" {
		a.noteWithActions(c, th, a.liveErr, true, stateAction{label: "重试", primary: true, click: a.retryLive})
		return
	}
	chs := a.visibleChannels()
	if len(chs) == 0 {
		if strings.TrimSpace(a.query) != "" && len(a.channels) > 0 {
			a.emptyState(c, th, "没有频道符合「"+strings.TrimSpace(a.query)+"」", "清空搜索框可以看到全部频道。",
				stateAction{label: "清空搜索", primary: true, click: func() {
					a.query = ""
					a.committed = ""
				}})
			return
		}
		a.emptyState(c, th, "没有频道", "这份播放列表是空的。",
			stateAction{label: "重新加载列表", primary: true, click: a.retryLive},
			stateAction{label: "管理订阅", click: a.openSubs})
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
	base := ""
	if a.catalog != nil && a.liveIndex >= 0 && a.liveIndex < len(a.catalog.Lives) {
		if entry := a.catalog.Lives[a.liveIndex]; entry != nil {
			base = entry.URL
		}
	}
	ui.Row(c).Grow(1).Gap(12).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(168).Gap(8).Children(func() {
			ui.Text(c, "分组").FontSize(fontCaption).TextColor(th.TextMuted)
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
		playing := (a.player != nil && a.fromLive) || (a.playLoading && a.fromLive)
		chCol := ui.Column(c)
		if playing {
			chCol = chCol.Width(300)
		} else {
			chCol = chCol.Grow(1)
		}
		chCol.Children(func() {
			ui.Text(c, fmt.Sprintf("%d 个频道", len(shown))).FontSize(fontCaption).TextColor(th.TextMuted)
			ui.List(c, &a.chState, len(shown), func(i int) {
				ch := shown[i]
				row := ui.Row(c).FillWidth().Padding(8, 12).Radius(8).Gap(12).AlignItems(ui.Center)
				bg := ui.Transparent
				if row.Hovered() || ch.Name == a.playTitle {
					bg = th.SurfaceHover
				}
				row.Background(bg).Children(func() {
					a.liveLogo(c, th, ch, base)
					ui.Text(c, ch.Name).Grow(1).SingleLine()
					a.iconBtn(c, "ch:"+ch.Group+"/"+ch.Name+"/"+ch.URL, iconPlay, "播放", 44, 20, th.Text, func() { a.playChannel(ch) })
				})
			}).Grow(1)
		})
		if a.player != nil && a.fromLive || a.playLoading && a.fromLive {
			ui.Column(c).Grow(1).Children(func() {
				a.paintStage(c, th, true)
			})
		}
	})
}

func (a *App) liveLogo(c *ui.Context, th *ui.Theme, ch live.Channel, base string) {
	frame := ui.Column(c).Size(36, 36).Radius(8).Background(th.Surface).Center().Clip()
	if logo := model.ResolveURL(base, ch.Logo); logo != "" {
		if bmp := a.poster(logo); bmp != nil {
			frame.Children(func() {
				ui.Image(c, bmp).Size(30, 30).Fit(ui.Contain)
			})
			return
		}
	}
	frame.Children(func() {
		ui.Text(c, mark(ch.Name)).FontSize(fontBody).Bold().TextColor(th.Text)
	})
}

func (a *App) viewUnsupported(c *ui.Context, th *ui.Theme) {
	m, ok := a.currentMeta()
	if !ok {
		a.emptyState(c, th, "没有选中的站点", "",
			stateAction{label: "回到首页", primary: true, click: a.goHome})
		return
	}
	ui.Column(c).Grow(1).Center().Children(func() {
		ui.Column(c).Width(460).Gap(12).Children(func() {
			ui.Box(c).Width(28).Height(2).Radius(1).Background(th.Accent)
			ui.Text(c, m.Name).FontSize(fontHero).Bold()
			ui.Text(c, config.KindLabel(m.Kind)).FontSize(fontCaption).TextColor(th.TextMuted)
			reason := m.Reason
			if reason == "" {
				reason = "这个站点本版不支持。"
			}
			ui.Text(c, reason).Wrap()
			if m.Kind == config.KindJar {
				ui.Text(c, "QuickBo 不执行 jar，也不会去加载 DEX。换一个 CMS、JS 或 XBPQ 源即可。").TextColor(th.TextMuted).Wrap()
			}
			if m.API != "" {
				ui.Text(c, m.API).FontSize(fontCaption).TextColor(th.TextMuted).Selectable().Wrap()
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
	if a.loading {
		ui.Column(c).Gap(8).Padding(8, 0).Children(func() {
			msg := a.loadNote
			if msg == "" {
				msg = "正在批量加载配置…"
			}
			a.loadingRow(c, th, msg)
			ui.Text(c, "打不开，或第一页没有内容的源，不会出现在侧栏。").TextColor(th.TextMuted).Wrap()
		})
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(12).Children(func() {
			ui.Text(c, "这是一份仓库索引，还不是点播配置。").FontSize(fontHeadline).Bold()
			ui.Text(c, "选一个地址继续加载。").TextColor(th.TextMuted)
			if a.catalog == nil {
				return
			}
			ui.Column(c).Gap(8).Children(func() {
				for _, u := range a.catalog.Index {
					u := u
					name := u.Name
					if name == "" {
						name = u.URL
					}
					row := ui.Column(c.Key(u.URL)).FillWidth().Padding(14, 16).Gap(4).Radius(8).Background(th.Surface).OnClick(func() { a.Load(u.URL) }).Cursor(ui.CursorPointer)
					if row.Hovered() {
						row.Background(th.SurfaceHover)
					}
					row.Children(func() {
						ui.Text(c, name).Bold()
						ui.Text(c, u.URL).FontSize(fontCaption).TextColor(th.TextMuted).Selectable()
					})
				}
			})
		})
	})
}

// choice is a chip that picks one of several (a source, a line, an
// episode). The chosen one is tinted and outlined with the accent rather
// than filled, leaving the filled look to the screen's primary action.
func (a *App) choice(c *ui.Context, th *ui.Theme, key, label string, on bool, click func()) {
	p := ui.Column(c.Key(key)).Padding(9, 14).Radius(10).OnClick(click).Cursor(ui.CursorPointer)
	// A long label, as a variety show's episode, may be cut to fit: the
	// pointer resting on it shows it whole.
	if utf8.RuneCountInString(label) > choiceTipRunes {
		p.Tooltip(label)
	}
	bg, fg := selectionColors(th, on, p.Hovered())
	edge := th.Border
	if on {
		edge = th.Accent.Alpha(0.55)
	}
	p.Background(p.AnimateColor("bg", bg, colorEase)).Border(1, p.AnimateColor("edge", edge, colorEase))
	pressPose(p, 0.96)
	p.Children(func() {
		w := 400
		if on {
			w = weightStrong
		}
		ui.Text(c, label).FontSize(fontBody).FontWeight(w).TextColor(fg).SingleLine()
	})
}

// deadChoice is a choice that cannot be picked: greyed, marked 无法播放,
// with why in its tooltip.
func (a *App) deadChoice(c *ui.Context, th *ui.Theme, key, label, reason string) {
	tip := "无法播放"
	if reason != "" {
		tip += "：" + reason
	}
	ui.Row(c.Key(key)).Padding(10, 12).Radius(8).Gap(6).AlignItems(ui.Center).
		Background(th.SurfaceHover.Alpha(0.45)).Cursor(ui.CursorNotAllowed).Tooltip(tip).Label(label + "，" + tip).
		Children(func() {
			ui.Text(c, label).FontSize(fontBody).TextColor(th.TextMuted.Alpha(0.6)).SingleLine()
			ui.Text(c, "无法播放").FontSize(fontCaption).TextColor(th.Danger.Alpha(0.75)).SingleLine()
		})
}

type stateAction struct {
	label   string
	primary bool
	click   func()
}

func (a *App) actionRow(c *ui.Context, actions []stateAction) {
	if len(actions) == 0 {
		return
	}
	ui.Row(c).Gap(8).Children(func() {
		for _, action := range actions {
			action := action
			var btn ui.Element
			if action.primary {
				btn = ui.PrimaryButton(c, action.label)
			} else {
				btn = ui.Button(c, action.label)
			}
			if btn.Clicked() && action.click != nil {
				action.click()
			}
		}
	})
}

func (a *App) note(c *ui.Context, th *ui.Theme, text string, bad bool) {
	a.noteWithActions(c, th, text, bad)
}

func (a *App) noteWithActions(c *ui.Context, th *ui.Theme, text string, bad bool, actions ...stateAction) {
	bg := th.Surface
	fg := th.TextMuted
	if bad {
		bg = th.Danger.Alpha(0.14)
		fg = th.Danger
	}
	ui.Column(c).FillWidth().Padding(12, 14).Radius(8).Background(bg).Gap(10).Children(func() {
		ui.Text(c, text).TextColor(fg).Wrap()
		a.actionRow(c, actions)
	})
}

func (a *App) emptyState(c *ui.Context, th *ui.Theme, title, body string, actions ...stateAction) {
	ui.Column(c).FillWidth().Padding(36, 0).Gap(10).Children(func() {
		ui.Box(c).Width(28).Height(2).Radius(1).Background(th.Accent)
		ui.Text(c, title).FontSize(fontHeadline).Bold()
		if body != "" {
			ui.Text(c, body).TextColor(th.TextMuted).Wrap()
		}
		a.actionRow(c, actions)
	})
}

func (a *App) retrySubs() {
	a.loadErr = ""
	if len(a.activeRefs()) == 0 {
		a.openSubs()
		return
	}
	a.LoadSaved()
}

func (a *App) retryList() {
	a.listErr = ""
	a.loadList()
}

func (a *App) retryDetail() {
	if a.detailItem.ID == "" {
		return
	}
	a.openDetail(a.detailItem)
}

func (a *App) retryLive() {
	if a.catalog == nil || a.liveIndex < 0 || a.liveIndex >= len(a.catalog.Lives) {
		a.openSubs()
		return
	}
	a.selectLive(a.liveIndex)
}

func (a *App) retryPlay() {
	if strings.TrimSpace(a.playURL) == "" {
		a.openSubs()
		return
	}
	a.playErr = ""
	a.startPlay(a.playTitle, a.playURL, a.playHeaders, a.fromLive)
}

func (a *App) goHome() {
	a.closePlayer()
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
	a.query = ""
	a.committed = ""
	a.homeHits = nil
	a.homeSearching = false
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

func (a *App) viewHistory(c *ui.Context, th *ui.Theme) {
	items := a.lib.watches()
	if len(items) == 0 {
		a.emptyState(c, th, "还没有历史", "播放过的片子会出现在这里，并记住进度。",
			stateAction{label: "去首页", primary: true, click: a.goHome})
		return
	}
	_, cellW, cellH, tracks := a.posterGrid(c)
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Grid(c).ColumnTracks(tracks...).Gap(16).Children(func() {
			for _, w := range items {
				w := w
				card := ui.Column(c.Key("hist-" + w.Key)).Width(cellW).Gap(8).OnClick(func() {
					a.playHistory(w)
				}).Cursor(ui.CursorPointer)
				hot := card.Hovered()
				card.Children(func() {
					frame := ui.Column(c.Key("history-frame:" + w.Key)).Width(cellW).Height(cellH).Radius(8).Clip().Background(th.Surface)
					a.animateCard(card, frame, hot, a.light)
					frame.Children(func() {
						a.cover(c, th, w.Pic, w.Title, cellW, cellH, nil, hot)
						if hot {
							a.removeBadge(c, th, "remove-history:"+w.Key, "删除历史", func() {
								a.lib.removeWatch(w.Key)
							})
						}
					})
					ui.Text(c, w.Title).FontSize(fontBody).SingleLine()
					sub := w.Episode
					if line := displayLine(w.Line); line != "" && sub != "" {
						sub = line + "  ·  " + sub
					}
					ui.Text(c, sub).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
					frac := float32(0)
					if w.Duration > 0 {
						frac = float32(w.Position / w.Duration)
					}
					ui.Box(c).Width(cellW).Height(3).Radius(2).Background(th.SurfaceHover).Children(func() {
						ui.Box(c).Height(3).WidthPercent(frac * 100).Background(th.Accent)
					})
				})
			}
		})
	})
}

func (a *App) viewFavs(c *ui.Context, th *ui.Theme) {
	items := a.lib.favs()
	if len(items) == 0 {
		a.emptyState(c, th, "还没有收藏", "在详情页点心形图标，片子会留在这里。",
			stateAction{label: "去首页", primary: true, click: a.goHome})
		return
	}
	_, cellW, cellH, tracks := a.posterGrid(c)
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Grid(c).ColumnTracks(tracks...).Gap(16).Children(func() {
			for _, f := range items {
				f := f
				a.posterCardWithRemove(c, th, f.SiteKey,
					model.Item{ID: f.VodID, Title: f.Title, Pic: f.Pic, Remarks: f.Remarks, Year: f.Year},
					cellW, cellH, func() { a.lib.removeFav(f.Key) })
			}
		})
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
	return a.resumeEpisode
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

func slugLine(name string) bool {
	n := strings.TrimSpace(name)
	if n == "" || len([]rune(n)) > 16 {
		return false
	}
	for _, r := range n {
		if r > 127 || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-') {
			return false
		}
	}
	return true
}

func lineLabel(name string, index int) string {
	if slugLine(name) {
		return fmt.Sprintf("线路 %d", index+1)
	}
	n := strings.TrimSpace(name)
	if n == "" {
		return fmt.Sprintf("线路 %d", index+1)
	}
	return n
}

func displayLine(name string) string {
	if slugLine(name) {
		return ""
	}
	return strings.TrimSpace(name)
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
