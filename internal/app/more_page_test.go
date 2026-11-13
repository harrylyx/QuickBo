package app

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

// The aggregate page merges the same title across sources, is not capped at
// the home row's size, and puts titles with a poster first.
func TestAggregateLaneMergesSourcesWithoutRowLimit(t *testing.T) {
	var balls []ballot
	for i := 0; i < 40; i++ {
		balls = append(balls, ballot{SiteKey: "a", SiteName: "甲", Bucket: "movie",
			Item: model.Item{ID: itoa(i), Title: "有图影片" + itoa(i), Pic: "http://x/" + itoa(i) + ".jpg"}})
	}
	// 同一部片在另一个源里也有，应合并成一条并记录两个来源。
	balls = append(balls, ballot{SiteKey: "b", SiteName: "乙", Bucket: "movie",
		Item: model.Item{ID: "z", Title: "有图影片0", Pic: "http://x/0.jpg"}})
	balls = append(balls, ballot{SiteKey: "b", SiteName: "乙", Bucket: "movie",
		Item: model.Item{ID: "n", Title: "无图影片"}})

	lane, part := aggregateLane("movie", "电影", balls)
	if len(lane.Items) <= hotLaneLimit {
		t.Fatalf("aggregate page must not be capped at the home row size: %d", len(lane.Items))
	}
	if hits := part[normTitle("有图影片0")]; len(hits) != 2 {
		t.Fatalf("same title must merge across sources: %+v", hits)
	}
	if last := lane.Items[len(lane.Items)-1]; last.Pic != "" {
		t.Fatalf("title without a poster must come last: %+v", last)
	}
}

// Opening a title from the aggregate page and going back returns to it.
func TestMorePageBackReturnsFromDetail(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.screen = screenMore
	a.moreRow = homeLane{Key: "movie", Name: "电影"}
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.sideSel = "home"
	a.openItem("c", model.Item{ID: "1", Title: "某片"})
	waitDetail(t, a)
	if a.detailFrom != screenMore {
		t.Fatalf("opening from the aggregate page must be remembered: %q", a.detailFrom)
	}
	a.screen = screenDetail
	a.back()
	if a.screen != screenMore {
		t.Fatalf("screen %q", a.screen)
	}
}

// pagedSrc serves numbered pages per category. A category with sticky set
// returns its first page for every page number, like sources that ignore it.
type pagedSrc struct {
	key    string
	cats   []model.Category
	pages  map[string][]model.Page
	sticky bool
	mu     sync.Mutex
	asked  []string
}

func (s *pagedSrc) Meta() source.Meta { return source.Meta{Key: s.key, Name: s.key} }
func (s *pagedSrc) Categories(context.Context) ([]model.Category, error) {
	return s.cats, nil
}
func (s *pagedSrc) List(_ context.Context, cat string, page int) (model.Page, error) {
	s.mu.Lock()
	s.asked = append(s.asked, cat+"#"+itoa(page))
	s.mu.Unlock()
	pages := s.pages[cat]
	if s.sticky && len(pages) > 0 {
		return pages[0], nil
	}
	if page < 1 || page > len(pages) {
		return model.Page{}, nil
	}
	return pages[page-1], nil
}
func (s *pagedSrc) Search(context.Context, string, int) (model.Page, error) {
	return model.Page{}, nil
}
func (s *pagedSrc) Detail(context.Context, string) (model.Detail, error) {
	return model.Detail{}, nil
}
func (s *pagedSrc) Resolve(context.Context, string, model.Episode) (model.Playable, error) {
	return model.Playable{}, nil
}

func pageOf(n, of int, titles ...string) model.Page {
	pg := model.Page{Page: n, PageCount: of}
	for _, t := range titles {
		pg.Items = append(pg.Items, model.Item{ID: t, Title: t, Pic: "http://x/" + t + ".jpg"})
	}
	return pg
}

func moreTitles(a *App) []string {
	var out []string
	for _, it := range a.moreRow.Items {
		out = append(out, it.Title)
	}
	return out
}

