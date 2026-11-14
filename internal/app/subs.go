package app

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/source"
)

// subscription is one saved subscription address. Disabled entries are kept
// in settings so they can be turned back on without retyping the URL.
type subscription struct {
	Ref     string `json:"ref"`
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled"`
}

func shortRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if u, err := url.Parse(ref); err == nil && u.Host != "" {
		return u.Host
	}
	if base := filepath.Base(ref); base != "" && base != "." && base != "/" {
		return base
	}
	if ref == "" {
		return "未命名"
	}
	return ref
}

func cloneSubs(in []subscription) []subscription {
	out := make([]subscription, len(in))
	copy(out, in)
	return out
}

func normalizeRefs(refs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}

// comboKey is the cache identity for the current set of enabled subscriptions.
// With a single subscription it equals that ref, so existing caches survive.
func comboKey(refs []string) string { return strings.Join(refs, "\n") }

func (a *App) activeRefs() []string {
	out := make([]string, 0, len(a.subs))
	for _, s := range a.subs {
		if s.Enabled && strings.TrimSpace(s.Ref) != "" {
			out = append(out, strings.TrimSpace(s.Ref))
		}
	}
	return out
}

// primaryRef keeps the legacy single-Config field meaningful for settings
// written by older versions.
func (a *App) primaryRef() string {
	for _, s := range a.subs {
		if strings.TrimSpace(s.Ref) != "" {
			return s.Ref
		}
	}
	return a.cfgRef
}

func (a *App) upsertSub(ref string, enabled bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return
	}
	for i := range a.subs {
		if a.subs[i].Ref == ref {
			a.subs[i].Enabled = enabled
			return
		}
	}
	a.subs = append(a.subs, subscription{Ref: ref, Name: shortRef(ref), Enabled: enabled})
}

func (a *App) AddSubscription(ref string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return
	}
	a.upsertSub(ref, true)
	a.LoadSubs(a.activeRefs())
}

func (a *App) toggleSubscription(ref string) {
	for i := range a.subs {
		if a.subs[i].Ref == ref {
			a.subs[i].Enabled = !a.subs[i].Enabled
			a.reloadSubs()
			return
		}
	}
}

func (a *App) removeSubscription(ref string) {
	a.tombstone(ref)
	kept := a.subs[:0]
	for _, s := range a.subs {
		if s.Ref != ref {
			kept = append(kept, s)
		}
	}
	a.subs = kept
	a.reloadSubs()
}

// subLabel is the sidebar grouping label of a subscription: its display name,
// or a short form of the address when unnamed.
func subLabel(s subscription) string {
	if s.Name != "" {
		return s.Name
	}
	return shortRef(s.Ref)
}

// subSourceCounts counts how many sources and live playlists each
// subscription contributed to the current catalog, keyed by its label.
func (a *App) subSourceCounts() map[string]int {
	out := map[string]int{}
	if a.catalog == nil {
		return out
	}
	for _, s := range a.catalog.Sites {
		m := s.Meta()
		if n := a.groupName(m.API, m.Key, ""); n != "" {
			out[n]++
		}
	}
	for _, lv := range a.catalog.Lives {
		if n := a.groupName("", "", lv.URL); n != "" {
			out[n]++
		}
	}
	return out
}

// sortedSubs returns the subscription list in the order chosen by subSort:
// name (default), ref, enabled or source count.
func (a *App) sortedSubs() []subscription {
	subs := cloneSubs(a.subs)
	counts := a.subSourceCounts()
	sort.SliceStable(subs, func(i, j int) bool {
		ni, nj := subLabel(subs[i]), subLabel(subs[j])
		switch a.subSort {
		case "地址":
			return subs[i].Ref < subs[j].Ref
		case "启用状态":
			if subs[i].Enabled != subs[j].Enabled {
				return subs[i].Enabled
			}
		case "片源数量":
			if ci, cj := counts[ni], counts[nj]; ci != cj {
				return ci > cj
			}
		}
		return ni < nj
	})
	return subs
}

// subSortOptions are the labels of the subscription sort dropdown; the chosen
// label is stored in subSort.
var subSortOptions = []string{"名称", "地址", "启用状态", "片源数量"}

// filteredSubs returns the sorted subscriptions whose name or address matches
// the search box.
func (a *App) filteredSubs() []subscription {
	subs := a.sortedSubs()
	q := strings.ToLower(strings.TrimSpace(a.subFilter))
	if q == "" {
		return subs
	}
	out := subs[:0]
	for _, s := range subs {
		if strings.Contains(strings.ToLower(s.Name), q) || strings.Contains(strings.ToLower(s.Ref), q) {
			out = append(out, s)
		}
	}
	return out
}

