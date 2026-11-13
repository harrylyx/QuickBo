package app

import (
	"fmt"
	"testing"

	"github.com/harrylyx/QuickBo/internal/model"
)

func coverBallots(withPic, withoutPic int) []ballot {
	var out []ballot
	for i := 0; i < withoutPic; i++ {
		out = append(out, ballot{SiteKey: "s", SiteName: "甲", Bucket: "movie",
			Item: model.Item{ID: fmt.Sprint("n", i), Title: fmt.Sprintf("无图影片%c", 'A'+i)}})
	}
	for i := 0; i < withPic; i++ {
		out = append(out, ballot{SiteKey: "s", SiteName: "甲", Bucket: "movie",
			Item: model.Item{ID: fmt.Sprint("p", i), Title: fmt.Sprintf("有图影片%c", 'A'+i), Pic: "http://x/p.jpg"}})
	}
	return out
}

// With enough posters, titles without one stay out of the row entirely.
func TestLaneDropsCoverlessWhenEnoughCovers(t *testing.T) {
	lane, _ := votedLane("movie", "电影", coverBallots(8, 3))
	if len(lane.Items) != 8 {
		t.Fatalf("items %d, want 8", len(lane.Items))
	}
	for _, it := range lane.Items {
		if it.Pic == "" {
			t.Fatalf("coverless title in row: %q", it.Title)
		}
	}
}

// With few posters, coverless titles only fill the row, after the covered.
func TestLaneFillsWithCoverlessLast(t *testing.T) {
	lane, _ := votedLane("movie", "电影", coverBallots(2, 5))
	if len(lane.Items) != railMinCovers {
		t.Fatalf("items %d, want %d", len(lane.Items), railMinCovers)
	}
	if lane.Items[0].Pic == "" || lane.Items[1].Pic == "" || lane.Items[2].Pic != "" {
		t.Fatalf("covered titles must come first: %+v", lane.Items)
	}
}
