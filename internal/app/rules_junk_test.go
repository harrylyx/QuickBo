package app

import (
	"testing"

	"github.com/harrylyx/QuickBo/internal/model"
)

// A keyword the user writes into a rule must be honored even when it names a
// category the built-in filter treats as junk (the user asked for it).
func TestEditedBuiltinRuleMatchesJunkCategory(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.ballots = (&cacheData{Sites: []cacheSite{{
		Key: "s1", Name: "甲", Voter: "site:s1",
		Cats: []cacheCat{{Name: "短剧", Items: []model.Item{{Title: "重生之路", Pic: "http://x/a.jpg"}}}},
	}}}).ballots(nil)
	a.bindRuleEdits()
	for i, rule := range a.homeRules {
		if rule.ID == "movie" {
			a.ruleNames[i] = "短剧"
			a.ruleWords[i] = "短剧"
		}
	}
	a.saveRules()
	if len(a.home) != 1 || a.home[0].Name != "短剧" {
		t.Fatalf("home %v", laneNames(a.home))
	}
}
