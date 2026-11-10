package app

import (
	"strings"
	"testing"

	"github.com/harrylyx/QuickBo/internal/model"
)

func TestVoteRanksOverlapFirst(t *testing.T) {
	ballots := []ballot{
		{SiteKey: "a", SiteName: "甲", Bucket: "hot", Item: model.Item{ID: "1", Title: "庆余年 2", Year: "2024"}},
		{SiteKey: "b", SiteName: "乙", Bucket: "hot", Item: model.Item{ID: "9", Title: "庆余年2", Year: "2024"}},
		{SiteKey: "c", SiteName: "丙", Bucket: "hot", Item: model.Item{ID: "3", Title: "庆余年 2"}},
		{SiteKey: "a", SiteName: "甲", Bucket: "hot", Item: model.Item{ID: "2", Title: "独一份"}},
		{SiteKey: "b", SiteName: "乙", Bucket: "region:香港", Item: model.Item{ID: "4", Title: "无间道"}},
	}
	lanes, lookup := tally(ballots)
	if len(lanes) != 2 || lanes[0].Name != "最热影视" || lanes[1].Name != "香港" {
		t.Fatalf("lanes %+v", laneNames(lanes))
	}
	if lanes[0].Items[0].Title != "庆余年 2" {
		t.Fatalf("first hot title %q", lanes[0].Items[0].Title)
	}
	if lanes[0].Items[1].Title != "独一份" {
		t.Fatalf("second hot title %q", lanes[0].Items[1].Title)
	}
	hits := lookup[normTitle("庆余年 2")]
	if len(hits) != 3 {
		t.Fatalf("hits %d", len(hits))
	}
}

func TestBucketsFollowSourceCategories(t *testing.T) {
	if got := bucketsFor("最新热播"); len(got) != 2 || got[0] != "hot" || got[1] != "new" {
		t.Fatalf("最新热播 -> %v", got)
	}
	if got := bucketsFor("香港剧"); len(got) != 1 || got[0] != "region:香港" {
		t.Fatalf("香港剧 -> %v", got)
	}
	if got := bucketsFor("国产动漫"); len(got) != 2 || got[0] != "anime" || got[1] != "region:大陆" {
		t.Fatalf("国产动漫 -> %v", got)
	}
	if bucketsFor("电影") == nil || bucketsFor("电影")[0] != "movie" {
		t.Fatalf("电影 -> %v", bucketsFor("电影"))
	}
	if bucketsFor("推荐") != nil || bucketsFor("伦理片") != nil || bucketsFor("短剧") != nil || bucketsFor("热映") != nil {
		t.Fatal("front page, junk, and non-chart words must not vote")
	}
	if got := bucketsFor("电影片"); len(got) != 1 || got[0] != "movie" {
		t.Fatalf("电影片 -> %v", got)
	}
	if got := bucketsFor("连续剧"); len(got) != 1 || got[0] != "tv" {
		t.Fatalf("连续剧 -> %v", got)
	}
	cats := []model.Category{
		{ID: "r", Name: "推荐"},
		{ID: "", Name: "最新"},
		{ID: "1", Name: "电影片"},
		{ID: "2", Name: "连续剧"},
		{ID: "30", Name: "伦理片"},
		{ID: "38", Name: "短剧"},
		{ID: "12", Name: "国产剧"},
		{ID: "13", Name: "香港剧"},
	}
	var names []string
	for _, p := range selectBallotCats(cats) {
		names = append(names, p.cat.Name)
	}
	want := []string{"最新", "电影片", "连续剧", "国产剧", "香港剧"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("picked %v", names)
	}
}

func TestMirrorsShareOneVote(t *testing.T) {
	ballots := []ballot{
		{SiteKey: "a", Voter: "host:cj.example", Bucket: "hot", Item: model.Item{Title: "庆余年"}},
		{SiteKey: "b", Voter: "host:cj.example", Bucket: "hot", Item: model.Item{Title: "庆余年"}},
		{SiteKey: "c", Voter: "host:bf.example", Bucket: "hot", Item: model.Item{Title: "折月亮"}},
		{SiteKey: "d", Voter: "host:lz.example", Bucket: "hot", Item: model.Item{Title: "折月亮"}},
		{SiteKey: "a", Bucket: "hot", Item: model.Item{Title: "首页"}},
	}
	lanes, lookup := tally(ballots)
	if len(lanes) != 1 || lanes[0].Items[0].Title != "折月亮" {
		t.Fatalf("hot %+v", lanes)
	}
	if len(lookup[normTitle("庆余年")]) != 2 {
		t.Fatal("both mirrors stay available on the title")
	}
}

func TestOffersSortByDelayFirst(t *testing.T) {
	if clarityScore("4K") <= clarityScore("1080P 蓝光") || clarityScore("1080P") <= clarityScore("720P") {
		t.Fatal("sharper copy should score higher")
	}
	if clarityScore("枪版 TS") >= clarityScore("HD") {
		t.Fatal("cam copy should rank last")
	}
	a := &App{stats: map[string]sourceStat{"slow": {Delay: 800}, "fast": {Delay: 40}, "sharp": {Delay: 40}}}
	a.offers = []playOffer{
		{SiteKey: "slow", SiteName: "慢", Clarity: 50},
		{SiteKey: "fast", SiteName: "快", Clarity: 10},
		{SiteKey: "sharp", SiteName: "清", Clarity: 50},
		{SiteKey: "blur", SiteName: "糊", Clarity: 40},
	}
	a.offerPick = 0
	a.sortOffers()
	if a.offers[0].SiteKey != "sharp" || a.offers[1].SiteKey != "fast" || a.offers[2].SiteKey != "slow" || a.offers[3].SiteKey != "blur" {
		t.Fatalf("delay order %s %s %s %s", a.offers[0].SiteKey, a.offers[1].SiteKey, a.offers[2].SiteKey, a.offers[3].SiteKey)
	}
	if a.offerPick != 2 {
		t.Fatalf("selected source moved to %d", a.offerPick)
	}
	a.offerBy = offerByClarity
	a.sortOffers()
	if a.offers[0].SiteKey != "sharp" || a.offers[1].SiteKey != "slow" || a.offers[2].SiteKey != "blur" || a.offers[3].SiteKey != "fast" {
		t.Fatalf("clarity order %s %s %s %s", a.offers[0].SiteKey, a.offers[1].SiteKey, a.offers[2].SiteKey, a.offers[3].SiteKey)
	}
	if a.offerPick != 1 {
		t.Fatalf("selected source moved to %d", a.offerPick)
	}
}

func laneNames(lanes []homeLane) []string {
	out := make([]string, len(lanes))
	for i, lane := range lanes {
		out[i] = lane.Name
	}
	return out
}
