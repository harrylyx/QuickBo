package app

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/source"
)

var (
	srcSortOptions   = []string{"名称", "延迟", "类型", "健康度", "订阅"}
	srcHealthOptions = []string{"全部", "仅可用", "仅失败", "仅未测", "较慢"}
	srcViewOptions   = []string{"列表", "按订阅", "按类型", "按健康度"}
)

func (a *App) viewSources(c *ui.Context, th *ui.Theme) {
	ui.Column(c).Grow(1).Gap(16).Children(func() {
		sortSel := a.srcSort
		if sortSel == "" {
			sortSel = "名称"
		}
		healthSel := a.srcHealth
		if healthSel == "" {
			healthSel = "全部"
		}
		viewSel := a.srcView
		if viewSel == "" {
			viewSel = "列表"
		}
		batchLabel := "批量管理"
		if a.srcBatch {
			batchLabel = "完成"
		}
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, "测连通、延迟和服务器所在地，并按健康度排序。停用的源仍在列表里，可随时启用。").TextColor(th.TextMuted).Shrink(1).Wrap()
			ui.Column(c).Grow(1)
			ui.SearchField(c, &a.srcFilter).Placeholder("搜索片源").Width(160).Label("搜索片源")
			ui.Select(c, &sortSel, srcSortOptions).Label("排序").Width(108)
			ui.Select(c, &healthSel, srcHealthOptions).Label("筛选").Width(108)
			ui.Select(c, &viewSel, srcViewOptions).Label("分组").Width(120)
			if ui.Button(c, "全部测速").Disabled(a.probingActive).Clicked() {
				a.probeAllSources()
			}
			if ui.Button(c, batchLabel).Clicked() {
				a.srcBatch = !a.srcBatch
				if !a.srcBatch {
					a.srcSel = nil
				}
			}
		})
		a.srcSort = sortSel
		a.srcHealth = healthSel
		a.srcView = viewSel

		if a.catalog == nil || len(a.allSites) == 0 {
			a.emptyState(c, th, "还没有片源", "添加一份配置后，能列出内容的源会出现在这里。",
				stateAction{label: "添加订阅", primary: true, click: a.openSubs})
			return
		}

		sites := a.filteredSites()
		total, ok, fail, untested, avg := a.sourceSummary()
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("共 %d 个片源", total)).FontSize(fontCaption).TextColor(th.TextMuted)
			ui.Text(c, "·").FontSize(fontCaption).TextColor(th.TextMuted)
			a.summaryCount(c, th, fmt.Sprintf("可用 %d", ok), ok > 0, th.Success)
			a.summaryCount(c, th, fmt.Sprintf("失败 %d", fail), fail > 0, th.Danger)
			a.summaryCount(c, th, fmt.Sprintf("未测 %d", untested), untested > 0, th.TextMuted)
			if ok > 0 {
				ui.Text(c, "·").FontSize(fontCaption).TextColor(th.TextMuted)
				ui.Text(c, fmt.Sprintf("平均延迟 %d ms", avg)).FontSize(fontCaption).TextColor(th.TextMuted)
			}
			ui.Column(c).Grow(1)
			if a.probingActive {
				frac := float32(0)
				if a.probingTotal > 0 {
					frac = float32(a.probingDone) / float32(a.probingTotal)
				}
				ui.Spinner(c)
				ui.Box(c).Width(120).Height(4).Radius(2).Background(th.SurfaceHover).Children(func() {
					ui.Box(c).Height(4).WidthPercent(frac * 100).Radius(2).Background(th.Accent)
				})
				ui.Text(c, fmt.Sprintf("%d/%d", a.probingDone, a.probingTotal)).FontSize(fontCaption).TextColor(th.TextMuted)
			}
		})

		if len(sites) == 0 {
			ui.Text(c, "没有符合当前筛选的片源").TextColor(th.TextMuted)
			return
		}

		if a.srcBatch {
			stop := a.sourceBatchBar(c, th, sites)
			if stop {
				return
			}
		}

		active := a.activeSiteKeys()
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(16).Children(func() {
				for _, g := range a.groupSources(sites) {
					g := g
					if g.name != "" {
						ui.Text(c, g.name).FontSize(fontCaption).Bold().TextColor(th.TextMuted)
					}
					ui.Column(c).Gap(8).Children(func() {
						stop := false
						for _, s := range g.sites {
							stop = a.sourceRow(c, th, s, active)
							if stop {
								break
							}
						}
						if stop {
							return
						}
					})
				}
			})
		})
	})
}

