package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
)

func cacheTestApp(t *testing.T) *App {
	t.Helper()
	a := New()
	a.settingsPath = filepath.Join(t.TempDir(), "quickbo", "settings.json")
	return a
}

func TestCacheRoundTripAndInvalidation(t *testing.T) {
	a := cacheTestApp(t)
	ref := "https://example.com/tvbox.json"
	a.writeCache(&cacheData{
		Ref: ref,
		Sites: []cacheSite{{
			Key: "s1", Name: "甲",
			Cats: []cacheCat{{Name: "电影片", Items: []model.Item{{Title: "无间道"}}}},
		}},
		Lives: []string{"https://example.com/live.m3u"},
	})
	got := a.readCache(ref)
	if got == nil || got.Version != cacheVersion || got.SavedAt.IsZero() {
		t.Fatalf("read %+v", got)
	}
	if !got.hasSite("s1") || !got.hasLive("https://example.com/live.m3u") {
		t.Fatal("cached site or live list missing")
	}
	if balls := got.ballots(nil); len(balls) != 1 || balls[0].Cat != "电影片" || balls[0].Bucket != "" {
		t.Fatalf("ballots %+v", balls)
	}
	if a.readCache("https://other.example/tvbox.json") != nil {
		t.Fatal("cache must be keyed by the subscription ref")
	}
	b, err := os.ReadFile(a.cachePath())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	raw["version"] = cacheVersion + 1
	b, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.cachePath(), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if a.readCache(ref) != nil {
		t.Fatal("a version mismatch must invalidate the cache")
	}
	a.clearCache()
	if a.readCache(ref) != nil {
		t.Fatal("clearCache did not remove the file")
	}
}

func TestCacheDisabledWithoutSettingsPath(t *testing.T) {
	a := New()
	a.settingsPath = ""
	if a.cachePath() != "" {
		t.Fatal("cache must be disabled when no settings path is set")
	}
	a.writeCache(&cacheData{Ref: "r", Sites: []cacheSite{{Key: "x"}}})
	if a.readCache("r") != nil {
		t.Fatal("nothing should be persisted in tests")
	}
}

func TestCollectSourceKeepsEveryCategory(t *testing.T) {
	s := &yieldSrc{
		cats: []model.Category{
			{ID: "1", Name: "电影片"},
			{ID: "2", Name: "纪录片"},
			{ID: "3", Name: "首页"},
			{ID: "4", Name: "伦理片"},
		},
		pages: []model.Page{
			{Items: []model.Item{{Title: "无间道"}}},
			{Items: []model.Item{{Title: "地球脉动"}}},
			{}, {},
		},
	}
	cats := collectSource(context.Background(), s)
	if len(cats) != 2 || cats[0].Name != "电影片" || cats[1].Name != "纪录片" {
		t.Fatalf("cats %+v", cats)
	}
}

// A keyword added after collection must reach a category that the old rules
// never selected, without issuing another request.
func TestNewRuleKeywordUsesCachedCategory(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.ballots = (&cacheData{Sites: []cacheSite{{
		Key: "s1", Name: "甲", Voter: "site:s1",
		Cats: []cacheCat{{Name: "纪录片", Items: []model.Item{{Title: "地球脉动"}}}},
	}}}).ballots(nil)
	a.rebuildHome()
	if len(a.home) != 0 {
		t.Fatalf("default rules should not show 纪录片: %v", laneNames(a.home))
	}

	a.addRule()
	last := len(a.homeRules) - 1
	a.ruleNames[last] = "纪录片"
	a.ruleWords[last] = "纪录"
	a.saveRules()

	found := false
	for _, lane := range a.home {
		if lane.Name == "纪录片" && len(lane.Items) == 1 && lane.Items[0].Title == "地球脉动" {
			found = true
		}
	}
	if !found {
		t.Fatalf("new keyword did not reach the cached category: %v", laneNames(a.home))
	}
	if a.screen != screenHome || a.rulesDirty {
		t.Fatal("save must refresh the home and clear the dirty flag")
	}
}

func TestSaveRulesPersistsToSettings(t *testing.T) {
	a := cacheTestApp(t)
	a.bindRuleEdits()
	for i, rule := range a.homeRules {
		if rule.ID != "movie" {
			continue
		}
		a.ruleNames[i] = "大片"
		a.ruleWords[i] = "影片"
	}
	a.saveRules()
	b, err := os.ReadFile(a.settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var s savedSettings
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rule := range s.HomeRules {
		if rule.ID == "movie" && rule.Name == "大片" && len(rule.Keywords) == 1 && rule.Keywords[0] == "影片" {
			found = true
		}
	}
	if !found {
		t.Fatalf("saved rules %+v", s.HomeRules)
	}
}

// A cache hit must finish the load synchronously, without starting the
// per-source probe/collection goroutine.
func TestApplyRestoresFromCacheWithoutCollecting(t *testing.T) {
	a := cacheTestApp(t)
	ref := "https://example.com/tvbox.json"
	a.writeCache(&cacheData{Ref: ref, Sites: []cacheSite{{
		Key: "site1", Name: "甲",
		Cats: []cacheCat{{Name: "电影片", Items: []model.Item{{Title: "无间道"}}}},
	}}})

	done := false
	a.afterLoad = func() { done = true }
	cfg := &config.Config{Sites: []config.Site{{
		Key: "site1", Name: "甲", Type: config.TypeJSON, API: "https://example.com/api.php",
	}}}
	a.apply(cfg, ref)

	if !done || a.loading {
		t.Fatal("cache hit must complete the load without starting a collection")
	}
	if len(a.catalog.Sites) != 1 {
		t.Fatalf("restored sites %d", len(a.catalog.Sites))
	}
	if len(a.ballots) != 1 || a.ballots[0].Item.Title != "无间道" {
		t.Fatalf("ballots %+v", a.ballots)
	}
	if len(a.home) != 1 || a.home[0].Name != "电影" {
		t.Fatalf("home %v", laneNames(a.home))
	}
}
