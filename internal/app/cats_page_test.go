package app

import (
	"context"
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

func TestCollectKeepsEveryCategoryName(t *testing.T) {
	cats := make([]model.Category, 0, 21)
	for i := 0; i < 20; i++ {
		cats = append(cats, model.Category{ID: fmt.Sprint(i), Name: fmt.Sprintf("分类%02d", i)})
	}
	cats = append(cats, model.Category{ID: "x", Name: "伦理片"})
	s := &yieldSrc{cats: cats, pages: []model.Page{{Items: []model.Item{{Title: "有片"}}}}}
	listed, names := collectSourceWith(context.Background(), s, nil)
	if len(listed) != 1 {
		t.Fatalf("only the capped lists are fetched, got %d", len(listed))
	}
	if len(names) != 20 {
		t.Fatalf("every non-junk name should be kept, got %d", len(names))
	}
	for _, n := range names {
		if n.Name == "伦理片" {
			t.Fatal("junk category was recorded")
		}
	}
}

func TestCategoryDirectoryMergesRulesAndSharedNames(t *testing.T) {
	a := New()
	a.catFacts = []catFact{
		{SiteKey: "a", Name: "电影片", ID: "1"},
		{SiteKey: "b", Name: "电影", ID: "1"},
		{SiteKey: "a", Name: "综艺", ID: "2"},
		{SiteKey: "b", Name: "综艺", ID: "9"},
		{SiteKey: "a", Name: "冷门", ID: "3"},
		{SiteKey: "c", Name: "伦理片", ID: "4"},
	}
	dir := a.categoryDirectory()
	var movie, variety aggEntry
	for _, e := range dir {
		switch e.Name {
		case "电影":
			movie = e
		case "综艺":
			variety = e
		case "冷门", "伦理片":
			t.Fatalf("directory included %q", e.Name)
		}
	}
	if movie.Key != "movie" || movie.Sources != 2 {
		t.Fatalf("movie %+v", movie)
	}
	if variety.Key != "agg:综艺" || variety.Sources != 2 {
		t.Fatalf("variety %+v", variety)
	}
	movieAt, varietyAt := -1, -1
	for i, e := range dir {
		if e.Name == "电影" {
			movieAt = i
		}
		if e.Name == "综艺" {
			varietyAt = i
		}
	}
	if movieAt < 0 || varietyAt < 0 || movieAt > varietyAt {
		t.Fatalf("rules must come before leftover names: %+v", dir)
	}
}

func TestOpenMovieCategoryUsesTheMovieRule(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.catFacts = []catFact{
		{SiteKey: "c", Name: "电影片", ID: "1"},
		{SiteKey: "c", Name: "综艺", ID: "2"},
	}
	a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
	tt := ui.NewTester(a.View, 1200, 800)
	if err := tt.Click("分类"); err != nil {
		t.Fatal(err)
	}
	if a.screen != screenCats {
		t.Fatalf("screen %q", a.screen)
	}
	if !tt.HasText("电影 · 1 个源") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("电影 · 1 个源"); err != nil {
		t.Fatal(err)
	}
	if a.screen != screenMore || a.moreRow.Key != "movie" || a.moreRow.Name != "电影" || a.aggExtra != nil {
		t.Fatalf("screen %q row %+v extra %+v", a.screen, a.moreRow, a.aggExtra)
	}
}

func TestOpenSharedNameUsesATemporaryRule(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.screen = screenCats
	a.openCategory("agg:综艺", "综艺")
	if a.screen != screenMore || a.moreFrom != screenCats || a.aggExtra == nil || len(a.aggExtra.Keywords) != 1 || a.aggExtra.Keywords[0] != "综艺" {
		t.Fatalf("screen %q from %q extra %+v", a.screen, a.moreFrom, a.aggExtra)
	}
	found := false
	for _, rule := range a.rulesForMore(*a.aggExtra) {
		if rule.ID == "agg:综艺" {
			found = true
		}
	}
	if !found {
		t.Fatal("the temporary rule was not passed to the aggregate lookup")
	}
}
