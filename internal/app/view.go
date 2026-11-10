package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/mybox/internal/config"
)

// View draws one frame of the native window.
func (a *App) View(c *ui.Context) {
	th := c.Theme()
	if a.catalog == nil {
		a.viewWelcome(c, th)
	} else {
		ui.Row(c).Fill().Children(func() {
			a.viewSidebar(c, th)
			ui.Column(c).Grow(1).Padding(16, 18).Gap(12).Children(func() {
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
					a.message(c, th, "这份配置里没有点播站，也没有直播。")
				default:
					a.viewBrowse(c, th)
				}
			})
		})
	}
	a.viewAdd(c, th)
}

func (a *App) viewWelcome(c *ui.Context, th *ui.Theme) {
	ui.Column(c).Fill().Center().Children(func() {
		ui.Column(c).Width(520).Gap(12).Children(func() {
			ui.Text(c, "mybox").FontSize(36).Bold()
			ui.Text(c, "桌面 TVBox 客户端").FontSize(16).TextColor(th.TextMuted)
			ui.Text(c, "不内置任何片源。把你自己的配置 URL 或本地 JSON 加进来，再浏览、搜索和播放。").TextColor(th.TextMuted).Wrap()
			ui.Text(c, "支持 JSON / XML CMS，以及一部分 JS（drpy）和 XBPQ 规则。JAR/DEX 爬虫（csp_*）会列出来，但本版不执行 jar。").TextColor(th.TextMuted).Wrap()
			if a.loading {
				ui.Row(c).Gap(8).Children(func() {
					ui.Spinner(c)
					ui.Text(c, "正在读取配置…")
				})
			}
			if a.loadErr != "" {
				ui.Text(c, a.loadErr).TextColor(th.Danger).Wrap()
			}
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, "添加配置").Clicked() {
					a.adding = true
				}
			})
			if ui.Checkbox(c, &a.preferExternal, "优先使用 mpv 播放").Changed() {
				a.saveSettings()
			}
		})
	})
}

