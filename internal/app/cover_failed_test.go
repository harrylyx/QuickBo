package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

// A title whose poster failed to download or decode is a blank card, so the
// home row and the banner must leave it out.
func TestHomeHidesCardsWhosePosterFailed(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
	a.failed["http://x/bad.jpg"] = true
	a.home = []homeLane{{Key: "c", Name: "演示源", Items: []model.Item{
		{ID: "1", Title: "加载失败片", Pic: "http://x/bad.jpg"},
		{ID: "2", Title: "正常影片", Pic: "http://x/ok.jpg"},
	}}}
	tt := ui.NewTester(a.View, 1200, 800)
	if tt.HasText("加载失败片") || !tt.HasText("正常影片") {
		t.Fatalf("texts %q", tt.Texts())
	}
}
