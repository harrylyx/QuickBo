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
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

// morePages is how many list pages are read from each matching category of
// each source for the aggregate page.
const morePages = 2

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
// list under that row's keywords, merged across sources.
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
	a.moreLoading = true
	a.fromMore = false
	a.browseScroll.Y = 0
	sites := append([]source.Source(nil), a.catalog.Sites...)
	rules := cloneRules(a.activeRules())
	a.moreSeq++
	seq := a.moreSeq
	go func() {
		found := collectMore(sites, rule, rules)
		a.post(func() {
			if a.moreSeq != seq {
				return
			}
			row, part := aggregateLane(lane.Key, lane.Name, found)
			a.moreRow = row
			a.moreLookup = part
			a.moreLoading = false
		})
	}()
}

func collectMore(sites []source.Source, rule homeRule, rules []homeRule) []ballot {
	var mu sync.Mutex
	var out []ballot
	var wg sync.WaitGroup
	sem := make(chan struct{}, indexProbeParallel)
	for _, s := range sites {
		wg.Add(1)
		go func(s source.Source) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			got := moreFromSite(ctx, s, rule, rules)
			mu.Lock()
			out = append(out, got...)
			mu.Unlock()
		}(s)
	}
	wg.Wait()
	return out
}

// moreFromSite reads the first pages of every category of s that the rule's
// keywords match (at most four categories).
func moreFromSite(ctx context.Context, s source.Source, rule homeRule, rules []homeRule) []ballot {
	m := s.Meta()
	voter := voterFor(m)
	var out []ballot
	add := func(items []model.Item) {
		for _, it := range items {
			if usableTitle(it.Title) {
				out = append(out, ballot{SiteKey: m.Key, SiteName: m.Name, Voter: voter, Bucket: rule.ID, Item: it})
			}
		}
	}
	cats, _ := s.Categories(ctx)
	picked := 0
	for _, cat := range cats {
		if picked >= 4 || ctx.Err() != nil {
			break
		}
		if !slices.Contains(matchRules(cat.Name, rules), rule.ID) {
			continue
		}
		picked++
		for p := 1; p <= morePages; p++ {
			pg, err := s.List(ctx, cat.ID, p)
			if err != nil || len(pg.Items) == 0 {
				break
			}
			add(pg.Items)
			if !pg.HasNext() {
				break
			}
		}
	}
	if rule.ID == latestRuleID && picked == 0 {
		if pg, ok := sourceSample(ctx, s); ok {
			add(pg.Items)
		}
	}
	return out
}

// aggregateLane merges ballots by title with no row limit: covered titles
// first, then those listed by more sources.
func aggregateLane(key, name string, ballots []ballot) (homeLane, map[string][]titleHit) {
	keys, by := groupBallots(ballots)
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
	// Unlike home rows, the aggregate page keeps titles without a poster
	// (they sort last) instead of dropping them once enough covers exist.
	return buildLane(key, name, keys, by)
}

func (a *App) viewMore(c *ui.Context, th *ui.Theme) {
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
		})
	})
}
