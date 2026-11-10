package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

func TestMergeConfigsDropsDupesAndKeepsOrder(t *testing.T) {
	a := &config.Config{Sites: config.SiteList{
		{Key: "a", Name: "甲", API: "http://x/a", Ext: "1"},
		{Key: "a", Name: "甲重复", API: "http://x/a", Ext: "1"},
		{Key: "b", Name: "乙", API: "http://x/b", Ext: "2"},
	}, Lives: config.LiveList{{Name: "直播", URL: "http://live/1"}}}
	b := &config.Config{Sites: config.SiteList{
		{Key: "a", Name: "另一份", API: "http://x/a", Ext: "9"},
		{Key: "", Name: "丙", API: "http://x/c"},
	}, Lives: config.LiveList{{Name: "又一份", URL: "http://live/1"}, {Name: "二", URL: "http://live/2"}}}
	got := mergeConfigs([]*config.Config{a, nil, b})
	want := []string{"a", "b", "a#2", "丙"}
	if len(got.Sites) != len(want) {
		t.Fatalf("sites %d, want %d", len(got.Sites), len(want))
	}
	for i, key := range want {
		if got.Sites[i].Key != key {
			t.Fatalf("site %d key %q, want %q", i, got.Sites[i].Key, key)
		}
	}
	if len(got.Lives) != 2 || got.Lives[0].URL != "http://live/1" || got.Lives[1].URL != "http://live/2" {
		t.Fatalf("lives %+v", got.Lives)
	}
}

func TestSourceSampleTriesLaterCategories(t *testing.T) {
	later := &yieldSrc{
		cats: []model.Category{{ID: "rec"}, {ID: "movie"}},
		pages: []model.Page{
			{},
			{Items: []model.Item{{Title: "有"}}},
		},
	}
	page, ok := sourceSample(context.Background(), later)
	if !ok || len(page.Items) == 0 || page.Items[0].Title != "有" {
		t.Fatal("second category has a title, source should be kept")
	}
	tooLate := &yieldSrc{
		cats: []model.Category{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}},
		pages: []model.Page{
			{},
			{},
			{},
			{Items: []model.Item{{Title: "太靠后"}}},
		},
	}
	if _, ok := sourceSample(context.Background(), tooLate); ok {
		t.Fatal("only the fourth category listed anything; the probe stops at three")
	}
	broken := &yieldSrc{err: errors.New("eof")}
	if _, ok := sourceSample(context.Background(), broken); ok {
		t.Fatal("a source that errors with no categories is a failure")
	}
	dead := &yieldSrc{
		level: config.LevelNo,
		cats:  []model.Category{{ID: "a"}},
		pages: []model.Page{{Items: []model.Item{{Title: "x"}}}},
	}
	if _, ok := sourceSample(context.Background(), dead); ok {
		t.Fatal("unsupported sources are not probed")
	}
}

func TestFetchEntriesSkipsFailuresAndOpensOneNestedIndex(t *testing.T) {
	a := &App{fetchConfig: func(_ context.Context, ref string) (*config.Config, error) {
		switch ref {
		case "bad":
			return nil, errors.New("403")
		case "nested":
			return &config.Config{URLs: []config.NamedURL{{URL: "leaf"}, {URL: "bad"}}}, nil
		case "leaf":
			return &config.Config{Sites: config.SiteList{{Key: "leaf", Name: "叶子", API: "http://leaf"}}}, nil
		case "direct":
			return &config.Config{Sites: config.SiteList{{Key: "direct", Name: "直接", API: "http://direct"}}}, nil
		default:
			return nil, errors.New(ref)
		}
	}}
	got := a.fetchEntries(context.Background(), []string{"bad", "direct", "nested", "direct"}, 0, &refSeen{m: map[string]struct{}{}})
	if len(got) != 2 {
		t.Fatalf("configs %d, want direct then leaf", len(got))
	}
	if got[0].Sites[0].Key != "direct" || got[1].Sites[0].Key != "leaf" {
		t.Fatalf("order %+v %+v", got[0].Sites, got[1].Sites)
	}
}

func TestLoadIndexHidesEmptyAndFailed(t *testing.T) {
	a := &App{
		fetchConfig: func(_ context.Context, ref string) (*config.Config, error) {
			switch ref {
			case "index":
				return &config.Config{URLs: []config.NamedURL{
					{URL: "bad"},
					{URL: "good"},
					{URL: "empty"},
				}}, nil
			case "bad":
				return nil, errors.New("timeout")
			case "good":
				return &config.Config{Sites: config.SiteList{
					{Key: "ok", Name: "可用", Type: config.TypeJSON, API: "http://127.0.0.1:1/ok"},
					{Key: "jar", Name: "不支持", Type: config.TypeSpider, API: "csp_Other"},
				}}, nil
			case "empty":
				return &config.Config{Sites: config.SiteList{
					{Key: "blank", Name: "空的", Type: config.TypeJSON, API: "http://127.0.0.1:1/blank"},
				}}, nil
			default:
				return nil, errors.New(ref)
			}
		},
		probeSite: func(_ context.Context, s source.Source) (model.Page, bool) {
			if s.Meta().Name != "可用" {
				return model.Page{}, false
			}
			return model.Page{Items: []model.Item{{ID: "1", Title: "有结果"}}}, true
		},
	}
	done := make(chan struct{})
	a.afterLoad = func() { close(done) }
	a.Load("index")
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("index load did not finish")
	}
	if a.loading {
		t.Fatal("still loading")
	}
	if len(a.catalog.Sites) != 1 || a.catalog.Sites[0].Meta().Name != "可用" {
		names := make([]string, 0, len(a.catalog.Sites))
		for _, s := range a.catalog.Sites {
			names = append(names, s.Meta().Name)
		}
		t.Fatalf("sidebar sources %v", names)
	}
	if a.screen != screenHome || a.sideSel != "home" || a.siteKey != "" {
		t.Fatalf("screen %s side %s site %s", a.screen, a.sideSel, a.siteKey)
	}
	if len(a.home) != 1 || a.home[0].Name != "最热影视" || a.home[0].Items[0].Title != "有结果" {
		t.Fatalf("home %+v", a.home)
	}
}

type yieldSrc struct {
	level config.Level
	cats  []model.Category
	pages []model.Page
	err   error
	n     int
}

func (s yieldSrc) Meta() source.Meta { return source.Meta{Level: s.level} }

func (s yieldSrc) Categories(context.Context) ([]model.Category, error) {
	if s.err != nil && len(s.cats) == 0 {
		return nil, s.err
	}
	return s.cats, nil
}

func (s *yieldSrc) List(_ context.Context, cat string, _ int) (model.Page, error) {
	for i, c := range s.cats {
		if c.ID == cat {
			if i < len(s.pages) {
				return s.pages[i], s.err
			}
			return model.Page{}, s.err
		}
	}
	return model.Page{}, s.err
}

func (s yieldSrc) Search(context.Context, string, int) (model.Page, error) {
	return model.Page{}, nil
}
func (s yieldSrc) Detail(context.Context, string) (model.Detail, error) { return model.Detail{}, nil }
func (s yieldSrc) Resolve(context.Context, string, model.Episode) (model.Playable, error) {
	return model.Playable{}, nil
}
