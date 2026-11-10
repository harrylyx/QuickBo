package app

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
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
	kept := a.subs[:0]
	for _, s := range a.subs {
		if s.Ref != ref {
			kept = append(kept, s)
		}
	}
	a.subs = kept
	a.reloadSubs()
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
	ctx, gen := a.beginLoad()
	key := comboKey(refs)
	subs := cloneSubs(a.subs)
	go func() {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cfgs := a.fetchEntries(cctx, refs, 0, &refSeen{m: map[string]struct{}{}})
		merged := mergeConfigs(cfgs)
		groups := groupsFor(subs, cfgs)
		a.post(func() {
			if a.gen != gen {
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

// sideGroups orders groups by the saved subscription list, putting sources
// whose subscription is unknown into a final "其他" group.
func (a *App) sideGroups() []sideGroup {
	var groups []sideGroup
	index := map[string]int{}
	for _, s := range a.subs {
		name := s.Name
		if name == "" {
			name = shortRef(s.Ref)
		}
		if _, ok := index[name]; ok {
			continue
		}
		index[name] = len(groups)
		groups = append(groups, sideGroup{name: name})
	}
	other := -1
	at := func(name string) int {
		if name == "" {
			if other < 0 {
				other = len(groups)
				index[""] = other
				groups = append(groups, sideGroup{name: "其他"})
			}
			return other
		}
		if i, ok := index[name]; ok {
			return i
		}
		index[name] = len(groups)
		groups = append(groups, sideGroup{name: name})
		return len(groups) - 1
	}
	if a.catalog != nil {
		for _, s := range a.catalog.Sites {
			m := s.Meta()
			i := at(a.groupName(m.API, m.Key, ""))
			groups[i].sites = append(groups[i].sites, s)
		}
		for i, lv := range a.catalog.Lives {
			j := at(a.groupName("", "", lv.URL))
			groups[j].lives = append(groups[j].lives, liveItem{idx: i, entry: lv})
		}
	}
	kept := groups[:0]
	for _, g := range groups {
		if len(g.sites) > 0 || len(g.lives) > 0 {
			kept = append(kept, g)
		}
	}
	return kept
}
