package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

// A warehouse subscription can bring five figures of sites. Every probe used
// to publish its own result (copying the whole sidebar and re-tallying the
// whole home), so the UI thread did quadratic work and the app hung. These
// tests keep both the collection and the cache-restore paths bounded while
// still showing every site in the sidebar.
const hugeSiteCount = 12000

func hugeSites(n int) []source.Source {
	out := make([]source.Source, n)
	for i := range out {
		out[i] = &yieldSrc{
			level: config.LevelYes,
			key:   fmt.Sprintf("s%d", i),
			name:  fmt.Sprintf("源%d", i),
		}
	}
	return out
}

// The windowed app tallies off the UI thread; headless tests normally take
// the synchronous path. This hook checks that a background result is still
// published and that a later rebuild wins over a stale one.
func TestBackgroundHomeTallyPublishesLatestResult(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.asyncHomeTally = true
	a.ballots = []ballot{
		{SiteKey: "a", SiteName: "甲", Bucket: hotLaneID, Item: model.Item{Title: "大片", Pic: "http://x/p.jpg"}},
		{SiteKey: "b", SiteName: "乙", Cat: "电影片", Item: model.Item{Title: "无间道"}},
	}
	a.scheduleHome()
	waitHome := func(want string) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for {
			found := false
			a.post(func() {
				for _, lane := range a.home {
					for _, it := range lane.Items {
						if it.Title == want {
							found = true
						}
					}
				}
			})
			if found {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("background tally never published %q", want)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
	waitHome("大片")

	// Queue a newer tally while the first may still be running; only the
	// latest generation may land on the home screen.
	a.ballots = []ballot{{SiteKey: "c", Bucket: hotLaneID, Item: model.Item{Title: "新片", Pic: "http://x/q.jpg"}}}
	a.scheduleHome()
	waitHome("新片")
	time.Sleep(60 * time.Millisecond)
	has := func(want string) bool {
		found := false
		a.post(func() {
			for _, lane := range a.home {
				for _, it := range lane.Items {
					if it.Title == want {
						found = true
					}
				}
			}
		})
		return found
	}
	if !has("新片") || has("大片") {
		t.Fatal("a stale background tally overwrote the newest home")
	}

	// A synchronous rebuild (rule edit, delete) also wins over results in
	// flight from the collection pass.
	a.ballots = []ballot{{SiteKey: "d", Bucket: hotLaneID, Item: model.Item{Title: "终片", Pic: "http://x/r.jpg"}}}
	a.rebuildHome()
	time.Sleep(60 * time.Millisecond)
	if !has("终片") || has("新片") {
		t.Fatal("a background tally overwrote a newer synchronous rebuild")
	}
}

func assertBoundedBallots(t *testing.T, ballots []ballot) {
	t.Helper()
	if len(ballots) > homeBallotLimit {
		t.Fatalf("ballots %d exceed the home limit %d", len(ballots), homeBallotLimit)
	}
	sites := map[string]bool{}
	for _, b := range ballots {
		sites[b.SiteKey] = true
	}
	if len(sites) > homeSourceLimit {
		t.Fatalf("%d sites feed the home, limit is %d", len(sites), homeSourceLimit)
	}
}

func TestHugeSubscriptionCollectionStaysBounded(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.cfgRef = "huge"
	a.probeSite = func(_ context.Context, _ source.Source) (model.Page, bool) {
		return model.Page{Items: []model.Item{{ID: "1", Title: "大片", Pic: "http://x/p.jpg"}}}, true
	}
	a.catalog = &source.Catalog{Sites: hugeSites(hugeSiteCount)}
	done := make(chan struct{})
	a.afterLoad = func() { close(done) }
	start := time.Now()
	a.keepWorking(context.Background(), a.catalog)
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("collection of 12k sources did not finish")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("collection took %v; quadratic publishing is back", elapsed)
	}
	if len(a.catalog.Sites) != hugeSiteCount {
		t.Fatalf("sidebar lost sources: %d", len(a.catalog.Sites))
	}
	if len(a.home) == 0 {
		t.Fatal("home was never tallied")
	}
	assertBoundedBallots(t, a.ballots)
}

func TestHugeCacheRestoreStaysBounded(t *testing.T) {
	a := New()
	a.settingsPath = ""
	c := &cacheData{Ref: "huge", Sites: make([]cacheSite, hugeSiteCount)}
	for i := 0; i < hugeSiteCount; i++ {
		key := fmt.Sprintf("s%d", i)
		// Sites past the home limit are cached by key only; they must still
		// come back into the sidebar.
		c.Sites[i] = cacheSite{Key: key, Name: fmt.Sprintf("源%d", i)}
		if i < homeSourceLimit {
			c.Sites[i].Hot = []model.Item{{Title: "大片", Pic: "http://x/p.jpg"}}
			c.Sites[i].Cats = []cacheCat{{Name: "电影片", Items: []model.Item{{Title: "无间道"}}}}
		}
	}
	start := time.Now()
	a.restoreFromCache(c, &source.Catalog{Sites: hugeSites(hugeSiteCount)})
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("cache restore took %v; linear lookups are missing", elapsed)
	}
	if len(a.catalog.Sites) != hugeSiteCount {
		t.Fatalf("restore dropped sites: %d", len(a.catalog.Sites))
	}
	if len(a.catFacts) > homeFactLimit {
		t.Fatalf("cat facts %d exceed %d", len(a.catFacts), homeFactLimit)
	}
	if len(a.home) == 0 {
		t.Fatal("cached home was not rebuilt")
	}
	assertBoundedBallots(t, a.ballots)
}
