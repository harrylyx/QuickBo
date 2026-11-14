package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/live"
	"github.com/harrylyx/QuickBo/internal/source"
)

// liveBuckets is the directory order: national channels, provincial
// satellite channels, then everything else.
var liveBuckets = []string{"央视", "地方卫视", "其他"}

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
		return lessLiveName(out[i].Name, out[j].Name)
	})
	return out
}

// lessLiveName orders channel names so a number is read as a number:
// CCTV1, CCTV2, CCTV10.
func lessLiveName(a, b string) bool {
	ar, br := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ar) && j < len(br) {
		if unicode.IsDigit(ar[i]) && unicode.IsDigit(br[j]) {
			ia, ja := i, j
			for i < len(ar) && unicode.IsDigit(ar[i]) {
				i++
			}
			for j < len(br) && unicode.IsDigit(br[j]) {
				j++
			}
			if c := cmpLiveDigits(ar[ia:i], br[ja:j]); c != 0 {
				return c < 0
			}
			continue
		}
		if ar[i] != br[j] {
			return ar[i] < br[j]
		}
		i++
		j++
	}
	return len(ar) < len(br)
}

// cmpLiveDigits compares two digit runs as numbers, without overflowing on
// a long run. Leading zeros do not change the value.
func cmpLiveDigits(a, b []rune) int {
	for len(a) > 1 && a[0] == '0' {
		a = a[1:]
	}
	for len(b) > 1 && b[0] == '0' {
		b = b[1:]
	}
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(string(a), string(b))
}

