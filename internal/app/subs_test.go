package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

func TestSubsPersistAndRestore(t *testing.T) {
	a := cacheTestApp(t)
	a.subs = []subscription{
		{Ref: "https://a.example/tvbox.json", Name: "A站", Enabled: true},
		{Ref: "https://b.example/tvbox.json", Name: "B站", Enabled: false},
	}
	a.cfgRef = comboKey(a.activeRefs())
	a.saveSettings()

	data, err := os.ReadFile(a.settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var s savedSettings
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Subs) != 2 || s.Config != "https://a.example/tvbox.json" {
		t.Fatalf("saved %+v", s)
	}
	b := New()
	b.settingsPath = a.settingsPath
	refs := b.RestoreSettings()
	if len(refs) != 1 || refs[0] != "https://a.example/tvbox.json" {
		t.Fatalf("active refs %v", refs)
	}
	if len(b.subs) != 2 || !b.subs[0].Enabled || b.subs[1].Enabled {
		t.Fatalf("restored %+v", b.subs)
	}
}

// Settings written before the subscription list only have Config.
func TestLegacyConfigMigratesToSubs(t *testing.T) {
	a := cacheTestApp(t)
	if err := os.MkdirAll(filepath.Dir(a.settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.settingsPath, []byte(`{"config":"https://old.example/x.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	b := New()
	b.settingsPath = a.settingsPath
	refs := b.RestoreSettings()
	if len(refs) != 1 || refs[0] != "https://old.example/x.json" {
		t.Fatalf("refs %v", refs)
	}
	if len(b.subs) != 1 || !b.subs[0].Enabled {
		t.Fatalf("subs %+v", b.subs)
	}
}

func TestComboKeySingleRefMatchesLegacy(t *testing.T) {
	refs := normalizeRefs([]string{" https://a.example/x.json ", "", "https://a.example/x.json"})
	if len(refs) != 1 || comboKey(refs) != "https://a.example/x.json" {
		t.Fatalf("refs %v", refs)
	}
	if comboKey([]string{"one", "two"}) != "one\ntwo" {
		t.Fatal("multi-subscription key must differ")
	}
}

func subsTestApp() *App {
	a := New()
	a.settingsPath = ""
	a.fetchConfig = func(_ context.Context, ref string) (*config.Config, error) {
		switch ref {
		case "one":
			return &config.Config{
				SourceURL: "one",
				Sites:     config.SiteList{{Key: "s1", Name: "甲", Type: config.TypeJSON, API: "http://a/api"}},
			}, nil
		case "two":
			return &config.Config{
				SourceURL: "two",
				Sites:     config.SiteList{{Key: "s2", Name: "乙", Type: config.TypeJSON, API: "http://b/api"}},
			}, nil
		}
		return nil, errors.New(ref)
	}
	a.probeSite = func(_ context.Context, s source.Source) (model.Page, bool) {
		return model.Page{Items: []model.Item{{Title: s.Meta().Name}}}, true
	}
	return a
}

func TestLoadSubsMergesAndGroupsSources(t *testing.T) {
	a := subsTestApp()
	done := make(chan struct{})
	a.afterLoad = func() { close(done) }
	a.LoadSubs([]string{"one", "two"})
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("load did not finish")
	}
	if len(a.catalog.Sites) != 2 {
		t.Fatalf("sites %d", len(a.catalog.Sites))
	}
	if a.cfgRef != "one\ntwo" {
		t.Fatalf("cache key %q", a.cfgRef)
	}
	if a.groupOf["http://a/api"] != "one" || a.groupOf["http://b/api"] != "two" {
		t.Fatalf("groups %+v", a.groupOf)
	}
	groups := a.sideGroups()
	if len(groups) != 2 || groups[0].name != "one" || groups[1].name != "two" {
		t.Fatalf("groups %+v", groups)
	}
	for _, g := range groups {
		if len(g.sites) != 1 {
			t.Fatalf("group %q has %d sites", g.name, len(g.sites))
		}
	}
}

func TestAddSubscriptionKeepsTheExistingOnes(t *testing.T) {
	a := subsTestApp()
	done := make(chan struct{})
	a.afterLoad = func() { close(done) }
	a.subs = []subscription{{Ref: "one", Name: "一", Enabled: true}}
	a.AddSubscription("two")
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("load did not finish")
	}
	if len(a.subs) != 2 {
		t.Fatalf("subs %+v", a.subs)
	}
	refs := a.activeRefs()
	if len(refs) != 2 || refs[0] != "one" || refs[1] != "two" {
		t.Fatalf("refs %v", refs)
	}
	// Adding the same ref again must not duplicate it.
	a.upsertSub("two", true)
	if len(a.subs) != 2 || len(a.activeRefs()) != 2 {
		t.Fatalf("duplicate ref: %+v", a.subs)
	}
}

func TestDisableAllSubscriptionsReturnsToWelcome(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.subs = []subscription{{Ref: "one", Name: "一", Enabled: true}}
	a.catalog = &source.Catalog{}
	a.home = []homeLane{{Name: "旧数据"}}
	a.ballots = []ballot{{SiteKey: "s1"}}
	a.toggleSubscription("one")
	if a.subs[0].Enabled {
		t.Fatal("subscription still enabled")
	}
	if a.catalog != nil || len(a.home) != 0 || len(a.ballots) != 0 || a.screen != screenWelcome {
		t.Fatal("stale sources were kept after disabling everything")
	}
}

func TestSubsDialogListsSubscriptions(t *testing.T) {
	a := subsTestApp()
	a.catalog = &source.Catalog{}
	a.screen = screenHome
	a.subs = []subscription{
		{Ref: "one", Name: "甲订阅", Enabled: true},
		{Ref: "two", Name: "乙订阅", Enabled: false},
	}
	tt := ui.NewTester(a.View, 1200, 900)
	if err := tt.Click("订阅"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"订阅列表", "添加订阅", "甲订阅", "乙订阅", "启用"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q in %q", text, tt.Texts())
		}
	}
}

// Disabling the only subscription takes the synchronous path (nothing left
// to load), so the frame that handled the click must not keep rendering rows.
func TestSubsDialogToggleDoesNotCrashTheFrame(t *testing.T) {
	a := subsTestApp()
	a.catalog = &source.Catalog{}
	a.screen = screenHome
	a.subs = []subscription{{Ref: "one", Name: "一", Enabled: true}}
	tt := ui.NewTester(a.View, 1200, 900)
	if err := tt.Click("订阅"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("停用"); err != nil {
		t.Fatal(err)
	}
	if a.subs[0].Enabled {
		t.Fatal("toggle did not disable the first subscription")
	}
	if a.catalog != nil || a.screen != screenWelcome {
		t.Fatal("disabling everything should return to the welcome screen")
	}
	tt.Frame()
}

func TestSubsAndSettingsDialogsAreSeparate(t *testing.T) {
	a := subsTestApp()
	a.catalog = &source.Catalog{}
	a.screen = screenHome
	tt := ui.NewTester(a.View, 1200, 900)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"设置", "外观", "完成"} {
		if !tt.HasText(text) {
			t.Fatalf("settings dialog missing %q in %q", text, tt.Texts())
		}
	}
	if tt.HasText("订阅列表") || tt.HasText("添加订阅") {
		t.Fatalf("settings dialog contains subscription controls: %q", tt.Texts())
	}
	if err := tt.Click("完成"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("订阅"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"订阅列表", "添加订阅", "配置 URL"} {
		if !tt.HasText(text) {
			t.Fatalf("subscription dialog missing %q in %q", text, tt.Texts())
		}
	}
	if tt.HasText("优先使用 mpv 播放") {
		t.Fatalf("subscription dialog contains settings: %q", tt.Texts())
	}
}
