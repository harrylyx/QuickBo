package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/live"
	"github.com/harrylyx/QuickBo/internal/source"
)

// liveLine is one address of an aggregated channel name.
type liveLine struct {
	Source string
	Group  string
	URL    string
	Logo   string
	Name   string
}

// liveAgg is one channel name gathered from every loaded playlist.
type liveAgg struct {
	Name    string
	Lines   []liveLine
	Sources int
}

// liveSnap is one playlist's parsed channels, used to build the directory.
type liveSnap struct {
	Name string
	URL  string
	Chs  []live.Channel
}

// aggregateLives groups channels by trimmed name. An identical address is
// kept once. CCTV-1 and CCTV1 stay separate. Source count is how many
// playlists contributed a kept address.
func aggregateLives(lists []liveSnap) []liveAgg {
	type acc struct {
		lines   []liveLine
		sources map[string]bool
		seenURL map[string]bool
	}
	order := []string{}
	by := map[string]*acc{}
	for _, list := range lists {
		id := strings.TrimSpace(list.URL)
		if id == "" {
			id = list.Name
		}
		for _, ch := range list.Chs {
			name := strings.TrimSpace(ch.Name)
			raw := strings.TrimSpace(ch.URL)
			if name == "" || raw == "" {
				continue
			}
			a := by[name]
			if a == nil {
				a = &acc{sources: map[string]bool{}, seenURL: map[string]bool{}}
				by[name] = a
				order = append(order, name)
			}
			if a.seenURL[raw] {
				continue
			}
			a.seenURL[raw] = true
			a.sources[id] = true
			a.lines = append(a.lines, liveLine{
				Source: list.Name, Group: strings.TrimSpace(ch.Group), URL: raw, Logo: ch.Logo, Name: name,
			})
		}
	}
	out := make([]liveAgg, 0, len(order))
	for _, name := range order {
		a := by[name]
		out = append(out, liveAgg{Name: name, Lines: a.lines, Sources: len(a.sources)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sources != out[j].Sources {
			return out[i].Sources > out[j].Sources
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// nextAggChannel is the next address of the same name. A different name in
// the slice is skipped, so a dead line never jumps to another channel.
func nextAggChannel(chs []live.Channel, ch live.Channel) (live.Channel, bool) {
	name := strings.TrimSpace(ch.Name)
	idx := -1
	for i, c := range chs {
		if c.URL == ch.URL && strings.TrimSpace(c.Name) == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return live.Channel{}, false
	}
	for j := 1; j < len(chs); j++ {
		n := chs[(idx+j)%len(chs)]
		if strings.TrimSpace(n.Name) == name && n.URL != ch.URL {
			return n, true
		}
	}
	return live.Channel{}, false
}

func linesAsChannels(lines []liveLine) []live.Channel {
	out := make([]live.Channel, len(lines))
	for i, ln := range lines {
		out[i] = live.Channel{Name: ln.Name, URL: ln.URL, Group: ln.Group, Logo: ln.Logo}
	}
	return out
}

// openLiveAgg shows the aggregated directory and loads any playlist that
// has not been parsed yet. A list that fails is skipped.
func (a *App) openLiveAgg() {
	a.query = ""
	a.committed = ""
	a.screen = screenLiveAgg
	a.liveAggName = ""
	a.liveAggLines = nil
	a.livePlayGen++
	a.closePlayer()
	a.fromLive = false
	a.playErr = ""
	a.playLoading = false
	if a.catalog == nil || len(a.catalog.Lives) == 0 {
		a.liveAggs = nil
		a.liveAggLoading = false
		a.liveAggMiss = 0
		return
	}
	a.liveAggs = nil
	a.liveAggMiss = 0
	a.liveAggLoading = true
	a.liveAggToken++
	token := a.liveAggToken
	lives := append([]*source.LiveEntry(nil), a.catalog.Lives...)
	go func() {
		out := make([]liveSnap, len(lives))
		miss := make([]bool, len(lives))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 4)
		for i, lv := range lives {
			if lv == nil {
				miss[i] = true
				continue
			}
			wg.Add(1)
			go func(i int, lv *source.LiveEntry) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
				defer cancel()
				chs, err := lv.Load(ctx)
				if err != nil || len(chs) == 0 {
					miss[i] = true
					return
				}
				out[i] = liveSnap{Name: lv.Name, URL: lv.URL, Chs: chs}
			}(i, lv)
		}
		wg.Wait()
		var lists []liveSnap
		skipped := 0
		for i := range lives {
			if miss[i] {
				skipped++
				continue
			}
			lists = append(lists, out[i])
		}
		aggs := aggregateLives(lists)
		a.post(func() {
			if a.liveAggToken != token {
				return
			}
			a.liveAggs = aggs
			a.liveAggMiss = skipped
			a.liveAggLoading = false
		})
	}()
}

func (a *App) visibleAgg() []liveAgg {
	q := strings.ToLower(strings.TrimSpace(a.query))
	if q == "" {
		return a.liveAggs
	}
	var out []liveAgg
	for _, g := range a.liveAggs {
		if strings.Contains(strings.ToLower(g.Name), q) {
			out = append(out, g)
		}
	}
	return out
}

func (a *App) aggByName(name string) (liveAgg, bool) {
	for _, g := range a.liveAggs {
		if g.Name == name {
			return g, true
		}
	}
	return liveAgg{}, false
}

// openLiveName replaces the directory with that name's addresses.
func (a *App) openLiveName(name string) {
	g, ok := a.aggByName(name)
	if !ok {
		return
	}
	a.liveAggName = name
	a.liveAggLines = linesAsChannels(g.Lines)
	a.liveTries = 0
	a.playErr = ""
	a.playLoading = false
	a.livePlayGen++
	a.closePlayer()
	a.fromLive = false
}

// closeLiveName stops playback and returns to the channel directory.
func (a *App) closeLiveName() {
	a.liveAggName = ""
	a.liveAggLines = nil
	a.liveTries = 0
	a.playErr = ""
	a.playLoading = false
	a.livePlayGen++
	a.closePlayer()
	a.fromLive = false
}

func (a *App) playLiveLine(line liveLine) {
	a.liveTries = 0
	a.playErr = ""
	a.startChannel(live.Channel{Name: line.Name, URL: line.URL, Group: line.Group, Logo: line.Logo})
}

func liveLineLabel(line liveLine) string {
	source := strings.TrimSpace(line.Source)
	if source == "" {
		source = "直播"
	}
	if line.Group == "" {
		return source
	}
	return source + "  ·  " + line.Group
}

// liveFullscreen reports a live picture that should cover the window.
func (a *App) liveFullscreen() bool {
	return (a.screen == screenLive || a.screen == screenLiveAgg) && a.fullscreen && a.fromLive && a.embedded()
}

func (a *App) viewLiveAgg(c *ui.Context, th *ui.Theme) {
	if a.liveAggName != "" {
		a.viewLiveLines(c, th)
		return
	}
	if a.liveAggLoading && len(a.liveAggs) == 0 {
		ui.Text(c, "正在读取直播列表。").TextColor(th.TextMuted).Wrap()
		return
	}
	shown := a.visibleAgg()
	if len(shown) == 0 {
		if strings.TrimSpace(a.query) != "" && len(a.liveAggs) > 0 {
			a.emptyState(c, th, "没有频道符合「"+strings.TrimSpace(a.query)+"」", "清空搜索框可以看到全部频道。",
				stateAction{label: "清空搜索", primary: true, click: func() {
					a.query = ""
					a.committed = ""
				}})
			return
		}
		body := "直播列表是空的。"
		if a.liveAggMiss > 0 {
			body = "直播列表没读到。"
		}
		a.emptyState(c, th, "没有频道", body)
		return
	}
	ui.Column(c).Grow(1).Gap(12).Children(func() {
		if a.liveAggMiss > 0 {
			ui.Text(c, fmt.Sprintf("%d 份列表没读到", a.liveAggMiss)).FontSize(fontCaption).TextColor(th.TextMuted)
		}
		ui.Text(c, fmt.Sprintf("%d 个频道", len(shown))).FontSize(fontCaption).TextColor(th.TextMuted)
		ui.List(c, &a.aggList, len(shown), func(i int) {
			g := shown[i]
			label := fmt.Sprintf("%s · %d 个源", g.Name, g.Sources)
			row := ui.Row(c).FillWidth().Padding(8, 12).Radius(8).Gap(12).AlignItems(ui.Center).
				OnClick(func() { a.openLiveName(g.Name) }).Cursor(ui.CursorPointer)
			bg := ui.Transparent
			if row.Hovered() {
				bg = th.SurfaceHover
			}
			row.Background(bg).Children(func() {
				ui.Text(c, label).Grow(1).SingleLine()
			})
		}).Grow(1)
	})
}

func (a *App) viewLiveLines(c *ui.Context, th *ui.Theme) {
	g, ok := a.aggByName(a.liveAggName)
	if !ok {
		a.closeLiveName()
		a.viewLiveAgg(c, th)
		return
	}
	ui.Column(c).Grow(1).Gap(12).Children(func() {
		if ui.Button(c, "返回").Clicked() {
			a.closeLiveName()
			return
		}
		if a.playErr != "" && a.player == nil && !a.playLoading {
			a.note(c, th, a.playErr, true)
		}
		playing := (a.player != nil && a.fromLive) || (a.playLoading && a.fromLive)
		ui.Row(c).Grow(1).Gap(12).AlignItems(ui.Stretch).Children(func() {
			col := ui.Column(c)
			if playing {
				col = col.Width(300)
			} else {
				col = col.Grow(1)
			}
			col.Children(func() {
				ui.Text(c, fmt.Sprintf("%d 条线路", len(g.Lines))).FontSize(fontCaption).TextColor(th.TextMuted)
				ui.List(c, &a.aggLineList, len(g.Lines), func(i int) {
					line := g.Lines[i]
					onAir := a.fromLive && a.playURL == line.URL
					row := ui.Row(c).FillWidth().Padding(8, 12).Radius(8).Gap(12).AlignItems(ui.Center).
						OnClick(func() { a.playLiveLine(line) }).Cursor(ui.CursorPointer)
					bg := ui.Transparent
					switch {
					case onAir:
						bg = th.Accent.Alpha(0.16)
					case row.Hovered():
						bg = th.SurfaceHover
					}
					row.Background(bg).Children(func() {
						ui.Text(c, liveLineLabel(line)).Grow(1).SingleLine()
					})
				}).Grow(1)
			})
			if playing {
				ui.Column(c).Grow(1).Children(func() {
					a.paintStage(c, th, true)
				})
			}
		})
	})
}
