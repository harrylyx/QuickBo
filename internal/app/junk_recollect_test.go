package app

import (
	"testing"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
)

func TestNeedsJunkCollectWhenCacheLacksGenre(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.cfg = &config.Config{}
	a.homeRules = append(defaultHomeRules(), homeRule{ID: "custom-1", Name: "伦理", Keywords: []string{"伦理"}})
	a.ballots = (&cacheData{Sites: []cacheSite{{
		Key: "s1", Name: "甲",
		Cats: []cacheCat{{Name: "电影片", Items: []model.Item{{Title: "无间道"}}}},
	}}}).ballots(nil)
	if !a.needsJunkCollect() {
		t.Fatal("cache has no 伦理 category, a recollect is needed")
	}
	a.ballots = append(a.ballots, ballot{SiteKey: "s1", Cat: "伦理片", Item: model.Item{Title: "伦理影片"}})
	if a.needsJunkCollect() {
		t.Fatal("cache already holds a matching category")
	}
}
