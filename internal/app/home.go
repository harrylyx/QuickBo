package app

import (
	"context"
	"sort"
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
	a.home = withoutLane(a.home, key)
	a.homeHits = withoutLane(a.homeHits, key)
	if a.siteKey == key {
		a.siteKey = ""
		a.page = model.Page{}
		a.detail = nil
		a.cats = nil
	}
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
	a.saveSettings()
}

func (a *App) dropLive(url string) {
	if a.catalog == nil {
		return
	}
	url = strings.TrimSpace(url)
	kept := make([]*source.LiveEntry, 0, len(a.catalog.Lives))
	found := false
	for _, lv := range a.catalog.Lives {
		if strings.TrimSpace(lv.URL) == url {
			found = true
			continue
		}
		kept = append(kept, lv)
	}
	if !found {
		return
	}
	a.catalog.Lives = kept
	if a.hidden == nil {
		a.hidden = map[string]bool{}
	}
	a.hidden[liveHideID(url)] = true
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
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
		hits := make([]homeLane, len(sites))
		var wg sync.WaitGroup
		sem := make(chan struct{}, indexProbeParallel)
		for i, s := range sites {
			wg.Add(1)
			go func(i int, s source.Source) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				ctx, cancel := context.WithTimeout(context.Background(), indexProbeTimeout)
				defer cancel()
				page, err := s.Search(ctx, q, 1)
				if err != nil || len(page.Items) == 0 {
					return
				}
				items := page.Items
				if len(items) > homeRailLimit {
					items = items[:homeRailLimit]
				}
				m := s.Meta()
				hits[i] = homeLane{Key: m.Key, Name: m.Name, Items: items}
			}(i, s)
		}
		wg.Wait()
		a.post(func() {
			if a.gen != gen {
				return
			}
			lanes := make([]homeLane, 0, len(hits))
			for _, lane := range hits {
				if lane.Key == "" {
					continue
				}
				if src := findSource(sites, lane.Key); src != nil && a.hidden[sourceHideID(src)] {
					continue
				}
				lanes = append(lanes, lane)
			}
			a.homeHits = lanes
			a.homeSearching = false
		})
	}()
}

func findSource(sites []source.Source, key string) source.Source {
	for _, s := range sites {
		if s.Meta().Key == key {
			return s
		}
	}
	return nil
}
