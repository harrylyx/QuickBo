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
	indexProbeCats     = 3
	homeRailLimit      = 12
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
	return config.Load(ctx, ref)
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
	sites := append([]source.Source(nil), full.Sites...)
	lives := append([]*source.LiveEntry(nil), full.Lives...)
	total := len(sites) + len(lives)
	a.loading = true
	a.loadNote = fmt.Sprintf("正在检查 %d 个源", total)
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
	a.siteKey = ""
	a.home = nil
	skipped := map[string]bool{}
	for id := range a.hidden {
		skipped[id] = true
	}
	go func() {
		okSite := make([]bool, len(sites))
		pages := make([]model.Page, len(sites))
		okLive := make([]bool, len(lives))
		var mu sync.Mutex
		var checked atomic.Int32
		publish := func() {
			mu.Lock()
			type kept struct {
				src   source.Source
				items []model.Item
			}
			got := make([]kept, 0, len(sites))
			for i, s := range sites {
				if okSite[i] {
					items := pages[i].Items
					if len(items) > homeRailLimit {
						items = append([]model.Item(nil), items[:homeRailLimit]...)
					}
					got = append(got, kept{src: s, items: items})
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
				lanes := make([]homeLane, 0, len(got))
				for _, g := range got {
					if a.hidden[sourceHideID(g.src)] {
						continue
					}
					m := g.src.Meta()
					lanes = append(lanes, homeLane{Key: m.Key, Name: m.Name, Items: g.items})
					siteSnap = append(siteSnap, g.src)
				}
				shownLive := liveSnap[:0]
				for _, lv := range liveSnap {
					if !a.hidden[liveHideID(lv.URL)] {
						shownLive = append(shownLive, lv)
					}
				}
				if a.sideSel == "" || a.sideSel == "home" {
					a.screen = screenHome
					a.sideSel = "home"
					a.lastSide = "home"
				}
				a.catalog.Sites = siteSnap
				a.catalog.Lives = shownLive
				a.home = lanes
				a.loadNote = fmt.Sprintf("正在检查源 %d/%d，可用 %d", n, total, len(siteSnap)+len(shownLive))
			})
		}
		sem := make(chan struct{}, indexProbeParallel)
		var wg sync.WaitGroup
		for i, s := range sites {
			if s.Meta().Level == config.LevelNo || skipped[sourceHideID(s)] {
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
					pages[i] = page
					mu.Unlock()
				}
				publish()
			}(i, s)
		}
		for i, lv := range lives {
			if skipped[liveHideID(lv.URL)] {
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
				}
				publish()
			}(i, lv)
		}
		wg.Wait()
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

// sourceSample returns the first page that actually lists titles.
// The first few categories are requested together, because "推荐" is
// often empty while a later category is not.
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		mu    sync.Mutex
		hit   model.Page
		found bool
		wg    sync.WaitGroup
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			page, err := s.List(ctx, id, 1)
			if err != nil || len(page.Items) == 0 {
				return
			}
			mu.Lock()
			if !found {
				found = true
				hit = page
				cancel()
			}
			mu.Unlock()
		}(cats[i].ID)
	}
	wg.Wait()
	return hit, found
}
