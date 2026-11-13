package app

import (
	"testing"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
)

// Editing an existing row after the home was restored from cache must change
// the home at once: the new name shows and the new keywords re-pick categories.
func TestEditRuleAfterCacheRestoreChangesHome(t *testing.T) {
	a := cacheTestApp(t)
	ref := "https://example.com/tvbox.json"
	a.writeCache(&cacheData{Ref: ref, Sites: []cacheSite{{
		Key: "site1", Name: "甲",
		Cats: []cacheCat{
			{Name: "电影片", Items: []model.Item{{Title: "无间道", Pic: "http://x/a.jpg"}}},
			{Name: "纪录片", Items: []model.Item{{Title: "地球脉动", Pic: "http://x/b.jpg"}}},
		},
	}}})
	cfg := &config.Config{Sites: []config.Site{{
		Key: "site1", Name: "甲", Type: config.TypeJSON, API: "https://example.com/api.php",
	}}}
	a.apply(cfg, ref)
	if len(a.home) != 1 || a.home[0].Name != "电影" {
		t.Fatalf("before edit: %v", laneNames(a.home))
	}

	a.bindRuleEdits()
	for i, rule := range a.homeRules {
		if rule.ID == "movie" {
			a.ruleNames[i] = "大片"
			a.ruleWords[i] = "纪录"
		}
	}
	a.saveRules()

	if len(a.home) != 1 || a.home[0].Name != "大片" || a.home[0].Items[0].Title != "地球脉动" {
		t.Fatalf("after edit: %v", laneNames(a.home))
	}
}
