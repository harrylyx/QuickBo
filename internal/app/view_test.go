package app

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

func TestWelcomeMentionsEmptyShellAndJar(t *testing.T) {
	a := New()
	tt := ui.NewTester(a.View, 960, 640)
	if !tt.HasText("QuickBo") || !tt.HasText("不内置任何片源") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if !tt.HasText("不执行 jar") {
		t.Fatalf("missing jar note: %q", tt.Texts())
	}
	if err := tt.Click("添加配置"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("配置 URL") {
		t.Fatalf("dialog texts %q", tt.Texts())
	}
}

func TestUnsupportedJarRenders(t *testing.T) {
	a := New()
	a.catalog = &source.Catalog{Sites: []source.Source{deadFake{}}}
	a.siteKey = "jar"
	a.sideSel = "site:jar"
	a.screen = screenUnsupported
	tt := ui.NewTester(a.View, 1100, 720)
	joined := stringsJoin(tt.Texts())
	if !contains(joined, "不执行 jar") || !contains(joined, "蜘蛛") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestHomeShowsEverySource(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}, otherFake{}}}
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
	a.home = []homeLane{
		{Key: "c", Name: "演示源", Items: []model.Item{{ID: "1", Title: "示例影片"}}},
		{Key: "d", Name: "另一源", Items: []model.Item{{ID: "2", Title: "另一部"}}},
	}
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("示例影片") || !tt.HasText("另一部") || !tt.HasText("全部片源") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.RightClick("演示源"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if len(menu) != 1 || menu[0] != "删除" {
		t.Fatalf("menu %v", menu)
	}
	if err := tt.ChooseMenuItem("删除"); err != nil {
		t.Fatal(err)
	}
	if len(a.catalog.Sites) != 1 || a.catalog.Sites[0].Meta().Name != "另一源" {
		t.Fatalf("sites left: %d", len(a.catalog.Sites))
	}
	if len(a.home) != 1 || a.home[0].Name != "另一源" {
		t.Fatalf("rails %+v", a.home)
	}
}

func TestBrowseShowsPosterCard(t *testing.T) {
	a := New()
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.siteKey = "c"
	a.sideSel = "site:c"
	a.screen = screenBrowse
	a.cats = []model.Category{{ID: "", Name: "最新"}, {ID: "1", Name: "电影"}}
	a.catID = ""
	a.page = model.Page{Items: []model.Item{{ID: "1", Title: "示例影片", Remarks: "HD"}}, Page: 1, PageCount: 1}
	tt := ui.NewTester(a.View, 1100, 720)
	if !tt.HasText("示例影片") || !tt.HasText("演示源") || !tt.HasText("电影") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestDetailCanSortOffersByClarityOrDelay(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{}
	a.screen = screenDetail
	a.detailTitle = "庆余年"
	a.stats = map[string]sourceStat{"slow": {Delay: 800}, "fast": {Delay: 40}}
	a.offers = []playOffer{
		{SiteKey: "slow", SiteName: "慢源", Clarity: 50},
		{SiteKey: "fast", SiteName: "快源", Clarity: 10},
	}
	a.sortOffers()
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("4K") || !tt.HasText("其他") || !tt.HasText("慢源") || !tt.HasText("快源 · 40 ms") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if a.offers[0].SiteKey != "slow" {
		t.Fatalf("sharper group should come first, got %s", a.offers[0].SiteKey)
	}
}

type deadFake struct{}

func (deadFake) Meta() source.Meta {
	return source.Meta{Key: "jar", Name: "蜘蛛", API: "csp_AppYS", Kind: config.KindJar, Level: config.LevelNo, Reason: "JAR/DEX 爬虫（csp_AppYS）本版不执行 jar。"}
}
func (deadFake) Categories(context.Context) ([]model.Category, error) {
	return nil, errFake("不执行 jar")
}
func (deadFake) List(context.Context, string, int) (model.Page, error) {
	return model.Page{}, errFake("不执行 jar")
}
func (deadFake) Search(context.Context, string, int) (model.Page, error) {
	return model.Page{}, errFake("不执行 jar")
}
func (deadFake) Detail(context.Context, string) (model.Detail, error) {
	return model.Detail{}, errFake("不执行 jar")
}
func (deadFake) Resolve(context.Context, string, model.Episode) (model.Playable, error) {
	return model.Playable{}, errFake("不执行 jar")
}

type cmsFake struct{}

func (cmsFake) Meta() source.Meta {
	return source.Meta{Key: "c", Name: "演示源", API: "https://example.invalid/vod", Kind: config.KindCMSJSON, Level: config.LevelYes}
}
func (cmsFake) Categories(context.Context) ([]model.Category, error) {
	return []model.Category{{ID: "", Name: "最新"}}, nil
}
func (cmsFake) List(context.Context, string, int) (model.Page, error) { return model.Page{}, nil }
func (cmsFake) Search(context.Context, string, int) (model.Page, error) {
	return model.Page{}, nil
}
func (cmsFake) Detail(context.Context, string) (model.Detail, error) { return model.Detail{}, nil }
func (cmsFake) Resolve(context.Context, string, model.Episode) (model.Playable, error) {
	return model.Playable{}, nil
}

type otherFake struct{}

func (otherFake) Meta() source.Meta {
	return source.Meta{Key: "d", Name: "另一源", API: "https://example.invalid/other", Kind: config.KindCMSJSON, Level: config.LevelYes}
}
func (otherFake) Categories(context.Context) ([]model.Category, error) { return nil, nil }
func (otherFake) List(context.Context, string, int) (model.Page, error) {
	return model.Page{}, nil
}
func (otherFake) Search(context.Context, string, int) (model.Page, error) {
	return model.Page{}, nil
}
func (otherFake) Detail(context.Context, string) (model.Detail, error) { return model.Detail{}, nil }
func (otherFake) Resolve(context.Context, string, model.Episode) (model.Playable, error) {
	return model.Playable{}, nil
}

type errFake string

func (e errFake) Error() string { return string(e) }

func stringsJoin(ss []string) string {
	out := ""
	for _, s := range ss {
		out += s + "\n"
	}
	return out
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
