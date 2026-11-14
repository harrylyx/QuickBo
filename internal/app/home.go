package app

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

// homeLane is one source's row on the aggregated home screen.
type homeLane struct {
	Key   string
	Name  string
	Items []model.Item
}

func (a *App) loadCtxOrBackground() context.Context {
	if a.loadCtx != nil {
		return a.loadCtx
	}
	return context.Background()
}

func sourceHideID(s source.Source) string {
	m := s.Meta()
	if api := strings.TrimSpace(m.API); api != "" {
		return "site:" + api
	}
	return "site:" + m.Key
}

func liveHideID(url string) string {
	return "live:" + strings.TrimSpace(url)
}

func hiddenIDs(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func disabledIDs(set map[string]bool) []string {
	return hiddenIDs(set)
}

func (a *App) dropSite(key string) {
	if a.catalog == nil {
		return
	}
	var gone source.Source
	kept := make([]source.Source, 0, len(a.catalog.Sites))
	for _, s := range a.catalog.Sites {
		if s.Meta().Key == key {
			gone = s
			continue
		}
		kept = append(kept, s)
	}
	if gone == nil {
		return
	}
	a.catalog.Sites = kept
	if a.hidden == nil {
		a.hidden = map[string]bool{}
	}
	a.hidden[sourceHideID(gone)] = true
	before := len(a.ballots)
	keptBallots := make([]ballot, 0, len(a.ballots))
	for _, b := range a.ballots {
		if b.SiteKey != key {
			keptBallots = append(keptBallots, b)
		}
	}
	a.ballots = keptBallots
	if len(a.ballots) != before {
		a.rebuildHome()
	} else {
		a.home = withoutLane(a.home, key)
	}
	a.homeHits = withoutLane(a.homeHits, key)
	if a.siteKey == key {
		a.siteKey = ""
		a.page = model.Page{}
		a.detail = nil
		a.cats = nil
	}
	// Leave the current screen alone unless it was showing the source just
	// removed: swapping the main view on every delete from the sidebar is
	// what made deleting feel jumpy.
	if a.sideSel == "site:"+key {
		a.screen = screenHome
		a.sideSel = "home"
		a.lastSide = "home"
	}
	a.saveSettings()
}

// removeSites drops several sources at once and rebuilds the home lanes in
// place, without leaving the current screen (the sources view stays open).
func (a *App) removeSites(keys []string) {
	if a.catalog == nil || len(keys) == 0 {
		return
	}
	drop := map[string]bool{}
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			drop[k] = true
		}
	}
	if len(drop) == 0 {
		return
	}
	// Hide from the full list, not just the catalog: a source that failed its
	// probe is listed in the management view (allSites) but was never put in
	// the catalog, and must be removable too.
	removed := false
	for _, s := range a.allSites {
		if drop[s.Meta().Key] {
			if a.hidden == nil {
				a.hidden = map[string]bool{}
			}
			a.hidden[sourceHideID(s)] = true
			removed = true
		}
	}
	kept := make([]source.Source, 0, len(a.catalog.Sites))
	for _, s := range a.catalog.Sites {
		if drop[s.Meta().Key] {
			if a.hidden == nil {
				a.hidden = map[string]bool{}
			}
			a.hidden[sourceHideID(s)] = true
			removed = true
			continue
		}
		kept = append(kept, s)
	}
	if !removed {
		return
	}
	a.catalog.Sites = kept
	before := len(a.ballots)
	keptBallots := a.ballots[:0]
	for _, b := range a.ballots {
		if !drop[b.SiteKey] {
			keptBallots = append(keptBallots, b)
		}
	}
	a.ballots = keptBallots
	if len(a.ballots) != before {
		a.rebuildHome()
	} else {
		for k := range drop {
			a.home = withoutLane(a.home, k)
		}
	}
	for k := range drop {
		a.homeHits = withoutLane(a.homeHits, k)
	}
	if a.siteKey != "" && drop[a.siteKey] {
		a.siteKey = ""
		a.page = model.Page{}
		a.detail = nil
		a.cats = nil
	}
	a.srcSel = nil
	a.saveSettings()
}

// disableSites turns sources off without removing them from the subscription.
// They stay visible in the management list and can be re-enabled.
func (a *App) disableSites(keys []string) {
	if len(keys) == 0 {
		return
	}
	drop := map[string]bool{}
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			drop[k] = true
		}
	}
	if a.disabled == nil {
		a.disabled = map[string]bool{}
	}
	for _, s := range a.allSites {
		if drop[s.Meta().Key] {
			a.disabled[sourceHideID(s)] = true
		}
	}
	if a.catalog != nil {
		kept := a.catalog.Sites[:0]
		for _, s := range a.catalog.Sites {
			if drop[s.Meta().Key] {
				continue
			}
			kept = append(kept, s)
		}
		a.catalog.Sites = kept
	}
	before := len(a.ballots)
	keptBallots := a.ballots[:0]
	for _, b := range a.ballots {
		if !drop[b.SiteKey] {
			keptBallots = append(keptBallots, b)
		}
	}
	a.ballots = keptBallots
	if len(a.ballots) != before {
		a.rebuildHome()
	} else {
		for k := range drop {
			a.home = withoutLane(a.home, k)
		}
	}
	for k := range drop {
		a.homeHits = withoutLane(a.homeHits, k)
	}
	if a.siteKey != "" && drop[a.siteKey] {
		a.siteKey = ""
		a.page = model.Page{}
		a.detail = nil
		a.cats = nil
	}
	a.srcSel = nil
	a.saveSettings()
}