// deleteSubscriptions removes several subscriptions at once and reloads.
func (a *App) deleteSubscriptions(refs []string) {
	if len(refs) == 0 {
		return
	}
	drop := map[string]bool{}
	for _, r := range refs {
		drop[strings.TrimSpace(r)] = true
	}
	for r := range drop {
		a.tombstone(r)
	}
	kept := a.subs[:0]
	for _, s := range a.subs {
		if !drop[s.Ref] {
			kept = append(kept, s)
		}
	}
	a.subs = kept
	a.subSel = nil
	a.reloadSubs()
}

// setSubscriptionsEnabled enables or disables several subscriptions at once.
func (a *App) setSubscriptionsEnabled(refs []string, enabled bool) {
	if len(refs) == 0 {
		return
	}
	want := map[string]bool{}
	for _, r := range refs {
		want[strings.TrimSpace(r)] = true
	}
	for i := range a.subs {
		if want[a.subs[i].Ref] {
			a.subs[i].Enabled = enabled
		}
	}
	a.subSel = nil
	a.reloadSubs()
}

func cloneBlacklist(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, kw := range in {
		if kw = strings.TrimSpace(kw); kw != "" {
			out = append(out, kw)
		}
	}
	return out
}

// addBlacklist records a keyword and re-filters the loaded sources. The
// reload reuses the on-disk cache, so it does not re-probe every source.
// addBlacklist adds one or more keywords. Several can be pasted at once,
// separated by commas, semicolons, spaces or new lines (parseKeywords).
func (a *App) addBlacklist(raw string) {
	words := parseKeywords(raw)
	a.blacklistDraft = ""
	if len(words) == 0 {
		return
	}
	seen := map[string]bool{}
	for _, k := range a.blacklist {
		seen[strings.ToLower(k)] = true
	}
	changed := false
	for _, kw := range words {
		if seen[strings.ToLower(kw)] {
			continue
		}
		seen[strings.ToLower(kw)] = true
		a.blacklist = append(a.blacklist, kw)
		changed = true
	}
	if changed {
		a.reloadSubs()
	}
}

func (a *App) removeBlacklist(kw string) {
	a.removeBlacklists([]string{kw})
}

// removeBlacklists drops several keywords (case-insensitive) and reloads the
// subscriptions once.
func (a *App) removeBlacklists(kws []string) {
	drop := map[string]bool{}
	for _, k := range kws {
		drop[strings.ToLower(strings.TrimSpace(k))] = true
	}
	kept := make([]string, 0, len(a.blacklist))
	for _, k := range a.blacklist {
		if !drop[strings.ToLower(strings.TrimSpace(k))] {
			kept = append(kept, k)
		}
	}
	a.blSel = nil
	if len(kept) == len(a.blacklist) {
		return
	}
	a.blacklist = kept
	a.reloadSubs()
}

// blacklistHits lists the subscription's sources whose name contains kw.
func (a *App) blacklistHits(kw string) []string {
	kw = strings.ToLower(strings.TrimSpace(kw))
	if kw == "" {
		return nil
	}
	var out []string
	for _, s := range a.allSites {
		if name := s.Meta().Name; strings.Contains(strings.ToLower(name), kw) {
			out = append(out, name)
		}
	}
	return out
}

// blacklisted reports whether a source or live playlist name matches a
// blacklist keyword. Matching is case-insensitive substring.
func (a *App) blacklisted(name string) bool {
	if len(a.blacklist) == 0 {
		return false
	}
	n := strings.ToLower(name)
	for _, kw := range a.blacklist {
		if kw = strings.ToLower(strings.TrimSpace(kw)); kw != "" && strings.Contains(n, kw) {
			return true
		}
	}
	return false
}

// reloadSubs applies a change to the subscription list. With nothing enabled
// the app returns to the welcome screen instead of serving stale sources.
func (a *App) reloadSubs() {
	a.saveSettings()
	refs := a.activeRefs()
	if len(refs) == 0 {
		if a.loadCancel != nil {
			a.loadCancel()
		}
		a.bump()
		a.loading = false
		a.loadErr = ""
		a.loadNote = ""
		a.cfg = nil
		a.catalog = nil
		a.ballots = nil
		a.home = nil
		a.groupOf = nil
		a.screen = screenWelcome
		return
	}
	// Editing a subscription list must refresh the sources without yanking the
	// user back to the home screen; keepScreen makes apply/keepWorking skip the
	// screen navigation for this reload.
	a.keepScreen = true
	a.LoadSubs(refs)
}

