package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

const (
	// Fetch and probe run wide on purpose. A dead host holds its worker
	// until the timeout, so the cap stays finite, but 6-wide left most of
	// the wait in the queue.
	indexFetchParallel = 16
	indexProbeParallel = 48
	indexFetchTimeout  = 15 * time.Second
	indexProbeTimeout  = 8 * time.Second
	indexMaxDepth      = 2
	indexProbeCats     = 5
	cacheCatLimit      = 16
	cacheItemLimit     = 16
)

func (a *App) beginLoad() (context.Context, int) {
	if a.loadCancel != nil {
		a.loadCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.loadCtx = ctx
	a.loadCancel = cancel
	return ctx, a.bump()
}

func (a *App) doneLoad() {
	a.keepScreen = false
	if a.afterLoad != nil {
		fn := a.afterLoad
		a.afterLoad = nil
		fn()
	}
}

func (a *App) fetch(ctx context.Context, ref string) (*config.Config, error) {
	if a.fetchConfig != nil {
		return a.fetchConfig(ctx, ref)
	}
	return a.fetchCached(ctx, ref, isNestedRef(ctx, ref))
}

func (a *App) sample(ctx context.Context, s source.Source) (model.Page, bool) {
	if a.probeSite != nil {
		return a.probeSite(ctx, s)
	}
	return sourceSample(ctx, s)
}

// expandIndex loads every address in a warehouse, then keeps only the
// sources whose first pages actually list something.
func (a *App) expandIndex(entries []config.NamedURL) {
	ctx := a.loadCtx
	if ctx == nil {
		ctx = context.Background()
	}
	refs := indexRefs(entries)
	a.loading = true
	a.loadNote = fmt.Sprintf("正在读取 %d 份配置", len(refs))
	go func() {
		cfgs := a.fetchEntries(ctx, refs, 0, &refSeen{m: map[string]struct{}{}})
		merged := mergeConfigs(cfgs)
		a.post(func() {
			if ctx.Err() != nil {
				return
			}
			opened := source.Open(merged)
			a.allSites = append([]source.Source(nil), opened.Sites...)
			if len(opened.Sites) == 0 && len(opened.Lives) == 0 {
				a.loading = false
				a.loadNote = ""
				a.loadErr = "这些配置都没有打开，或里面没有源"
				a.screen = screenEmpty
				a.catalog = opened
				a.doneLoad()
				return
			}
			a.cfg = merged
			if c := a.readCache(a.cfgRef); c != nil {
				a.restoreFromCache(c, opened)
				return
			}
			a.catalog = &source.Catalog{Parses: opened.Parses}
			a.keepWorking(ctx, opened)
		})
	}()
}

type refSeen struct {
	mu sync.Mutex
	m  map[string]struct{}
}

func (s *refSeen) add(ref string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[ref]; ok {
		return false
	}
	s.m[ref] = struct{}{}
	return true
}

func (a *App) fetchEntries(ctx context.Context, refs []string, depth int, seen *refSeen) []*config.Config {
	buckets := make([][]*config.Config, len(refs))
	var done atomic.Int32
	total := int32(len(refs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, indexFetchParallel)
	for i, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" || !seen.add(ref) {
			continue
		}
		wg.Add(1)
		go func(i int, ref string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			cctx, cancel := context.WithTimeout(ctx, indexFetchTimeout)
			defer cancel()
			cfg, err := a.fetch(cctx, ref)
			n := done.Add(1)
			a.post(func() {
				if ctx.Err() != nil {
					return
				}
				a.loadNote = fmt.Sprintf("正在读取配置 %d/%d", n, total)
			})
			if err != nil || cfg == nil {
				return
			}
			if len(cfg.Sites) == 0 && len(cfg.Lives) == 0 && depth+1 < indexMaxDepth {
				if nested := cfg.IndexEntries(); len(nested) > 0 {
					buckets[i] = a.fetchEntries(ctx, indexRefs(nested), depth+1, seen)
					return
				}
			}
			buckets[i] = []*config.Config{cfg}
		}(i, ref)
	}
	wg.Wait()
	var out []*config.Config
	for _, b := range buckets {
		out = append(out, b...)
	}
	return out
}

func (a *App) keepWorking(ctx context.Context, full *source.Catalog) {
	a.keepWorkingMode(ctx, full, false)
}

