package app

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	a.saveRules()
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

// Clicking a button changes the rule list in the middle of a frame. The rest
// of that frame must not index draft buffers that no longer match homeRules.
func TestRuleButtonsDoNotCrashTheFrame(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{}
	a.screen = screenRules
	a.sideSel = "rules"
	a.homeRules = []homeRule{{ID: "custom-1", Name: "我的", Keywords: []string{"纪录"}}}
	a.resetRuleEdits()
	tt := ui.NewTester(a.View, 1200, 800)

	if err := tt.Click("添加"); err != nil {
		t.Fatal(err)
	}
	if len(a.homeRules) != 2 {
		t.Fatalf("after add: %+v", a.homeRules)
	}
	tt.Frame()

	if err := tt.Click("恢复默认"); err != nil {
		t.Fatal(err)
	}
	if len(a.homeRules) != len(defaultHomeRules()) || a.homeRules[0].ID != "hot" {
		t.Fatalf("rules not reset: %+v", a.homeRules)
	}
	if len(a.ruleNames) != len(a.homeRules) || len(a.ruleWords) != len(a.homeRules) ||
		len(a.ruleFocus) != len(a.homeRules)*2 {
		t.Fatalf("drafts out of sync: names=%d words=%d focus=%d",
			len(a.ruleNames), len(a.ruleWords), len(a.ruleFocus))
	}
	if a.ruleNames[0] != "最热影视" {
		t.Fatalf("draft shows stale text: %q", a.ruleNames[0])
	}
	tt.Frame()
	if !tt.HasText("恢复默认") {
		t.Fatal("rules screen did not render after reset")
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if a.screen != screenHome || a.rulesDirty {
		t.Fatal("save must apply the drafts and refresh the home")
	}
	tt.Frame()
}

// An explicit --config must not skip the saved homepage rules and preferences.
func TestRestoreSettingsWithExplicitConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	data, err := json.Marshal(savedSettings{
		Config:         "https://example.com/tvbox.json",
		PreferExternal: true,
		Light:          true,
		HomeRules:      []homeRule{{ID: "custom-7", Name: "纪录片", Keywords: []string{"纪录"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	a := New()
	a.settingsPath = path
	refs := a.RestoreSettings()
	if len(refs) != 1 || refs[0] != "https://example.com/tvbox.json" {
		t.Fatalf("refs %v", refs)
	}
	if len(a.homeRules) != 1 || a.homeRules[0].Name != "纪录片" {
		t.Fatalf("rules %+v", a.homeRules)
	}
	if !a.preferExternal || !a.light {
		t.Fatal("saved preferences were not restored")
	}
	if a.catalog != nil || a.cfgRef != "" {
		t.Fatal("RestoreSettings must not load a subscription")
	}
	if len(a.ruleNames) != 1 || a.ruleNames[0] != "纪录片" {
		t.Fatalf("drafts %v", a.ruleNames)
	}
}

func TestDeleteRuleKeepsOtherDrafts(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.bindRuleEdits()
	a.ruleWords[1] = "未提交的关键字"
	a.deleteRule(0)
	if len(a.ruleNames) != len(a.homeRules) || len(a.ruleWords) != len(a.homeRules) ||
		len(a.ruleFocus) != len(a.homeRules)*2 {
		t.Fatalf("drafts out of sync: rules=%d names=%d words=%d focus=%d",
			len(a.homeRules), len(a.ruleNames), len(a.ruleWords), len(a.ruleFocus))
	}
	if a.ruleWords[0] != "未提交的关键字" {
		t.Fatalf("uncommitted edit lost: %q", a.ruleWords[0])
	}
}

func TestRuleRenameChangesHome(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.ballots = []ballot{
		{SiteKey: "a", Voter: "site:a", Bucket: "movie", Cat: "电影片", Item: model.Item{Title: "无间道"}},
	}
	a.rebuildHome()
	a.bindRuleEdits()
	for i, rule := range a.homeRules {
		if rule.ID != "movie" {
			continue
		}
		a.ruleNames[i] = "大片"
		a.saveRules()
		break
	}
	if len(a.home) != 1 || a.home[0].Name != "大片" {
		t.Fatalf("home %v", laneNames(a.home))
	}
}