func (a *App) viewAdd(c *ui.Context, th *ui.Theme) {
	ui.Modal(c, &a.adding, func() {
		ui.Column(c).Width(480).Padding(20).Gap(12).Children(func() {
			ui.Text(c, "添加配置").FontSize(18).Bold()
			ui.Text(c, "TVBox / FongMi 配置的 URL，或本机 JSON 文件。mybox 不内置任何片源。").TextColor(th.TextMuted).Wrap()
			field := ui.TextInput(c, &a.urlDraft).Placeholder("https://example.com/tvbox.json").Label("配置 URL")
			if field.Submitted() {
				a.adding = false
				a.Load(a.urlDraft)
			}
			if a.loadErr != "" && a.catalog != nil {
				ui.Text(c, a.loadErr).TextColor(th.Danger).Wrap()
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
	side := ui.Sidebar(c, &a.sideSel, func() {
		if a.catalog != nil && len(a.catalog.Sites) > 0 {
			ui.SidebarSection(c, "点播", nil, func() {
				for _, s := range a.catalog.Sites {
					m := s.Meta()
					label := m.Name
					switch m.Level {
					case config.LevelNo:
						label += "  ·  不支持"
					case config.LevelPartial:
						label += "  ·  部分"
					}
					ui.SidebarItem(c, "site:"+m.Key, nil, label)
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
	}).Width(248)
	if side.Changed() {
		a.onSide()
	}
	_ = th
}

func (a *App) viewTop(c *ui.Context, th *ui.Theme) {
	ui.Row(c).Gap(8).Children(func() {
		if a.screen == screenDetail || a.screen == screenPlay {
			if ui.Button(c, "返回").Clicked() {
				a.back()
			}
		}
		title := "mybox"
		if m, ok := a.currentMeta(); ok && a.screen != screenLive && a.screen != screenIndex {
			title = m.Name
			ui.Text(c, title).FontSize(18).Bold().Shrink(1)
			ui.Badge(c, config.KindLabel(m.Kind))
		} else if a.screen == screenLive && a.catalog != nil && a.liveIndex >= 0 && a.liveIndex < len(a.catalog.Lives) {
			ui.Text(c, a.catalog.Lives[a.liveIndex].Name).FontSize(18).Bold().Shrink(1)
		} else {
			ui.Text(c, title).FontSize(18).Bold()
		}
		ui.Column(c).Grow(1)
		if a.screen == screenBrowse || a.screen == screenLive {
			ph := "搜索当前源，回车开始"
			if a.screen == screenLive {
				ph = "过滤频道"
			}
			field := ui.SearchField(c, &a.query).Placeholder(ph).Width(260).Label("搜索")
			if a.screen == screenBrowse && (field.Submitted() || (field.Changed() && strings.TrimSpace(a.query) == "")) {
				a.pageNum = 1
				a.loadList()
			}
		}
		if ui.Button(c, "更换配置").Clicked() {
			a.urlDraft = a.cfgRef
			a.adding = true
		}
	})
	if a.loading {
		ui.Text(c, "正在读取新的配置…").TextColor(th.TextMuted)
	}
	if a.loadErr != "" {
		ui.Text(c, a.loadErr).TextColor(th.Danger).Wrap()
	}
}

func (a *App) viewBrowse(c *ui.Context, th *ui.Theme) {
	if m, ok := a.currentMeta(); ok && m.Level == config.LevelPartial && m.Reason != "" {
		ui.Text(c, m.Reason).FontSize(12).TextColor(th.TextMuted).Wrap()
	}
	if len(a.cats) > 0 {
		ui.ScrollBoth(c).MaxHeight(48).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				for _, cat := range a.cats {
					var b ui.Element
					if cat.ID == a.catID && strings.TrimSpace(a.query) == "" {
						b = ui.PrimaryButton(c.Key("cat:"+cat.ID), cat.Name)
					} else {
						b = ui.Button(c.Key("cat:"+cat.ID), cat.Name)
					}
					if b.Clicked() {
						a.pickCat(cat.ID)
					}
				}
			})
		})
	}
	if a.listLoad {
		ui.Row(c).Gap(8).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在加载…").TextColor(th.TextMuted)
		})
	}
	if a.listErr != "" {
		ui.Text(c, a.listErr).TextColor(th.Danger).Wrap()
	}
	if a.selected >= len(a.page.Items) {
		a.selected = -1
	}
	n := len(a.page.Items)
	if n == 0 && !a.listLoad && a.listErr == "" {
		a.message(c, th, "没有结果。")
		return
	}
	grid := ui.GridView(c, &a.grid, n, 168, 292, func(i int) {
		item := a.page.Items[i]
		open := func() { a.openDetail(item) }
		ui.Column(c).Padding(8).Gap(6).Radius(12).Background(th.Surface).OnClick(open).Children(func() {
			a.posterBox(c, th, item.Pic, 196, open)
			ui.Text(c, item.Title).SingleLine().OnClick(open)
			remarks := item.Remarks
			if remarks == "" {
				remarks = " "
			}
			ui.Text(c, remarks).FontSize(12).TextColor(th.TextMuted).SingleLine()
		})
	}).Grow(1)
	if grid.Submitted() && a.selected >= 0 && a.selected < n {
		a.openDetail(a.page.Items[a.selected])
	}
	a.viewPager(c, th)
}

func (a *App) viewPager(c *ui.Context, th *ui.Theme) {
	if a.pageNum <= 1 && !a.page.HasNext() {
		return
	}
	label := fmt.Sprintf("第 %d 页", a.pageNum)
	if a.page.PageCount > 0 {
		label = fmt.Sprintf("第 %d / %d 页", a.pageNum, a.page.PageCount)
	}
	ui.Row(c).Gap(8).Children(func() {
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
	if a.detailLoading {
		ui.Row(c).Gap(8).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在读取详情…").TextColor(th.TextMuted)
		})
	}
	if a.detailErr != "" {
		ui.Text(c, a.detailErr).TextColor(th.Danger).Wrap()
	}
	d := a.detail
	title := a.detailTitle
	if d != nil && d.Title != "" {
		title = d.Title
	}
	ui.Row(c).Gap(16).Children(func() {
		pic := ""
		if d != nil {
			pic = d.Pic
		}
		a.posterBox(c, th, pic, 220, nil)
		ui.Column(c).Grow(1).Gap(6).Children(func() {
			ui.Text(c, title).FontSize(22).Bold().Wrap()
			if d != nil {
				meta := joinMeta(d.TypeName, d.Year, d.Area, d.Remarks)
				if meta != "" {
					ui.Text(c, meta).TextColor(th.TextMuted).Wrap()
				}
				if d.Content != "" {
					ui.Text(c, d.Content).Wrap()
				}
			}
		})
	})
	if d == nil {
		return
	}
	if len(d.Lines) == 0 {
		ui.Text(c, "没有可播放的剧集。").TextColor(th.TextMuted)
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(12).Children(func() {
			for _, line := range d.Lines {
				ui.Text(c, line.Name).Bold()
				ui.Grid(c).Columns(4).Gap(6).Children(func() {
					lineName := line.Name
					for _, ep := range line.Episodes {
						ep := ep
						label := ep.Name
						if label == "" {
							label = "播放"
						}
						if ui.Button(c.Key(lineName+"\n"+ep.URL+"\n"+label), label).Clicked() {
							a.playEpisode(lineName, ep)
						}
					}
				})
			}
		})
	})
}

func (a *App) viewPlay(c *ui.Context, th *ui.Theme) {
	ui.Row(c).Gap(8).Children(func() {
		ui.Text(c, a.playTitle).FontSize(18).Bold().Shrink(1)
		ui.Column(c).Grow(1)
		if ui.Button(c, "用 mpv 播放").Clicked() {
			a.openExternal()
		}
	})
	if ui.Checkbox(c, &a.preferExternal, "之后优先使用 mpv").Changed() {
		a.saveSettings()
	}
	if a.playLoading {
		ui.Row(c).Gap(8).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在解析播放地址…").TextColor(th.TextMuted)
		})
	}
	if a.playErr != "" {
		ui.Text(c, a.playErr).TextColor(th.Danger).Wrap()
	}
	if a.playNote != "" {
		ui.Text(c, a.playNote).TextColor(th.TextMuted).Wrap()
	}
	if a.playURL != "" {
		ui.Text(c, a.playURL).FontSize(12).TextColor(th.TextMuted).Selectable().Wrap()
	}
	if a.player != nil {
		ui.Column(c).Height(420).Radius(12).Background(th.Surface).Children(func() {
			ui.Video(c, a.player).Grow(1).Fit(ui.Contain)
		})
		if !a.player.Ready() || a.player.Buffering() {
			ui.Row(c).Gap(8).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "缓冲…").TextColor(th.TextMuted)
			})
		}
		if err := a.player.Err(); err != nil {
			ui.Text(c, err.Error()).TextColor(th.Danger).Wrap()
		}
		ui.Row(c).Gap(8).Children(func() {
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
				ui.Text(c, "直播或时长未知").TextColor(th.TextMuted).Grow(1)
			}
			vol := a.volume
			if ui.Slider(c.Key("volume"), &vol, 0, 1).Width(120).Label("音量").Changed() {
				a.volume = vol
				a.player.SetVolume(vol)
				a.saveSettings()
			}
		})
	}
	ui.Text(c, "原生播放走系统解码器：MP4（H.264/AAC）在各平台可用，HLS 在 macOS 和 Windows 可用。Linux 需要 GStreamer，装不上或不支持的流用 mpv。").FontSize(12).TextColor(th.TextMuted).Wrap()
}