// LoadSubs fetches every enabled subscription in parallel, merges them into
// one catalog and hands it to apply. It reuses the warehouse-index machinery:
// fetchEntries already expands index-only configs and mergeConfigs dedupes.
func (a *App) LoadSubs(refs []string) {
	refs = normalizeRefs(refs)
	if len(refs) == 0 {
		a.loading = false
		a.loadErr = "请先添加一份订阅地址"
		return
	}
	for _, ref := range refs {
		a.upsertSub(ref, true)
	}
	a.saveSettings()
	a.loading = true
	a.loadErr = ""
	a.loadNote = fmt.Sprintf("正在读取 %d 份订阅", len(refs))
	ctx, _ := a.beginLoad()
	key := comboKey(refs)
	subs := cloneSubs(a.subs)
	go func() {
		cctx, cancel := context.WithTimeout(withUserRefs(ctx, refs), 30*time.Second)
		defer cancel()
		cfgs := a.fetchEntries(cctx, refs, 0, &refSeen{m: map[string]struct{}{}})
		merged := mergeConfigs(cfgs)
		groups := groupsFor(subs, cfgs)
		a.post(func() {
			// A newer load cancels ctx. Unrelated navigation bumps gen, and must
			// not make a finished subscription load vanish.
			if ctx.Err() != nil {
				return
			}
			if len(merged.Sites) == 0 && len(merged.Lives) == 0 && len(merged.IndexEntries()) == 0 {
				a.loading = false
				a.loadNote = ""
				a.loadErr = "这些订阅都没有打开，或里面没有源"
				a.doneLoad()
				return
			}
			a.groupOf = groups
			a.apply(merged, key)
		})
	}()
}

// groupsFor maps a source's API (or a live playlist's URL) to the name of the
// subscription it came from, so the sidebar can group them. API is used as
// the key because mergeConfigs may rename duplicate site keys to key#N.
func groupsFor(subs []subscription, cfgs []*config.Config) map[string]string {
	out := map[string]string{}
	nameOf := func(sourceURL string) string {
		sourceURL = strings.TrimSpace(sourceURL)
		if sourceURL == "" {
			return ""
		}
		for _, s := range subs {
			if s.Ref == sourceURL {
				if s.Name != "" {
					return s.Name
				}
				return shortRef(s.Ref)
			}
		}
		// Local files are recorded as file:// URLs, so match by display name.
		label := shortRef(sourceURL)
		for _, s := range subs {
			if s.Name == label || shortRef(s.Ref) == label {
				if s.Name != "" {
					return s.Name
				}
				return shortRef(s.Ref)
			}
		}
		return label
	}
	put := func(key, name string) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		if _, ok := out[key]; !ok {
			out[key] = name
		}
	}
	for _, cfg := range cfgs {
		if cfg == nil {
			continue
		}
		name := nameOf(cfg.SourceURL)
		for _, site := range cfg.Sites {
			key := strings.TrimSpace(site.API)
			if key == "" {
				key = site.Key
			}
			put(key, name)
		}
		for _, lv := range cfg.Lives {
			put(lv.URL, name)
		}
	}
	return out
}

func (a *App) groupName(api, key, live string) string {
	if name := a.groupOf[strings.TrimSpace(api)]; name != "" {
		return name
	}
	if name := a.groupOf[strings.TrimSpace(key)]; name != "" {
		return name
	}
	if name := a.groupOf[strings.TrimSpace(live)]; name != "" {
		return name
	}
	return ""
}

// liveItem keeps the catalog-wide index, because sideSel encodes live:<n>.
type liveItem struct {
	idx   int
	entry *source.LiveEntry
}

type sideGroup struct {
	name  string
	sites []source.Source
	lives []liveItem
}

// sideGroups splits the sidebar into on-demand sources and live lists.
func (a *App) sideGroups() []sideGroup {
	vod := sideGroup{name: "片源"}
	live := sideGroup{name: "直播"}
	if a.catalog != nil {
		vod.sites = append(vod.sites, a.catalog.Sites...)
		for i, lv := range a.catalog.Lives {
			live.lives = append(live.lives, liveItem{idx: i, entry: lv})
		}
	}
	var groups []sideGroup
	if len(vod.sites) > 0 {
		groups = append(groups, vod)
	}
	if len(live.lives) > 0 {
		groups = append(groups, live)
	}
	return groups
}

// sidebarOpenState returns a stable pointer to a subscription group's
// expanded flag, so SidebarSection can collapse and reopen it in place.
// Groups start collapsed, so a long source list does not bury the
// navigation, unless they hold the source or live list on screen.
func (a *App) sidebarOpenState(name string, selected bool) *bool {
	if a.sidebarOpen == nil {
		a.sidebarOpen = map[string]*bool{}
	}
	if p, ok := a.sidebarOpen[name]; ok {
		return p
	}
	open := selected
	a.sidebarOpen[name] = &open
	return a.sidebarOpen[name]
}

// sideGroupTitle is the section name and how many entries it holds.
func sideGroupTitle(g sideGroup) string {
	name := strings.TrimSpace(g.name)
	if name == "" {
		name = "片源"
	}
	return fmt.Sprintf("%s  %d", name, len(g.sites)+len(g.lives))
}

// groupHasSelection reports whether the sidebar selection is in g.
func (a *App) groupHasSelection(g sideGroup) bool {
	for _, s := range g.sites {
		if a.sideSel == "site:"+s.Meta().Key {
			return true
		}
	}
	for _, li := range g.lives {
		if a.sideSel == fmt.Sprintf("live:%d", li.idx) {
			return true
		}
	}
	return false
}
