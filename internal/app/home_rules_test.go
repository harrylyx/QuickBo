package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

func TestRulesRenameReorderAndCustomKeyword(t *testing.T) {
	rules := defaultHomeRules()
	for i := range rules {
		if rules[i].ID != "movie" {
			continue
		}
		movie := rules[i]
		movie.Name = "大片"
		rules = append(rules[:i], rules[i+1:]...)
		rules = append([]homeRule{movie}, rules...)
		break
	}
	rules = append(rules, homeRule{ID: "custom-1", Name: "纪录片", Keywords: []string{"纪录", "记录"}})
	ballots := []ballot{
		{SiteKey: "a", Cat: "热门电影", Item: model.Item{Title: "庆余年"}},
		{SiteKey: "a", Cat: "电影片", Item: model.Item{Title: "无间道"}},
		{SiteKey: "b", Cat: "记录片", Item: model.Item{Title: "地球脉动"}},
		{SiteKey: "a", Cat: "热映", Item: model.Item{Title: "不该出现"}},
	}
	lanes, lookup := tallyRules(ballots, rules)
	if len(lanes) < 2 || lanes[0].Name != "大片" {
		t.Fatalf("lanes %v", laneNames(lanes))
	}
	if lanes[0].Items[0].Title != "庆余年" && lanes[0].Items[0].Title != "无间道" {
		t.Fatalf("movie row %+v", lanes[0].Items)
	}
	foundHot, foundDoc := false, false
	for _, lane := range lanes {
		if lane.Name == "最热影视" && len(lane.Items) == 1 && lane.Items[0].Title == "庆余年" {
			foundHot = true
		}
		if lane.Name == "纪录片" && len(lane.Items) == 1 && lane.Items[0].Title == "地球脉动" {
			foundDoc = true
		}
		for _, item := range lane.Items {
			if item.Title == "不该出现" {
				t.Fatal("热映 is not a hot keyword")
			}
		}
	}
	if !foundHot || !foundDoc {
		t.Fatalf("lanes %v", laneNames(lanes))
	}
	if len(lookup[normTitle("庆余年")]) != 1 {
		t.Fatal("title stays reachable")
	}
	if got := matchRules("短剧", []homeRule{{ID: "custom-2", Name: "短剧", Keywords: []string{"短剧"}}}); len(got) != 1 {
		t.Fatalf("custom row can name a junk genre, got %v", got)
	}
}

func TestMoveRuleChangesHomeOrder(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.ballots = []ballot{
		{SiteKey: "a", Bucket: "hot", Item: model.Item{Title: "庆余年"}},
		{SiteKey: "a", Bucket: "movie", Item: model.Item{Title: "无间道"}},
	}
	a.rebuildHome()
	if a.home[0].Name != "最热影视" {
		t.Fatalf("before %v", laneNames(a.home))
	}
	movie := -1
	for i, rule := range a.homeRules {
		if rule.ID == "movie" {
			movie = i
		}
	}
	a.ruleNames = []string{"大片"}
	a.ruleWords = []string{"电影"}
	// bind fills the rest; commit the movie row after placing drafts at its index
	a.bindRuleEdits()
	a.ruleNames[movie] = "大片"
	a.moveRule(movie, -movie)
	if a.homeRules[0].ID != "movie" || a.homeRules[0].Name != "大片" {
		t.Fatalf("rules %+v", a.homeRules[0])
	}
	if a.home[0].Name != "大片" {
		t.Fatalf("home %v", laneNames(a.home))
	}
}

func TestRuleScreenListsEditableRows(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{}
	a.screen = screenRules
	a.sideSel = "rules"
	tt := ui.NewTester(a.View, 1200, 800)
	for _, text := range []string{"首页分类", "添加", "上移", "下移"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %s in %q", text, tt.Texts())
		}
	}
	if err := tt.Click("添加"); err != nil {
		t.Fatal(err)
	}
	last := a.homeRules[len(a.homeRules)-1]
	if last.Name != "新分类" || !strings.HasPrefix(last.ID, "custom-") {
		t.Fatalf("last %+v", last)
	}
}
