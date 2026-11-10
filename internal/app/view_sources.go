package app

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *App) viewSources(c *ui.Context, th *ui.Theme) {
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Text(c, "测连通、延迟和服务器所在地。延迟低、清晰度高的源，在影片页里排在前面。").TextColor(th.TextMuted).Shrink(1).Wrap()
				ui.Column(c).Grow(1)
				if ui.Button(c, "全部测速").Clicked() {
					a.probeAllSources()
				}
			})
			if a.catalog == nil || len(a.catalog.Sites) == 0 {
				a.emptyState(c, th, "还没有片源", "添加一份配置后，能列出内容的源会出现在这里。")
				return
			}
			for _, s := range a.catalog.Sites {
				m := s.Meta()
				key := m.Key
				ui.Row(c.Key("src:"+key)).FillWidth().Padding(12, 14).Radius(8).Background(th.Surface).AlignItems(ui.Center).Gap(12).Children(func() {
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, m.Name).FontSize(16).Bold().SingleLine()
						ui.Text(c, hostOf(m.API)).FontSize(12).TextColor(th.TextMuted).SingleLine()
						ui.Text(c, a.statLine(key)).FontSize(13).TextColor(th.TextMuted).SingleLine()
					})
					if ui.Button(c.Key("probe:"+key), "测速").Clicked() {
						a.probeSource(key)
					}
					if ui.Button(c.Key("drop-panel:"+key), "移除").Clicked() {
						a.dropSite(key)
					}
				})
			}
		})
	})
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
		ui.Text(c, "片源").FontSize(16).Bold()
		for _, g := range groups {
			g := g
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, g.name).FontSize(13).TextColor(th.TextMuted)
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