// keepWorkingMode runs the collection pass. In silent mode the home restored
// from cache stays on screen and no loading state is shown: each source's
// fresh ballots replace its cached ones as they arrive.
func (a *App) keepWorkingMode(ctx context.Context, full *source.Catalog, silent bool) {
	sites := append([]source.Source(nil), full.Sites...)
	lives := append([]*source.LiveEntry(nil), full.Lives...)
	total := len(sites) + len(lives)
	if !silent {
		a.loading = true
		a.loadNote = fmt.Sprintf("正在检查 %d 个源", total)
		if !a.keepScreen {
			a.screen = screenHome
			a.sideSel = "home"
			a.lastSide = "home"
			a.siteKey = ""
			a.home = nil
		}
	}
	skipped := map[string]bool{}
	for id := range a.hidden {
		skipped[id] = true
	}
	for id := range a.disabled {
		skipped[id] = true
	}
	ref := a.cfgRef
	rules := cloneRules(a.activeRules())
	if !silent {
		a.catFacts = nil
	}
	// A silent refresh starts from the sources the cache restored, so the
	// first probe result does not shrink the sidebar to a single source.
	seedSite := make([]bool, len(sites))
	seedLive := make([]bool, len(lives))
	if silent && a.catalog != nil {
		liveKept := map[string]bool{}
		for _, lv := range a.catalog.Lives {
			liveKept[lv.URL] = true
		}
		for i, s := range sites {
			seedSite[i] = a.catalog.Find(s.Meta().Key) != nil
		}
		for i, lv := range lives {
			seedLive[i] = liveKept[lv.URL]
		}
	}
	go func() {
		okSite := append([]bool(nil), seedSite...)
		okLive := append([]bool(nil), seedLive...)
		var mu sync.Mutex
		var cacheMu sync.Mutex
		var cacheSites []cacheSite
		var cacheLives []string
		var checked atomic.Int32
		publish := func() {
			mu.Lock()
			type kept struct {
				src source.Source
			}
			got := make([]kept, 0, len(sites))
			for i, s := range sites {
				if okSite[i] {
					got = append(got, kept{src: s})
				}
			}
			liveSnap := make([]*source.LiveEntry, 0, len(lives))
			for i, lv := range lives {
				if okLive[i] {
					liveSnap = append(liveSnap, lv)
				}
			}
			n := int(checked.Load())
			mu.Unlock()
			a.post(func() {
				if ctx.Err() != nil {
					return
				}
				siteSnap := make([]source.Source, 0, len(got))
				for _, g := range got {
					if a.hidden[sourceHideID(g.src)] || a.disabled[sourceHideID(g.src)] || a.blacklisted(g.src.Meta().Name) {
						continue
					}
					siteSnap = append(siteSnap, g.src)
				}
				shownLive := liveSnap[:0]
				for _, lv := range liveSnap {
					if !a.hidden[liveHideID(lv.URL)] && !a.blacklisted(lv.Name) {
						shownLive = append(shownLive, lv)
					}
				}
				// Show home as sources come in, but only from screens that
				// are waiting for them: opening a title (or anything else)
				// during the load keeps sideSel at "home", and must not be
				// undone by the next probe result.
				if a.homeAwaitsSources() {
					a.screen = screenHome
					a.sideSel = "home"
					a.lastSide = "home"
				}
				a.catalog.Sites = siteSnap
				a.catalog.Lives = shownLive
				a.loadNote = fmt.Sprintf("正在检查源 %d/%d，可用 %d", n, total, len(siteSnap)+len(shownLive))
			})
		}
		dead := a.loadDead()
		sem := make(chan struct{}, indexProbeParallel)
		var wg sync.WaitGroup
		for i, s := range sites {
			if s.Meta().Level == config.LevelNo || skipped[sourceHideID(s)] || a.blacklisted(s.Meta().Name) || a.isDead(sourceHideID(s), dead) {
				checked.Add(1)
				continue
			}
			wg.Add(1)
			go func(i int, s source.Source) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}
				cctx, cancel := context.WithTimeout(ctx, indexProbeTimeout)
				page, ok := a.sample(cctx, s)
				cancel()
				checked.Add(1)
				if ok {
					mu.Lock()
					okSite[i] = true
					mu.Unlock()
					publish()
					m := s.Meta()
					voter := voterFor(m)
					hot := hotCandidates(page.Items)
					var cats []cacheCat
					var names []model.Category
					if a.probeSite == nil {
						bctx, bcancel := context.WithTimeout(ctx, 20*time.Second)
						cats, names = collectSourceWith(bctx, s, rules)
						bcancel()
						if len(hot) == 0 && len(cats) > 0 {
							hot = hotCandidates(cats[0].Items)
						}
					}
					if len(names) > 0 {
						names := append([]model.Category(nil), names...)
						key, siteName := m.Key, m.Name
						a.post(func() {
							if ctx.Err() != nil {
								return
							}
							a.replaceCatFacts(key, siteName, names)
						})
					}
					if len(hot) > 0 || len(cats) > 0 || len(names) > 0 {
						cacheMu.Lock()
						cacheSites = append(cacheSites, cacheSite{
							Key: m.Key, Name: m.Name, Voter: voter,
							Hot: hot, Cats: cats, Names: cacheNames(names),
						})
						cacheMu.Unlock()
					}
					var got []ballot
					for _, it := range hot {
						got = append(got, ballot{
							SiteKey: m.Key, SiteName: m.Name, Voter: voter,
							Bucket: hotLaneID, Item: it,
						})
					}
					for _, cat := range cats {
						for _, it := range cat.Items {
							if !usableTitle(it.Title) {
								continue
							}
							got = append(got, ballot{
								SiteKey: m.Key, SiteName: m.Name, Voter: voter,
								Cat: cat.Name, Item: it,
							})
						}
					}
					if len(got) > 0 {
						got := got
						key := m.Key
						a.post(func() {
							if ctx.Err() != nil {
								return
							}
							a.mergeBallots(key, got)
						})
					}
				} else {
					publish()
				}
			}(i, s)
		}
		for i, lv := range lives {
			if skipped[liveHideID(lv.URL)] || a.isDead(liveHideID(lv.URL), dead) {
				checked.Add(1)
				continue
			}
			wg.Add(1)
			go func(i int, lv *source.LiveEntry) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}
				cctx, cancel := context.WithTimeout(ctx, indexProbeTimeout)
				chs, err := lv.Load(cctx)
				cancel()
				checked.Add(1)
				if err == nil && len(chs) > 0 {
					mu.Lock()
					okLive[i] = true
					mu.Unlock()
					cacheMu.Lock()
					cacheLives = append(cacheLives, lv.URL)
					cacheMu.Unlock()
				}
				publish()
			}(i, lv)
		}
		wg.Wait()
		// Persist the dead list from this pass: sources that failed are skipped
		// for deadBlacklistTTL on the next load instead of being retried every
		// time, and sources that recovered are removed.
		if ctx.Err() == nil {
			now := time.Now()
			for i, s := range sites {
				id := sourceHideID(s)
				if s.Meta().Level == config.LevelNo || skipped[id] {
					continue
				}
				if okSite[i] {
					delete(dead, id)
				} else {
					dead[id] = now
				}
			}
			for i, lv := range lives {
				id := liveHideID(lv.URL)
				if skipped[id] {
					continue
				}
				if okLive[i] {
					delete(dead, id)
				} else {
					dead[id] = now
				}
			}
			a.saveDead(dead)
		}
		// Only a complete pass is worth persisting; a cancelled load should be
		// collected again next time instead of caching a partial result.
		if ctx.Err() == nil && (len(cacheSites) > 0 || len(cacheLives) > 0) {
			a.writeCache(&cacheData{Ref: ref, Sites: cacheSites, Lives: cacheLives})
		}
		a.post(func() {
			if ctx.Err() != nil {
				return
			}
			a.loading = false
			a.loadNote = ""
			if len(a.catalog.Sites) == 0 && len(a.catalog.Lives) == 0 {
				a.loadErr = "配置都打开了，但没有能列出内容的源"
				a.screen = screenEmpty
			}
			a.doneLoad()
		})
	}()
}