// enableSites re-enables sources and re-collects their content in place.
func (a *App) enableSites(keys []string) {
	if len(keys) == 0 {
		return
	}
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if s := a.findAll(k); s != nil {
			delete(a.disabled, sourceHideID(s))
		}
	}
	a.srcSel = nil
	a.saveSettings()
	a.recollect()
}

// addSourcesBlacklist hides every selected source by name (batch add).
func (a *App) addSourcesBlacklist(keys []string) {
	if len(keys) == 0 {
		return
	}
	seen := map[string]bool{}
	for _, kw := range a.blacklist {
		seen[strings.ToLower(kw)] = true
	}
	changed := false
	for _, k := range keys {
		s := a.findAll(k)
		if s == nil {
			continue
		}
		name := strings.TrimSpace(s.Meta().Name)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		a.blacklist = append(a.blacklist, name)
		changed = true
	}
	if changed {
		a.srcSel = nil
		a.reloadSubs()
	}
}

func (a *App) dropLive(url string) {
	if a.catalog == nil {
		return
	}
	url = strings.TrimSpace(url)
	kept := make([]*source.LiveEntry, 0, len(a.catalog.Lives))
	removedAt := -1
	for i, lv := range a.catalog.Lives {
		if strings.TrimSpace(lv.URL) == url {
			if removedAt < 0 {
				removedAt = i
			}
			continue
		}
		kept = append(kept, lv)
	}
	if removedAt < 0 {
		return
	}
	a.catalog.Lives = kept
	if a.hidden == nil {
		a.hidden = map[string]bool{}
	}
	a.hidden[liveHideID(url)] = true
	// Sidebar ids are live:<index>, so the lists after the removed one move
	// up by one. Only leave the screen if it showed the removed list.
	viewing := strings.HasPrefix(a.sideSel, "live:")
	switch {
	case viewing && a.liveIndex == removedAt:
		a.screen = screenHome
		a.sideSel = "home"
		a.lastSide = "home"
	case a.liveIndex > removedAt:
		a.liveIndex--
		if viewing {
			a.sideSel = "live:" + strconv.Itoa(a.liveIndex)
			a.lastSide = a.sideSel
		}
	}
	a.saveSettings()
}

func withoutLane(lanes []homeLane, key string) []homeLane {
	out := make([]homeLane, 0, len(lanes))
	for _, lane := range lanes {
		if lane.Key != key {
			out = append(out, lane)
		}
	}
	return out
}

// searchHitLimit caps the poster wall: a search reads page one of every
// source at once, so the merged list can be large. A home rail holds 18.
const searchHitLimit = 120

// searchLane merges a search's hits across sources. The wall shows far more
// than a rail and keeps titles without a poster (they sort last), so every
// hit stays reachable instead of being cut to one row's worth.
func searchLane(ballots []ballot) (homeLane, map[string][]titleHit) {
	lane, part := aggregateLane("search", "搜索结果", ballots)
	if len(lane.Items) > searchHitLimit {
		lane.Items = lane.Items[:searchHitLimit]
	}
	return lane, part
}

// searchHome asks every kept source for the committed query and groups
// the hits by source. An empty query restores the aggregated rows.
func (a *App) searchHome() {
	q := strings.TrimSpace(a.committed)
	a.homeHits = nil
	if q == "" || a.catalog == nil || len(a.catalog.Sites) == 0 {
		a.homeSearching = false
		return
	}
	a.homeSearching = true
	sites := append([]source.Source(nil), a.catalog.Sites...)
	gen := a.bump()
	go func() {
		var mu sync.Mutex
		var found []ballot
		var wg sync.WaitGroup
		sem := make(chan struct{}, indexProbeParallel)
		for _, s := range sites {
			wg.Add(1)
			go func(s source.Source) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				ctx, cancel := context.WithTimeout(context.Background(), indexProbeTimeout)
				defer cancel()
				page, err := s.Search(ctx, q, 1)
				if err != nil || len(page.Items) == 0 {
					return
				}
				m := s.Meta()
				mu.Lock()
				for _, it := range page.Items {
					found = append(found, ballot{SiteKey: m.Key, SiteName: m.Name, Bucket: hotLaneID, Item: it})
				}
				mu.Unlock()
			}(s)
		}
		wg.Wait()
		a.post(func() {
			if a.gen != gen {
				return
			}
			lane, part := searchLane(found)
			if len(lane.Items) == 0 {
				a.homeHits = nil
				a.searchLookup = nil
			} else {
				a.homeHits = []homeLane{lane}
				a.searchLookup = part
			}
			a.homeSearching = false
		})
	}()
}
