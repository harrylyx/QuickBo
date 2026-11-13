package app

import (
	"testing"

	"github.com/harrylyx/QuickBo/internal/model"
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
	a.openItem("movie", model.Item{ID: "1", Title: "某片"})
	if !a.fromMore {
		t.Fatal("opening from the aggregate page must be remembered")
	}
	a.screen = screenDetail
	a.back()
	if a.screen != screenMore || a.fromMore {
		t.Fatalf("screen %q fromMore %v", a.screen, a.fromMore)
	}
}