func (a *App) summaryCount(c *ui.Context, th *ui.Theme, label string, highlight bool, col ui.Color) {
	fg := th.TextMuted
	if highlight {
		fg = col
	}
	ui.Text(c, label).FontSize(fontCaption).TextColor(fg).SingleLine()
}

// sourceBatchBar renders the batch-mode toolbar and returns true when one of
// its actions invalidated the current frame (the caller must stop rendering).
func (a *App) sourceBatchBar(c *ui.Context, th *ui.Theme, sites []source.Source) bool {
	all := ui.CheckOff
	selCount := 0
	for _, s := range sites {
		if a.srcSel[s.Meta().Key] {
			selCount++
		}
	}
	switch {
	case selCount == len(sites) && len(sites) > 0:
		all = ui.CheckOn
	case selCount > 0:
		all = ui.CheckMixed
	}
	stop := false
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.CheckboxState(c, &all, "全选").Changed() {
			if a.srcSel == nil {
				a.srcSel = map[string]bool{}
			}
			if all == ui.CheckOn {
				for _, s := range sites {
					a.srcSel[s.Meta().Key] = true
				}
			} else {
				a.srcSel = nil
			}
		}
		ui.Text(c, fmt.Sprintf("已选 %d 个", selCount)).FontSize(fontCaption).TextColor(th.TextMuted)
		keys := a.selectedSourceKeys()
		if ui.Button(c, "测速").Disabled(selCount == 0).Clicked() && selCount > 0 {
			a.probeSources(keys)
		}
		if ui.Button(c, "停用").Disabled(selCount == 0).Clicked() && selCount > 0 {
			a.disableSites(keys)
			stop = true
		}
		if ui.Button(c, "启用").Disabled(selCount == 0).Clicked() && selCount > 0 {
			a.enableSites(keys)
			stop = true
		}
		if ui.Button(c, "加黑名单").Disabled(selCount == 0).Clicked() && selCount > 0 {
			a.addSourcesBlacklist(keys)
			stop = true
		}
		if ui.Button(c, "移除").Disabled(selCount == 0).Clicked() && selCount > 0 {
			a.removeSites(keys)
			stop = true
		}
	})
	return stop
}

// sourceRow renders one source and returns true if it changed visible state.
func (a *App) sourceRow(c *ui.Context, th *ui.Theme, s source.Source, active map[string]bool) bool {
	m := s.Meta()
	key := m.Key
	isActive := active[key]
	isDisabled := a.disabled[sourceHideID(s)]
	stop := false
	ui.Row(c.Key("src:"+key)).FillWidth().Padding(12, 14).Radius(8).Background(th.Surface).AlignItems(ui.Center).Gap(12).Children(func() {
		if a.srcBatch {
			checked := a.srcSel[key]
			if ui.Checkbox(c, &checked, "").Changed() {
				if a.srcSel == nil {
					a.srcSel = map[string]bool{}
				}
				if checked {
					a.srcSel[key] = true
				} else {
					delete(a.srcSel, key)
				}
			}
		}
		ui.Column(c).Grow(1).Gap(4).Shrink(1).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, m.Name).FontSize(fontTitle).Bold().SingleLine()
				if isDisabled {
					ui.Text(c, "已停用").FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
				} else if !isActive && m.Level != config.LevelNo {
					ui.Text(c, "未通过检测").FontSize(fontCaption).TextColor(th.Warning).SingleLine()
				}
			})
			ui.Text(c, a.sourceMetaLine(m)).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
			if m.Level == config.LevelNo {
				reason := m.Reason
				if reason == "" {
					reason = "本版不支持"
				}
				ui.Text(c, reason).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
			} else {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					a.healthChip(c, th, a.stats[key])
					ui.Text(c, a.statLine(key)).FontSize(fontCaption).TextColor(statColor(th, a.stats[key])).SingleLine()
				})
			}
		})
		if ui.Button(c.Key("probe:"+key), "测速").Disabled(isDisabled || m.Level == config.LevelNo).Clicked() {
			a.probeSource(key)
		}
		if !a.srcBatch {
			if isDisabled {
				if ui.Button(c.Key("enable:"+key), "启用").Clicked() {
					a.enableSites([]string{key})
					stop = true
				}
			} else if ui.Button(c.Key("disable:"+key), "停用").Clicked() {
				a.disableSites([]string{key})
				stop = true
			}
			if ui.Button(c.Key("drop-panel:"+key), "移除").Clicked() {
				a.removeSites([]string{key})
				stop = true
			}
		}
	})
	return stop
}

