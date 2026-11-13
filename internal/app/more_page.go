package app

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

// moreFirstCats caps how many matching categories of one source feed the
// aggregate page; morePageTimeout bounds one page request.
const (
	moreFirstCats   = 4
	morePageTimeout = 20 * time.Second
)

// moreCursor is where one matching category of one source has been read up
// to on the aggregate page. Cursors are only touched on the UI thread; the
// fetch goroutines get copies.
type moreCursor struct {
	src      source.Source
	siteKey  string
	siteName string
	voter    string
	catID    string
	next     int             // page to read next
	done     bool            // no further page, or the source stopped answering
	seen     map[string]bool // item IDs already read, to catch sources that ignore the page number
}

// morePage is one fetched page for the cursor at index.
type morePage struct {
	index int
	page  model.Page
	err   error
}

func (a *App) ruleByID(id string) (homeRule, bool) {
	for _, r := range a.activeRules() {
		if r.ID == id {
			return r, true
		}
	}
	return homeRule{}, false
}

// canMore reports whether a home row has an aggregate page behind it.
func (a *App) canMore(lane homeLane) bool {
	if a.catalog == nil || len(a.catalog.Sites) == 0 {
		return false
	}
	_, ok := a.ruleByID(lane.Key)
	return ok
}

// openMore opens the aggregate page of a home row: every title the sources
// list under that row's keywords, merged across sources. The first page of
// each matching category is read now; later pages load as the user scrolls
// to the bottom (maybeMoreNext).
func (a *App) openMore(lane homeLane) {
	rule, ok := a.ruleByID(lane.Key)
	if !ok || a.catalog == nil {
		return
	}
	a.closePlayer()
	a.screen = screenMore
	a.sideSel, a.lastSide = "home", "home"
	a.moreRow = homeLane{Key: lane.Key, Name: lane.Name}
	a.moreLookup = nil
	a.moreKeys = nil
	a.moreBallots = nil
	a.moreCursors = nil
	a.moreLoading = true
	a.moreAppending = false
	a.browseScroll.Y = 0
	sites := append([]source.Source(nil), a.catalog.Sites...)
	rules := cloneRules(a.activeRules())
	a.moreSeq++
	seq := a.moreSeq
	go func() {
		cursors := discoverMore(sites, rule, rules)
		pages := fetchMorePages(cursors)
		a.post(func() {
			if a.moreSeq != seq {
				return
			}
			a.moreCursors = cursors
			a.moreLoading = false
			a.applyMorePages(pages)
		})
	}()
}

// maybeMoreNext loads the next pages once the aggregate page is scrolled near
// its end, or when the first batch is too short to scroll at all.
func (a *App) maybeMoreNext() {
	if a.moreLoading || a.moreAppending || !a.moreHasNext() {
		return
	}
	s := a.browseScroll
	near := s.MaxY > 48 && s.Y >= s.MaxY-320
	short := s.MaxY <= 1
	if near || short {
		a.moreNext()
	}
}

func (a *App) moreHasNext() bool {
	for _, c := range a.moreCursors {
		if !c.done {
			return true
		}
	}
	return false
}

// moreNext reads the next page of every category that still has one.
func (a *App) moreNext() {
	if a.moreLoading || a.moreAppending || !a.moreHasNext() {
		return
	}
	a.moreAppending = true
	cursors := append([]moreCursor(nil), a.moreCursors...)
	seq := a.moreSeq
	go func() {
		pages := fetchMorePages(cursors)
		a.post(func() {
			if a.moreSeq != seq {
				return
			}
			a.moreAppending = false
			a.applyMorePages(pages)
		})
	}()
}

// discoverMore finds the categories of every source that feed the rule's
// aggregate page, as cursors at page 1.
func discoverMore(sites []source.Source, rule homeRule, rules []homeRule) []moreCursor {
	found := make([][]moreCursor, len(sites))
	var wg sync.WaitGroup
	sem := make(chan struct{}, indexProbeParallel)
	for i, s := range sites {
		wg.Add(1)
		go func(i int, s source.Source) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), morePageTimeout)
			defer cancel()
			found[i] = moreCursorsFor(ctx, s, rule, rules)
		}(i, s)
	}
	wg.Wait()
	var out []moreCursor
	for _, cs := range found {
		out = append(out, cs...)
	}
	return out
}

// moreCursorsFor picks the categories of s that the rule's keywords match (at
// most moreFirstCats). The "latest" row falls back to the source's first
// category when none matches.
func moreCursorsFor(ctx context.Context, s source.Source, rule homeRule, rules []homeRule) []moreCursor {
	m := s.Meta()
	if m.Level == config.LevelNo {
		return nil
	}
	cats, _ := s.Categories(ctx)
	mk := func(catID string) moreCursor {
		return moreCursor{src: s, siteKey: m.Key, siteName: m.Name, voter: voterFor(m),
			catID: catID, next: 1, seen: map[string]bool{}}
	}
	var out []moreCursor
	for _, cat := range cats {
		if len(out) >= moreFirstCats {
			break
		}
		if slices.Contains(matchRules(cat.Name, rules), rule.ID) {
			out = append(out, mk(cat.ID))
		}
	}
	if len(out) == 0 && rule.ID == latestRuleID && len(cats) > 0 {
		out = append(out, mk(cats[0].ID))
	}
	return out
}