// liveBucket is the directory section a channel name belongs to.
func liveBucket(name string) string {
	if strings.Contains(name, "央视") || strings.Contains(strings.ToUpper(name), "CCTV") {
		return "央视"
	}
	if strings.Contains(name, "卫视") {
		return "地方卫视"
	}
	return "其他"
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

// playLiveName starts the first address of a channel. The other addresses
// stay in the player's line menu.
func (a *App) playLiveName(name string) {
	g, ok := a.aggByName(name)
	if !ok || len(g.Lines) == 0 {
		return
	}
	a.liveAggName = name
	a.liveAggLines = linesAsChannels(g.Lines)
	a.showDrawer = false
	a.playLiveLine(g.Lines[0])
}

func (a *App) playLiveLine(line liveLine) {
	g, ok := a.aggByName(line.Name)
	if ok {
		a.liveAggName = line.Name
		a.liveAggLines = linesAsChannels(g.Lines)
		for i, ln := range g.Lines {
			if ln.URL == line.URL {
				a.liveLineAt = i
				break
			}
		}
	}
	a.liveTries = 0
	a.playErr = ""
	a.liveWatchAt = time.Time{}
	a.startChannel(live.Channel{Name: line.Name, URL: line.URL, Group: line.Group, Logo: line.Logo})
}

// noteLiveSwitch tells the viewer the player moved to another address on
// its own. A hand-picked line does not come through here.
func (a *App) noteLiveSwitch(ch live.Channel) {
	label := a.liveLineLabelAt(ch.URL)
	if label == "" {
		label = "下一条线路"
	}
	a.notify("播放超时，已换到 " + label)
	a.px.noticeUntil = time.Now().Add(2500 * time.Millisecond)
	if g, ok := a.aggByName(a.liveAggName); ok {
		for i, ln := range g.Lines {
			if ln.URL == ch.URL {
				a.liveLineAt = i
				break
			}
		}
	}
}

func (a *App) liveLineLabelAt(url string) string {
	g, ok := a.aggByName(a.liveAggName)
	if !ok {
		return ""
	}
	for _, ln := range g.Lines {
		if ln.URL == url {
			return liveLineLabel(ln)
		}
	}
	return ""
}

// watchLivePlayback moves to the next address when the one that just
// opened never becomes ready, or the player reports it failed.
func (a *App) watchLivePlayback() {
	if a.screen != screenLiveAgg || !a.fromLive || a.playLoading || a.player == nil {
		return
	}
	if len(a.liveAggLines) < 2 {
		return
	}
	failed := a.player.Err() != nil
	if !failed && a.player.Ready() {
		a.liveWatchAt = time.Time{}
		return
	}
	if a.liveWatchAt.IsZero() {
		a.liveWatchAt = time.Now()
	}
	if !failed && time.Since(a.liveWatchAt) < livePlayTimeout {
		return
	}
	ch := a.liveAggLines[0]
	if a.liveLineAt >= 0 && a.liveLineAt < len(a.liveAggLines) {
		ch = a.liveAggLines[a.liveLineAt]
	}
	if a.liveHealth == nil {
		a.liveHealth = map[string]liveHealthState{}
	}
	a.liveHealth["live:"+strings.TrimSpace(ch.URL)] = liveHealthState{ok: false, at: time.Now()}
	a.closePlayer()
	a.switchChannel(ch)
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

func (a *App) aggBuckets() [3][]liveAgg {
	var out [3][]liveAgg
	for _, g := range a.visibleAgg() {
		switch liveBucket(g.Name) {
		case "央视":
			out[0] = append(out[0], g)
		case "地方卫视":
			out[1] = append(out[1], g)
		default:
			out[2] = append(out[2], g)
		}
	}
	return out
}

func (a *App) viewLiveAgg(c *ui.Context, th *ui.Theme) {
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
	playing := (a.player != nil && a.fromLive) || (a.playLoading && a.fromLive)
	buckets := a.aggBuckets()
	ui.Row(c).Grow(1).Gap(12).AlignItems(ui.Stretch).Children(func() {
		col := ui.Column(c)
		if playing {
			col = col.Width(300)
		} else {
			col = col.Grow(1)
		}
		col.Gap(12).Children(func() {
			if a.liveAggMiss > 0 {
				ui.Text(c, fmt.Sprintf("%d 份列表没读到", a.liveAggMiss)).FontSize(fontCaption).TextColor(th.TextMuted)
			}
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(18).Children(func() {
					for i, name := range liveBuckets {
						items := buckets[i]
						if len(items) == 0 {
							continue
						}
						name, items := name, items
						ui.Column(c).Gap(8).Children(func() {
							ui.Text(c, name).FontSize(fontTitle).Bold()
							for _, g := range items {
								g := g
								onAir := a.fromLive && a.liveAggName == g.Name
								row := ui.Row(c).FillWidth().Padding(8, 12).Radius(8).Gap(12).AlignItems(ui.Center).
									OnClick(func() { a.playLiveName(g.Name) }).Cursor(ui.CursorPointer)
								bg := ui.Transparent
								switch {
								case onAir:
									bg = th.Accent.Alpha(0.16)
								case row.Hovered():
									bg = th.SurfaceHover
								}
								row.Background(bg).Children(func() {
									ui.Text(c, g.Name).Grow(1).SingleLine()
								})
							}
						})
					}
				})
			})
		})
		if playing {
			ui.Column(c).Grow(1).Children(func() {
				a.paintStage(c, th, true)
			})
		}
	})
}

// liveLineDrawer lists the addresses of the channel that is on, the same
// way a show lists its lines.
func (a *App) liveLineDrawer(c *ui.Context, th *ui.Theme) {
	g, ok := a.aggByName(a.liveAggName)
	ui.Column(c).Absolute().Top(0).Right(0).Bottom(0).Width(300).Padding(20, 18).Gap(12).
		Background(stageInk(false)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "线路").FontSize(fontTitle).Bold().TextColor(th.AccentText).Grow(1)
			a.iconBtn(c, "close-eps", iconBack, "关闭", 44, 22, th.AccentText, func() { a.showDrawer = false })
		})
		if !ok || len(g.Lines) == 0 {
			ui.Text(c, "没有其他线路").TextColor(th.AccentText.Alpha(0.7))
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				for i, ln := range g.Lines {
					i, ln := i, ln
					a.stageChoice(c, th, "live-line:"+ln.URL, liveLineLabel(ln), i == a.liveLineAt, func() {
						a.showDrawer = false
						a.playLiveLine(ln)
					})
				}
			})
		})
	})
}