func (a *App) healthChip(c *ui.Context, th *ui.Theme, st sourceStat) {
	tier := healthTier(st)
	col := statColor(th, st)
	ui.Column(c).Padding(2, 6).Radius(4).Background(col.Alpha(0.14)).Children(func() {
		ui.Text(c, tier).FontSize(fontCaption).TextColor(col).SingleLine()
	})
}

func (a *App) sourceMetaLine(m source.Meta) string {
	parts := []string{hostOf(m.API), config.KindLabel(m.Kind)}
	if g := a.groupName(m.API, m.Key, ""); g != "" {
		parts = append(parts, g)
	}
	if m.Level != config.LevelYes {
		parts = append(parts, m.Level.String())
	}
	return strings.Join(parts, "  ·  ")
}

func (a *App) selectedSourceKeys() []string {
	var out []string
	for k, on := range a.srcSel {
		if on {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// activeSiteKeys maps every catalog source key that actually passed the probe
// (or came from cache) to true, so the view can tell "working" from disabled
// or failed sources.
func (a *App) activeSiteKeys() map[string]bool {
	out := map[string]bool{}
	if a.catalog != nil {
		for _, s := range a.catalog.Sites {
			out[s.Meta().Key] = true
		}
	}
	return out
}

// visibleSources lists every source that is not removed or blacklisted.
func (a *App) visibleSources() []source.Source {
	out := make([]source.Source, 0, len(a.allSites))
	for _, s := range a.allSites {
		m := s.Meta()
		if a.hidden[sourceHideID(s)] || a.blacklisted(m.Name) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (a *App) passesHealth(key string) bool {
	st := a.stats[key]
	switch a.srcHealth {
	case "仅可用":
		return st.Delay > 0
	case "仅失败":
		return st.Err != "" || st.Delay < 0
	case "仅未测":
		return st.Delay == 0 && st.Err == ""
	case "较慢":
		return st.Delay >= 1000
	default:
		return true
	}
}

func matchesSource(m source.Meta, q string) bool {
	return strings.Contains(strings.ToLower(m.Name), q) ||
		strings.Contains(strings.ToLower(m.Key), q) ||
		strings.Contains(strings.ToLower(m.API), q)
}

func (a *App) sortedSources() []source.Source {
	out := a.visibleSources()
	sort.SliceStable(out, func(i, j int) bool {
		mi, mj := out[i].Meta(), out[j].Meta()
		sti, stj := a.stats[mi.Key], a.stats[mj.Key]
		switch a.srcSort {
		case "延迟":
			return delaySortKey(sti) < delaySortKey(stj)
		case "类型":
			if ki, kj := config.KindLabel(mi.Kind), config.KindLabel(mj.Kind); ki != kj {
				return ki < kj
			}
		case "健康度":
			if ri, rj := healthRank(sti), healthRank(stj); ri != rj {
				return ri < rj
			}
		case "订阅":
			if gi, gj := a.groupName(mi.API, mi.Key, ""), a.groupName(mj.API, mj.Key, ""); gi != gj {
				return gi < gj
			}
		}
		return mi.Name < mj.Name
	})
	return out
}

// delaySortKey orders sources by latency with successes first, untested next,
// and failures last.
func delaySortKey(st sourceStat) int {
	switch {
	case st.Err != "" || st.Delay < 0:
		return 1 << 30
	case st.Delay == 0:
		return 1 << 29
	default:
		return st.Delay
	}
}

// filteredSites applies the health filter and the search box to the sorted,
// visible source list.
func (a *App) filteredSites() []source.Source {
	srcs := a.sortedSources()
	q := strings.ToLower(strings.TrimSpace(a.srcFilter))
	out := srcs[:0]
	for _, s := range srcs {
		m := s.Meta()
		if !a.passesHealth(m.Key) {
			continue
		}
		if q != "" && !matchesSource(m, q) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// sourceSummary counts the visible sources by health and returns the average
// latency over the ones that answered.
func (a *App) sourceSummary() (total, ok, fail, untested, avg int) {
	for _, s := range a.visibleSources() {
		st := a.stats[s.Meta().Key]
		total++
		switch {
		case st.Err != "" || st.Delay < 0:
			fail++
		case st.Delay > 0:
			ok++
			avg += st.Delay
		default:
			untested++
		}
	}
	if ok > 0 {
		avg /= ok
	}
	return
}

type sourceGroup struct {
	name  string
	sites []source.Source
}

func (a *App) groupSources(sites []source.Source) []sourceGroup {
	if a.srcView == "列表" {
		if len(sites) == 0 {
			return nil
		}
		return []sourceGroup{{sites: sites}}
	}
	var groups []sourceGroup
	idx := map[string]int{}
	for _, s := range sites {
		name := a.sourceGroupKey(s)
		if i, ok := idx[name]; ok {
			groups[i].sites = append(groups[i].sites, s)
		} else {
			idx[name] = len(groups)
			groups = append(groups, sourceGroup{name: name, sites: []source.Source{s}})
		}
	}
	return groups
}

func (a *App) sourceGroupKey(s source.Source) string {
	m := s.Meta()
	switch a.srcView {
	case "按订阅":
		if g := a.groupName(m.API, m.Key, ""); g != "" {
			return g
		}
		return "其他"
	case "按类型":
		return config.KindLabel(m.Kind)
	case "按健康度":
		return healthTier(a.stats[m.Key])
	default:
		return ""
	}
}

func statColor(th *ui.Theme, st sourceStat) ui.Color {
	switch {
	case st.Err != "" || st.Delay < 0:
		return th.Danger
	case st.Delay > 0 && st.Delay < 200:
		return th.Success
	case st.Delay > 0 && st.Delay < 800:
		return th.Warning
	default:
		return th.TextMuted
	}
}

func hostOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}

func offerGroup(n int) string {
	if label := clarityLabel(n); label != "" {
		return label
	}
	return "其他"
}

func (a *App) viewOffers(c *ui.Context, th *ui.Theme) {
	type group struct {
		name string
		idx  []int
	}
	var groups []group
	for i, o := range a.offers {
		name := offerGroup(o.Clarity)
		if len(groups) == 0 || groups[len(groups)-1].name != name {
			groups = append(groups, group{name: name})
		}
		last := &groups[len(groups)-1]
		last.idx = append(last.idx, i)
	}
	checking, measured := 0, 0
	for _, o := range a.offers {
		if o.Checking {
			checking++
		}
		if o.Width > 0 && o.Height > 0 {
			measured++
		}
	}
	ui.Column(c).Gap(14).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "片源").FontSize(fontTitle).Bold()
			switch {
			case checking > 0:
				done := len(a.offers) - checking
				ui.Spinner(c)
				ui.Text(c, fmt.Sprintf("正在检测各片源的实际清晰度 %d/%d，完成后自动排序并选出最佳片源", done, len(a.offers))).
					FontSize(fontCaption).TextColor(th.TextMuted).Shrink(1).SingleLine()
				ui.Box(c).Width(96).Height(4).Radius(2).Background(th.SurfaceHover).Children(func() {
					ui.Box(c).Height(4).WidthPercent(float32(done) * 100 / float32(len(a.offers))).Radius(2).Background(th.Accent)
				})
			case measured > 0:
				ui.Text(c, fmt.Sprintf("已按实测清晰度排序（实测 %d/%d 个）", measured, len(a.offers))).
					FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
			}
		})
		for _, g := range groups {
			g := g
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, g.name).FontSize(fontCaption).TextColor(th.TextMuted)
				ui.Row(c).Gap(8).Wrap().Children(func() {
					for _, i := range g.idx {
						i := i
						o := a.offers[i]
						label := o.SiteName
						if o.Width > 0 && o.Height > 0 {
							label += fmt.Sprintf(" · %d×%d", o.Width, o.Height)
						}
						if st := a.stats[o.SiteKey]; st.Delay > 0 {
							label += fmt.Sprintf(" · %d ms", st.Delay)
						}
						if o.Checking {
							label += " · 检测中…"
						} else if o.Err != "" && o.Width == 0 {
							label += " · 读取失败"
						}
						a.choice(c, th, "offer:"+o.SiteKey, label, i == a.offerPick, func() { a.pickOffer(i) })
					}
				})
			})
		}
	})
}