// waitMore waits for the background load started by openMore / moreNext.
func waitMore(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.postMu.Lock()
		busy := a.moreLoading || a.moreAppending
		a.postMu.Unlock()
		if !busy {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("aggregate page load did not finish")
}

// The aggregate page reads page 1 first and the next pages on demand,
// appending new titles after the ones already shown, until every matching
// category runs out.
func TestMorePageLoadsNextPagesOnDemand(t *testing.T) {
	a := New()
	a.settingsPath = ""
	src := &pagedSrc{key: "a",
		cats: []model.Category{{ID: "m", Name: "电影"}, {ID: "t", Name: "电视剧"}},
		pages: map[string][]model.Page{
			"m": {pageOf(1, 2, "乙片", "甲片"), pageOf(2, 2, "丙片")},
			"t": {pageOf(1, 1, "剧集一")},
		}}
	a.catalog = &source.Catalog{Sites: []source.Source{src}}

	a.openMore(homeLane{Key: "movie", Name: "电影"})
	waitMore(t, a)
	if got := moreTitles(a); !slices.Equal(got, []string{"乙片", "甲片"}) && !slices.Equal(got, []string{"甲片", "乙片"}) {
		t.Fatalf("first batch must be page 1 of the matching category only: %v", got)
	}
	if !a.moreHasNext() {
		t.Fatal("page 2 is still to come")
	}
	first := moreTitles(a)

	a.moreNext()
	waitMore(t, a)
	if got := moreTitles(a); !slices.Equal(got, append(first, "丙片")) {
		t.Fatalf("page 2 must be appended after the shown titles: %v", got)
	}
	if a.moreHasNext() {
		t.Fatal("the last page was read; nothing more must load")
	}
	if slices.Contains(src.asked, "t#1") {
		t.Fatalf("a category the rule does not match must not be read: %v", src.asked)
	}
}

// Titles already shown keep their place when a later page brings the same
// title from another source; it only joins their source list.
func TestMoreMergeKeepsShownOrder(t *testing.T) {
	a := New()
	a.moreRow = homeLane{Key: "movie", Name: "电影"}
	a.mergeMore([]ballot{
		{SiteKey: "a", SiteName: "甲", Bucket: "movie", Item: model.Item{ID: "1", Title: "无图片"}},
		{SiteKey: "a", SiteName: "甲", Bucket: "movie", Item: model.Item{ID: "2", Title: "有图片", Pic: "http://x/2.jpg"}},
	})
	if got := moreTitles(a); !slices.Equal(got, []string{"有图片", "无图片"}) {
		t.Fatalf("first batch ranks covers first: %v", got)
	}
	a.mergeMore([]ballot{
		{SiteKey: "b", SiteName: "乙", Bucket: "movie", Item: model.Item{ID: "9", Title: "新片", Pic: "http://x/9.jpg"}},
		{SiteKey: "b", SiteName: "乙", Bucket: "movie", Item: model.Item{ID: "8", Title: "无图片", Pic: "http://x/8.jpg"}},
	})
	if got := moreTitles(a); !slices.Equal(got, []string{"有图片", "无图片", "新片"}) {
		t.Fatalf("shown titles must not move, new ones append: %v", got)
	}
	if hits := a.moreLookup[normTitle("无图片")]; len(hits) != 2 {
		t.Fatalf("the later source must join the shown title: %+v", hits)
	}
}

// A source that ignores the page number keeps returning page 1; the cursor
// must stop instead of loading the same titles forever.
func TestMoreCursorStopsOnRepeatedPage(t *testing.T) {
	a := New()
	a.settingsPath = ""
	src := &pagedSrc{key: "a", sticky: true,
		cats:  []model.Category{{ID: "m", Name: "电影"}},
		pages: map[string][]model.Page{"m": {pageOf(1, 9, "甲片", "乙片")}}}
	a.catalog = &source.Catalog{Sites: []source.Source{src}}
	a.openMore(homeLane{Key: "movie", Name: "电影"})
	waitMore(t, a)
	a.moreNext()
	waitMore(t, a)
	if a.moreHasNext() {
		t.Fatal("a repeated page must end the category")
	}
	if n := len(a.moreRow.Items); n != 2 {
		t.Fatalf("repeated titles must not be added twice: %d", n)
	}
}

// The "latest" row reads a source's first category when none matches.
func TestMoreCursorsLatestFallsBackToFirstCategory(t *testing.T) {
	src := &pagedSrc{key: "a", cats: []model.Category{{ID: "x", Name: "精选"}, {ID: "y", Name: "专题"}}}
	rules := defaultHomeRules()
	var latest homeRule
	for _, r := range rules {
		if r.ID == latestRuleID {
			latest = r
		}
	}
	if latest.ID == "" {
		t.Skip("no latest row among the default rules")
	}
	got := moreCursorsFor(context.Background(), src, latest, rules)
	if len(got) != 1 || got[0].catID != "x" {
		t.Fatalf("latest must fall back to the first category: %+v", got)
	}
}