func indexRefs(entries []config.NamedURL) []string {
	out := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, e := range entries {
		u := strings.TrimSpace(e.URL)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

func mergeConfigs(cfgs []*config.Config) *config.Config {
	dst := &config.Config{}
	keys := map[string]int{}
	sigs := map[string]bool{}
	lives := map[string]bool{}
	for _, cfg := range cfgs {
		if cfg == nil {
			continue
		}
		if dst.Spider == "" {
			dst.Spider = cfg.Spider
		}
		dst.Parses = append(dst.Parses, cfg.Parses...)
		for _, site := range cfg.Sites {
			api := strings.TrimSpace(site.API)
			sig := api + "\n" + strings.TrimSpace(site.ExtString())
			if api != "" {
				if sigs[sig] {
					continue
				}
				sigs[sig] = true
			}
			base := site.Key
			if base == "" {
				base = site.Name
			}
			keys[base]++
			if keys[base] > 1 {
				site.Key = fmt.Sprintf("%s#%d", base, keys[base])
			} else if site.Key == "" {
				site.Key = base
			}
			dst.Sites = append(dst.Sites, site)
		}
		for _, lv := range cfg.Lives {
			u := strings.TrimSpace(lv.URL)
			if u == "" || lives[u] {
				continue
			}
			lives[u] = true
			dst.Lives = append(dst.Lives, lv)
		}
	}
	return dst
}

// hotCandidates keeps a bounded, deduplicated set of usable titles from a
// source's first non-empty page. It is the raw fact behind the hot lane.
func hotCandidates(items []model.Item) []model.Item {
	out := make([]model.Item, 0, cacheItemLimit)
	seen := map[string]bool{}
	for _, it := range items {
		if !usableTitle(it.Title) {
			continue
		}
		id := normTitle(it.Title)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, it)
		if len(out) >= cacheItemLimit {
			break
		}
	}
	return out
}

// sourceSample returns the earliest non-empty page among the first
// indexProbeCats categories. They are requested together because "推荐" is
// often empty while a later category is not. Keeping the lowest category index
// makes the homepage hot candidates deterministic.
func sourceSample(ctx context.Context, s source.Source) (model.Page, bool) {
	if s == nil || s.Meta().Level == config.LevelNo {
		return model.Page{}, false
	}
	cats, _ := s.Categories(ctx)
	if len(cats) == 0 || ctx.Err() != nil {
		return model.Page{}, false
	}
	n := len(cats)
	if n > indexProbeCats {
		n = indexProbeCats
	}
	type sampleHit struct {
		index int
		page  model.Page
	}
	results := make(chan sampleHit, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			page, err := s.List(ctx, id, 1)
			if err != nil || len(page.Items) == 0 {
				return
			}
			results <- sampleHit{index: i, page: page}
		}(i, cats[i].ID)
	}
	wg.Wait()
	close(results)
	var best sampleHit
	found := false
	for r := range results {
		if !found || r.index < best.index {
			best = r
			found = true
		}
	}
	return best.page, found
}