func (a *App) viewLive(c *ui.Context, th *ui.Theme) {
	if a.liveLoading {
		ui.Row(c).Gap(8).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在读取播放列表…").TextColor(th.TextMuted)
		})
	}
	if a.liveErr != "" {
		ui.Text(c, a.liveErr).TextColor(th.Danger).Wrap()
		return
	}
	chs := a.visibleChannels()
	if len(chs) == 0 && !a.liveLoading {
		a.message(c, th, "没有匹配的频道。")
		return
	}
	ui.Text(c, fmt.Sprintf("%d 个频道", len(chs))).FontSize(12).TextColor(th.TextMuted)
	ui.List(c, &a.chState, len(chs), func(i int) {
		ch := chs[i]
		label := ch.Name
		if ch.Group != "" {
			label = ch.Group + "   " + ch.Name
		}
		ui.Row(c).Gap(8).Padding(4, 0).Children(func() {
			ui.Text(c, label).Grow(1).SingleLine()
			if ui.Button(c.Key("ch:"+ch.Group+"/"+ch.Name+"/"+ch.URL), "播放").Clicked() {
				a.playChannel(ch)
			}
		})
	}).Grow(1)
}

func (a *App) viewUnsupported(c *ui.Context, th *ui.Theme) {
	m, ok := a.currentMeta()
	if !ok {
		a.message(c, th, "没有选中的站点。")
		return
	}
	ui.Column(c).Gap(8).MaxWidth(640).Children(func() {
		ui.Text(c, m.Name).FontSize(22).Bold()
		ui.Badge(c, config.KindLabel(m.Kind))
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
}

func (a *App) viewIndex(c *ui.Context, th *ui.Theme) {
	ui.Text(c, "这是一份仓库索引，还不是点播配置。选一个地址继续加载。").TextColor(th.TextMuted).Wrap()
	if a.catalog == nil {
		return
	}
	ui.Column(c).Gap(8).Children(func() {
		for _, u := range a.catalog.Index {
			name := u.Name
			if name == "" {
				name = u.URL
			}
			if ui.Button(c.Key(u.URL), name).Clicked() {
				a.Load(u.URL)
			}
			ui.Text(c, u.URL).FontSize(12).TextColor(th.TextMuted).Selectable()
		}
	})
}

func (a *App) posterBox(c *ui.Context, th *ui.Theme, pic string, h float32, open func()) {
	box := ui.Column(c).Width(h * 0.72).Height(h).Radius(10).Background(th.Background).Center()
	if open != nil {
		box = box.OnClick(open)
	}
	box.Children(func() {
		if bmp := a.poster(pic); bmp != nil {
			img := ui.Image(c, bmp).Width(h * 0.72).Height(h).Fit(ui.Cover).Radius(10)
			if open != nil {
				img.OnClick(open)
			}
			return
		}
		ui.Text(c, "暂无海报").FontSize(12).TextColor(th.TextMuted)
	})
}

func (a *App) message(c *ui.Context, th *ui.Theme, text string) {
	ui.Text(c, text).TextColor(th.TextMuted)
}

func joinMeta(parts ...string) string {
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "  ·  ")
}
