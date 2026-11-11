package app

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

func heroApp() *App {
	a := New()
	a.settingsPath = ""
	a.lib = &library{}
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
	lane := homeLane{Key: latestRuleID, Name: "最新影视"}
	for i, t := range []string{"甲片", "乙片", "无图片", "丙片", "丁片", "戊片", "己片"} {
		it := model.Item{ID: titlePrefix + t, Title: t}
		if t != "无图片" {
			it.Pic = "https://example.invalid/" + t
			a.rememberImage(it.Pic, poster(i), nil, color.RGBA{})
		}
		lane.Items = append(lane.Items, it)
	}
	a.home = []homeLane{{Key: "movie", Name: "电影", Items: lane.Items[:1]}, lane}
	return a
}

func TestHeroTakesTheFirstPostersOfLatest(t *testing.T) {
	a := heroApp()
	key, items := a.heroItems()
	if key != latestRuleID || len(items) != heroCount {
		t.Fatalf("lane %q, %d items", key, len(items))
	}
	for _, it := range items {
		if it.Title == "无图片" {
			t.Fatal("a title without a poster made it into the banner")
		}
	}
	a.heroStep(-1, len(items))
	if a.heroIdx != len(items)-1 {
		t.Fatalf("stepping back from the first wraps to the last, got %d", a.heroIdx)
	}
}

func TestHeroRotatesUnlessHovered(t *testing.T) {
	a := heroApp()
	tt := ui.NewTester(a.View, 1280, 820)
	if !tt.HasText("甲片") || !tt.HasText("查看详情") {
		t.Fatalf("banner missing: %q", tt.Texts())
	}
	a.heroAt = time.Now().Add(-heroEvery - time.Second)
	tt.Frame()
	if a.heroIdx != 1 {
		t.Fatalf("banner did not move on after %v: idx %d", heroEvery, a.heroIdx)
	}
	// With the pointer on it, it stays.
	r, _ := tt.Find("查看详情")
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	a.heroAt = time.Now().Add(-heroEvery - time.Second)
	tt.Frame()
	if a.heroIdx != 1 {
		t.Fatalf("banner moved on under the pointer: idx %d", a.heroIdx)
	}
}

func TestBlurImageKeepsAspectAndColor(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 300, 450))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 200, 40, 60, 255
	}
	blurred, tint := blurImage(img)
	if b := blurred.Bounds(); b.Dx() != backdropWidth || b.Dy() != backdropWidth*3/2 {
		t.Fatalf("size %v", b)
	}
	if tint != (color.RGBA{200, 40, 60, 255}) {
		t.Fatalf("tint %v", tint)
	}
	if c := blurred.RGBAAt(10, 10); c != (color.RGBA{200, 40, 60, 255}) {
		t.Fatalf("a flat image blurred to %v", c)
	}
}