// collectSource records every usable category of a source, not just the ones
// the current homepage rules happen to match. New rule keywords can then be
// applied to the cached facts without another network pass.
func collectSource(ctx context.Context, s source.Source) []cacheCat {
	cats, _ := collectSourceWith(ctx, s, nil)
	return cats
}

// wantedJunk reports whether a junk-genre category is named by a keyword the
// user wrote into a rule on purpose; such categories are collected too.
func wantedJunk(name string, rules []homeRule) bool {
	for _, rule := range rules {
		if ruleNamesJunk(rule) && keywordHit(name, rule.Keywords) {
			return true
		}
	}
	return false
}

// collectSourceWith is collectSource that also keeps the junk-genre categories
// the given rules ask for by name. They do not count against cacheCatLimit.
// The returned names include every usable category, junk genres included,
// so the category page can list them without another scan.
func collectSourceWith(ctx context.Context, s source.Source, rules []homeRule) ([]cacheCat, []model.Category) {
	cats, err := s.Categories(ctx)
	if err != nil || len(cats) == 0 {
		return nil, nil
	}
	var names []model.Category
	picks := make([]model.Category, 0, cacheCatLimit)
	var extra []model.Category
	for _, cat := range cats {
		name := strings.TrimSpace(cat.Name)
		if !usableTitle(name) {
			continue
		}
		names = append(names, model.Category{ID: cat.ID, Name: name})
		if junkCategory(name) {
			if wantedJunk(name, rules) {
				extra = append(extra, cat)
			}
			continue
		}
		if len(picks) < cacheCatLimit {
			picks = append(picks, cat)
		}
	}
	picks = append(picks, extra...)
	out := make([]cacheCat, len(picks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, cat := range picks {
		wg.Add(1)
		go func(i int, cat model.Category) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			page, err := s.List(ctx, cat.ID, 1)
			if err != nil || len(page.Items) == 0 {
				return
			}
			items := page.Items
			if len(items) > cacheItemLimit {
				items = items[:cacheItemLimit]
			}
			out[i] = cacheCat{Name: strings.TrimSpace(cat.Name), Items: items}
		}(i, cat)
	}
	wg.Wait()
	var kept []cacheCat
	for _, cat := range out {
		if len(cat.Items) > 0 {
			kept = append(kept, cat)
		}
	}
	return kept, names
}

// recollect is the manual refresh: discard the cache and run one fresh
// collection pass over the current subscription.
func (a *App) recollect() {
	if a.cfg == nil {
		return
	}
	a.clearCache()
	// 重新采集 means fresh from the network: configs and warehouse entries
	// too, including the ones skipped for failing lately.
	a.clearConfigCache()
	if refs := a.activeRefs(); len(refs) > 0 {
		a.keepScreen = true
		a.LoadSubs(refs)
		return
	}
	a.ballots = nil
	a.home = nil
	full := source.Open(a.cfg)
	a.allSites = append([]source.Source(nil), full.Sites...)
	a.catalog = &source.Catalog{Parses: full.Parses, Index: full.Index}
	a.keepWorking(a.loadCtxOrBackground(), full)
}

// homeAwaitsSources reports whether the screen is still waiting for the first
// sources of a load, so that a probe result may switch it to home. Opening a
// title, a source or a player during the load leaves sideSel at "home"; those
// screens must stay put.
func (a *App) homeAwaitsSources() bool {
	if a.sideSel != "" && a.sideSel != "home" {
		return false
	}
	switch a.screen {
	case screenHome, screenEmpty, screenIndex, screenWelcome:
		return true
	}
	return false
}
