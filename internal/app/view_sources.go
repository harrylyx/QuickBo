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

// The source list's sort, filter and grouping. Each option says what it
// does, as the menus show nothing else ("名称" alone did not say it sorts).
const (
	srcSortName    = "按名称排序"
	srcSortDelay   = "按延迟排序"
	srcSortKind    = "按类型排序"
	srcSortHealth  = "按健康度排序"
	srcSortSub     = "按订阅排序"
	srcShowAll     = "显示全部"
	srcShowOK      = "只看可用"
	srcShowFailed  = "只看失败"
	srcShowUntest  = "只看未测速"
	srcShowSlow    = "只看较慢（≥1 秒）"
	srcGroupNone   = "不分组"
	srcGroupSub    = "按订阅分组"
	srcGroupKind   = "按类型分组"
	srcGroupHealth = "按健康度分组"
)

var (
	srcSortOptions   = []string{srcSortName, srcSortDelay, srcSortKind, srcSortHealth, srcSortSub}
	srcHealthOptions = []string{srcShowAll, srcShowOK, srcShowFailed, srcShowUntest, srcShowSlow}
	srcViewOptions   = []string{srcGroupNone, srcGroupSub, srcGroupKind, srcGroupHealth}
)

func (a *App) viewSources(c *ui.Context, th *ui.Theme) {
	ui.Column(c).Grow(1).Gap(16).Children(func() {
		sortSel := a.srcSort
		if sortSel == "" {
			sortSel = srcSortName
		}
		healthSel := a.srcHealth
		if healthSel == "" {
			healthSel = srcShowAll
		}
		viewSel := a.srcView
		if viewSel == "" {
			viewSel = srcGroupNone
		}
		batchLabel := "批量管理"
		if a.srcBatch {
			batchLabel = "完成"
		}
		ui.Text(c, "测连通和延迟，停用后可再启用。").TextColor(th.TextMuted).Wrap()
		ui.Row(c).AlignItems(ui.Center).Gap(10).Wrap().Children(func() {
			ui.SearchField(c, &a.srcFilter).Placeholder("搜索片源名称或地址").Width(200).Label("搜索片源")
			ui.Button(c, "筛选").Label("筛选").Tooltip("排序、按状态筛选、分组").PopoverMenuButton(func(m *ui.Menu) {
				m.Item("排序").Disabled(true)
				for _, opt := range srcSortOptions {
					if m.Item(opt).Radio(sortSel == opt).Chosen() {
						a.srcSort = opt
					}
				}
				m.Separator()
				m.Item("状态").Disabled(true)
				for _, opt := range srcHealthOptions {
					if m.Item(opt).Radio(healthSel == opt).Chosen() {
						a.srcHealth = opt
					}
				}
				m.Separator()
				m.Item("分组").Disabled(true)
				for _, opt := range srcViewOptions {
					if m.Item(opt).Radio(viewSel == opt).Chosen() {
						a.srcView = opt
					}
				}
			})
			ui.Column(c).Grow(1)
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
		if a.catalog == nil || len(a.allSites) == 0 {
			a.emptyState(c, th, "还没有片源", "添加一份配置后，能列出内容的源会出现在这里。",
				stateAction{label: "添加订阅", primary: true, click: a.openSubs})
			return
		}

		list := a.sourceRows()
		total, ok, fail, untested, avg := list.total, list.ok, list.fail, list.untested, list.avg
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

		if len(list.rows) == 0 {
			ui.Text(c, "没有符合当前筛选的片源").TextColor(th.TextMuted)
			return
		}

		if a.srcBatch {
			stop := a.sourceBatchBar(c, th, list.sites)
			if stop {
				return
			}
		}

		active := a.activeSiteKeys()
		rows := list.rows
		a.srcList.Key = func(i int) any {
			if i < len(rows) {
				return rows[i].id
			}
			return i
		}
		a.srcList.Header = func(i int) bool {
			return i < len(rows) && rows[i].head != ""
		}
		stop := false
		// ui.List is virtual: it builds only the rows in view, so a
		// subscription of any size scrolls as smoothly as a small one. The
		// sorted, filtered rows themselves come from a.sourceRows, cached
		// until their inputs change.
		ui.List(c, &a.srcList, len(rows), func(i int) {
			if stop || i >= len(rows) {
				return
			}
			r := rows[i]
			if r.head != "" {
				ui.Box(c.Key(r.id)).FillWidth().Background(th.Background).Padding(6, 14).Children(func() {
					ui.Text(c, r.head).FontSize(fontCaption).Bold().TextColor(th.TextMuted).SingleLine()
				})
				return
			}
			if a.sourceRow(c, th, r.site, active) {
				stop = true
			}
		}).Gap(8).Grow(1)
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
	case srcShowOK:
		return st.Delay > 0
	case srcShowFailed:
		return st.Err != "" || st.Delay < 0
	case srcShowUntest:
		return st.Delay == 0 && st.Err == ""
	case srcShowSlow:
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

// sortSources orders the visible sources for the management list. It sorts
// the slice in place; the caller passes a slice it owns.
func (a *App) sortSources(out []source.Source) {
	sort.SliceStable(out, func(i, j int) bool {
		mi, mj := out[i].Meta(), out[j].Meta()
		sti, stj := a.stats[mi.Key], a.stats[mj.Key]
		switch a.srcSort {
		case srcSortDelay:
			return delaySortKey(sti) < delaySortKey(stj)
		case srcSortKind:
			if ki, kj := config.KindLabel(mi.Kind), config.KindLabel(mj.Kind); ki != kj {
				return ki < kj
			}
		case srcSortHealth:
			if ri, rj := healthRank(sti), healthRank(stj); ri != rj {
				return ri < rj
			}
		case srcSortSub:
			if gi, gj := a.groupName(mi.API, mi.Key, ""), a.groupName(mj.API, mj.Key, ""); gi != gj {
				return gi < gj
			}
		}
		return mi.Name < mj.Name
	})
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

// srcListKey captures every input the management list derives from, so the
// sort, the filters and the grouping run once per change, not once per frame.
type srcListKey struct {
	filter, sortKey, health, view string
	gen                           int
	catalog                       *source.Catalog
	sites, hidden, disabled       int
	blacklist, stats              int
	digest                        uint64
}

// srcActiveKey identifies the catalog state behind the active-source map.
type srcActiveKey struct {
	catalog *source.Catalog
	n       int
	gen     int
}

// srcRow is one row of the management list: a group header (head set) or a
// source (site set).
type srcRow struct {
	id   string // stable identity for the virtual list
	head string
	site source.Source
}

// srcRows is the management list computed for one set of inputs: the
// sorted, filtered sources (sites), the flattened rows with group headers
// (rows), and the health summary over every visible source.
type srcRows struct {
	key                            srcListKey
	sites                          []source.Source
	rows                           []srcRow
	total, ok, fail, untested, avg int
}

// sourceRows returns the management list, cached until any input it derives
// from changes. A frame that only scrolls or hovers then costs nothing but
// the rows in view.
func (a *App) sourceRows() *srcRows {
	k := a.sourceListKey()
	if a.srcCache != nil && a.srcCache.key == k {
		return a.srcCache
	}
	r := &srcRows{key: k}
	vis := a.visibleSources()
	for _, s := range vis {
		st := a.stats[s.Meta().Key]
		r.total++
		switch {
		case st.Err != "" || st.Delay < 0:
			r.fail++
		case st.Delay > 0:
			r.ok++
			r.avg += st.Delay
		default:
			r.untested++
		}
	}
	if r.ok > 0 {
		r.avg /= r.ok
	}
	a.sortSources(vis)
	r.sites = vis
	q := strings.ToLower(strings.TrimSpace(a.srcFilter))
	if (a.srcHealth != "" && a.srcHealth != srcShowAll) || q != "" {
		kept := make([]source.Source, 0, len(vis))
		for _, s := range vis {
			m := s.Meta()
			if !a.passesHealth(m.Key) {
				continue
			}
			if q != "" && !matchesSource(m, q) {
				continue
			}
			kept = append(kept, s)
		}
		r.sites = kept
	}
	for _, g := range a.groupSources(r.sites) {
		if g.name != "" {
			r.rows = append(r.rows, srcRow{id: "srch:" + g.name, head: g.name})
		}
		for _, s := range g.sites {
			r.rows = append(r.rows, srcRow{id: "src:" + s.Meta().Key, site: s})
		}
	}
	a.srcCache = r
	return r
}

// sourceListKey snapshots the inputs of the management list. The stats
// digest (order-independent) catches probe results, which rewrite entries
// without changing the map's length; the gen counter catches a reload that
// leaves every length as it was.
func (a *App) sourceListKey() srcListKey {
	k := srcListKey{
		filter: a.srcFilter, sortKey: a.srcSort, health: a.srcHealth, view: a.srcView,
		gen:       a.gen,
		catalog:   a.catalog,
		sites:     len(a.allSites),
		hidden:    len(a.hidden),
		disabled:  len(a.disabled),
		blacklist: len(a.blacklist),
		stats:     len(a.stats),
	}
	for key, st := range a.stats {
		k.digest += uint64(st.Delay)<<20 ^ uint64(len(st.Err)) ^ uint64(fnv32(key))*0x9e3779b97f4a7c15
	}
	return k
}

func fnv32(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// activeSiteKeys maps every catalog source key that actually passed the probe
// (or came from cache) to true, so the view can tell "working" from disabled
// or failed sources. Cached by catalog identity and size: removeSites and
// disableSites filter a.catalog.Sites in place without changing the pointer.
func (a *App) activeSiteKeys() map[string]bool {
	k := srcActiveKey{catalog: a.catalog, gen: a.gen}
	if a.catalog != nil {
		k.n = len(a.catalog.Sites)
	}
	if a.srcActive != nil && a.srcActiveKey == k {
		return a.srcActive
	}
	out := map[string]bool{}
	if a.catalog != nil {
		for _, s := range a.catalog.Sites {
			out[s.Meta().Key] = true
		}
	}
	a.srcActive, a.srcActiveKey = out, k
	return out
}

type sourceGroup struct {
	name  string
	sites []source.Source
}

func (a *App) groupSources(sites []source.Source) []sourceGroup {
	if a.srcView == srcGroupNone {
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
	case srcGroupSub:
		if g := a.groupName(m.API, m.Key, ""); g != "" {
			return g
		}
		return "其他"
	case srcGroupKind:
		return config.KindLabel(m.Kind)
	case srcGroupHealth:
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

func offerGroup(o playOffer) string {
	if o.Unplayable {
		return "无法播放"
	}
	if label := clarityLabel(o.Clarity); label != "" {
		return label
	}
	// Playable, but its picture size could not be read (single-bitrate HLS
	// segment lists and the like): say so rather than a vague "其他".
	return "清晰度未知"
}

func (a *App) viewOffers(c *ui.Context, th *ui.Theme) {
	type group struct {
		name string
		idx  []int
	}
	checking, measured, dead := 0, 0, 0
	for _, o := range a.offers {
		if o.Checking {
			checking++
		}
		if o.Width > 0 && o.Height > 0 {
			measured++
		}
		if o.Unplayable {
			dead++
		}
	}
	var groups []group
	for i, o := range a.offers {
		// Offers are re-sorted (unplayable last) only when the check ends;
		// until then an unplayable one greys out in place, under its
		// clarity group, so the groups do not split up.
		name := offerGroup(o)
		if checking > 0 {
			name = offerGroup(playOffer{Clarity: o.Clarity})
		}
		if len(groups) == 0 || groups[len(groups)-1].name != name {
			groups = append(groups, group{name: name})
		}
		last := &groups[len(groups)-1]
		last.idx = append(last.idx, i)
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
			case measured > 0 || dead > 0:
				note := fmt.Sprintf("已按实测清晰度排序（实测 %d/%d 个）", measured, len(a.offers))
				if dead > 0 {
					note += fmt.Sprintf("，%d 个无法播放", dead)
				}
				ui.Text(c, note).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
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
						if o.Unplayable {
							a.deadChoice(c, th, "offer:"+o.SiteKey, o.SiteName, o.Reason)
							continue
						}
						if o.Checking {
							label += " · 检测中…"
						}
						a.choice(c, th, "offer:"+o.SiteKey, label, i == a.offerPick, func() { a.pickOffer(i) })
					}
				})
			})
		}
	})
}
