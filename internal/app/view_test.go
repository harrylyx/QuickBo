package app

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/mybox/internal/config"
	"github.com/harrylyx/mybox/internal/model"
	"github.com/harrylyx/mybox/internal/source"
)

func TestWelcomeMentionsEmptyShellAndJar(t *testing.T) {
	a := New()
	tt := ui.NewTester(a.View, 960, 640)
	if !tt.HasText("mybox") || !tt.HasText("不内置任何片源") {
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