// fetchMorePages reads the next page of every unfinished cursor in parallel.
func fetchMorePages(cursors []moreCursor) []morePage {
	var mu sync.Mutex
	var out []morePage
	var wg sync.WaitGroup
	sem := make(chan struct{}, indexProbeParallel)
	for i, c := range cursors {
		if c.done {
			continue
		}
		wg.Add(1)
		go func(i int, c moreCursor) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), morePageTimeout)
			defer cancel()
			pg, err := c.src.List(ctx, c.catID, c.next)
			mu.Lock()
			out = append(out, morePage{index: i, page: pg, err: err})
			mu.Unlock()
		}(i, c)
	}
	wg.Wait()
	// Keep source order so the merge does not depend on which reply came first.
	sort.Slice(out, func(i, j int) bool { return out[i].index < out[j].index })
	return out
}

// applyMorePages advances the cursors and merges the new items into the page.
// A cursor stops on an error, an empty page, no next page, or a page with no
// item it has not already returned (a source that ignores the page number).
func (a *App) applyMorePages(pages []morePage) {
	var fresh []ballot
	for _, p := range pages {
		if p.index < 0 || p.index >= len(a.moreCursors) {
			continue
		}
		c := &a.moreCursors[p.index]
		if p.err != nil || len(p.page.Items) == 0 {
			c.done = true
			continue
		}
		added := 0
		for _, it := range p.page.Items {
			id := it.ID
			if id == "" {
				id = "title:" + it.Title
			}
			if c.seen[id] {
				continue
			}
			c.seen[id] = true
			added++
			if usableTitle(it.Title) {
				fresh = append(fresh, ballot{SiteKey: c.siteKey, SiteName: c.siteName,
					Voter: c.voter, Bucket: a.moreRow.Key, Item: it})
			}
		}
		c.next++
		if added == 0 || !p.page.HasNext() {
			c.done = true
		}
	}
	a.mergeMore(fresh)
}

// mergeMore adds ballots to the aggregate page. Titles already shown keep
// their place (a new source only joins their source list); new titles are
// ranked among themselves and appended, so the page never jumps under the
// user while more loads.
func (a *App) mergeMore(fresh []ballot) {
	a.moreBallots = append(a.moreBallots, fresh...)
	keys, by := groupBallots(a.moreBallots)
	shown := make(map[string]bool, len(a.moreKeys))
	for _, k := range a.moreKeys {
		shown[k] = true
	}
	var added []string
	for _, k := range keys {
		if !shown[k] {
			added = append(added, k)
		}
	}
	sortAggregate(added, by)
	a.moreKeys = append(a.moreKeys, added...)
	row, part := buildLane(a.moreRow.Key, a.moreRow.Name, a.moreKeys, by)
	a.moreRow = row
	a.moreLookup = part
}

// aggregateLane merges ballots by title with no row limit: covered titles
// first, then those listed by more sources.
func aggregateLane(key, name string, ballots []ballot) (homeLane, map[string][]titleHit) {
	keys, by := groupBallots(ballots)
	sortAggregate(keys, by)
	// Unlike home rows, the aggregate page keeps titles without a poster
	// (they sort last) instead of dropping them once enough covers exist.
	return buildLane(key, name, keys, by)
}

func sortAggregate(keys []string, by map[string]*laneCandidate) {
	sort.SliceStable(keys, func(i, j int) bool {
		ai, aj := by[keys[i]], by[keys[j]]
		pi := strings.TrimSpace(ai.item.Pic) != ""
		pj := strings.TrimSpace(aj.item.Pic) != ""
		if pi != pj {
			return pi
		}
		if len(ai.voters) != len(aj.voters) {
			return len(ai.voters) > len(aj.voters)
		}
		if ai.year != aj.year {
			return ai.year > aj.year
		}
		return ai.item.Title < aj.item.Title
	})
}

func (a *App) viewMore(c *ui.Context, th *ui.Theme) {
	a.maybeMoreNext()
	winW, _ := c.Size()
	inner := winW - sideWidth - pagePad*2
	cols := int((inner + 12) / (132 + 12))
	if cols < 2 {
		cols = 2
	}
	tracks := make([]ui.Track, cols)
	for i := range tracks {
		tracks[i] = ui.Fixed(132)
	}
	ui.Scroll(c).Grow(1).TrackScroll(&a.browseScroll).Children(func() {
		ui.Column(c).Padding(8, 0, 28, 0).Gap(18).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, "返回首页").Clicked() {
					a.goHome()
					return
				}
				ui.Text(c, a.moreRow.Name).FontSize(fontHero).Bold().SingleLine()
				if n := len(a.moreRow.Items); n > 0 {
					ui.Text(c, fmt.Sprintf("汇总自各片源，共 %d 部", n)).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
				}
			})
			if a.moreLoading {
				a.loadingRow(c, th, "正在汇总各片源的「"+a.moreRow.Name+"」…")
				return
			}
			if len(a.moreRow.Items) == 0 {
				a.emptyState(c, th, "没有内容", "各片源里没有找到这个分类的影片。",
					stateAction{label: "返回首页", primary: true, click: a.goHome})
				return
			}
			ui.Grid(c).ColumnTracks(tracks...).Gap(20).Children(func() {
				for _, it := range a.moreRow.Items {
					a.railCard(c, th, a.moreRow.Key, it)
				}
			})
			switch {
			case a.moreAppending:
				a.loadingRow(c, th, "正在加载更多…")
			case !a.moreHasNext():
				ui.Text(c, "已经到底了").FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
			}
		})
	})
}
