package app

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/source"
)

func (a *App) viewSources(c *ui.Context, th *ui.Theme) {
	ui.Column(c).Grow(1).Gap(16).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, "测连通、延迟和服务器所在地。延迟低、清晰度高的源，在影片页里排在前面。").TextColor(th.TextMuted).Shrink(1).Wrap()
			ui.Column(c).Grow(1)
			ui.SearchField(c, &a.srcFilter).Placeholder("搜索片源").Width(180).Label("搜索片源")
			if ui.Button(c, "全部测速").Clicked() {
				a.probeAllSources()
			}
			batchLabel := "批量删除"
			if a.srcBatch {
				batchLabel = "完成"
			}
			if ui.Button(c, batchLabel).Clicked() {
				a.srcBatch = !a.srcBatch
				if !a.srcBatch {
					a.srcSel = nil
				}
			}
		})

		if a.catalog == nil || len(a.catalog.Sites) == 0 {
			a.emptyState(c, th, "还没有片源", "添加一份配置后，能列出内容的源会出现在这里。",
				stateAction{label: "添加订阅", primary: true, click: a.openSubs})
			return
		}

		sites := a.filteredSites()
		if len(sites) == 0 {
			ui.Text(c, "没有匹配「"+strings.TrimSpace(a.srcFilter)+"」的片源").TextColor(th.TextMuted)
			return
		}

		if a.srcBatch {
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
				ui.Column(c).Grow(1)
				ui.Text(c, fmt.Sprintf("已选 %d 个", selCount)).FontSize(fontCaption).TextColor(th.TextMuted)
				if ui.Button(c, "删除所选").Disabled(selCount == 0).Clicked() && selCount > 0 {
					var keys []string
					for k, on := range a.srcSel {
						if on {
							keys = append(keys, k)
						}
					}
					a.removeSites(keys)
					stop = true
				}
			})
			if stop {
				return
			}
		}

		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(16).Children(func() {
				for _, s := range sites {
					m := s.Meta()
					key := m.Key
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
						ui.Column(c).Grow(1).Gap(4).Children(func() {
							ui.Text(c, m.Name).FontSize(fontTitle).Bold().SingleLine()
							ui.Text(c, hostOf(m.API)).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
							ui.Text(c, a.statLine(key)).FontSize(fontCaption).TextColor(statColor(th, a.stats[key])).SingleLine()
						})
						if ui.Button(c.Key("probe:"+key), "测速").Clicked() {
							a.probeSource(key)
						}
						if !a.srcBatch {
							if ui.Button(c.Key("drop-panel:"+key), "移除").Clicked() {
								a.removeSites([]string{key})
							}
						}
					})
				}
			})
		})
	})
}

// filteredSites returns the loaded sources whose name, key or API host
// matches the search box.
func (a *App) filteredSites() []source.Source {
	q := strings.ToLower(strings.TrimSpace(a.srcFilter))
	if q == "" {
		return a.catalog.Sites
	}
	var out []source.Source
	for _, s := range a.catalog.Sites {
		m := s.Meta()
		if strings.Contains(strings.ToLower(m.Name), q) ||
			strings.Contains(strings.ToLower(m.Key), q) ||
			strings.Contains(strings.ToLower(m.API), q) {
			out = append(out, s)
		}
	}
	return out
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
	ui.Column(c).Gap(14).Children(func() {
		ui.Text(c, "片源").FontSize(fontTitle).Bold()
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
						a.choice(c, th, "offer:"+o.SiteKey, label, i == a.offerPick, func() { a.pickOffer(i) })
					}
				})
			})
		}
	})
}
