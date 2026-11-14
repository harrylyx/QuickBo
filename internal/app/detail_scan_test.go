package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

func detailScanApp() *App {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.siteKey = "c"
	a.screen = screenDetail
	a.detailTitle = "示例影片"
	d := model.Detail{Title: "示例影片", Lines: []model.Line{{
		Name: "线路", Episodes: []model.Episode{{Name: "1", URL: "https://x/1.m3u8"}},
	}}}
	a.detail = &d
	return a
}

func TestDetailPlayIsDisabledUntilOfferScanEnds(t *testing.T) {
	a := detailScanApp()
	a.offersScanning = true
	a.offers = []playOffer{{SiteKey: "c", SiteName: "演示源", Checking: true}}
	tt := ui.NewTester(a.View, 1200, 900)
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"正在检测片源…", "检测完成后自动选出最佳片源"} {
		if !strings.Contains(texts, want) {
			t.Fatalf("missing %q in %q", want, texts)
		}
	}
	// Texts also carries accessibility labels; the disabled pill's label
	// starts with "播放 1", so look for the enabled button's exact text.
	for _, s := range tt.Texts() {
		if s == "播放 1" {
			t.Fatalf("play button is still enabled: %q", texts)
		}
	}
	// Even if some other entry point reaches playEpisode, the guard holds.
	a.playEpisode("线路", a.detail.Lines[0].Episodes[0])
	if a.screen != screenDetail || a.player != nil {
		t.Fatal("playback started while the scan was running")
	}
	a.offersScanning = false
	tt.Frame()
	if !tt.HasText("播放 1") {
		t.Fatalf("play button did not come back: %q", tt.Texts())
	}
}

// The scan itself must own the lock: openTitle starts it, loadOffers clears
// it when the final pick is published. cmsFake's empty detail keeps the
// whole pass offline (no episodes to measure).
func TestOpenTitleLocksPlayUntilScanPublishes(t *testing.T) {
	a := detailScanApp()
	a.lookup = map[string][]titleHit{"t": {{SiteKey: "c", SiteName: "演示源", ItemID: "1", Title: "示例影片"}}}
	a.openTitle("t")
	if !a.offersBusy() {
		t.Fatal("scan did not lock playback")
	}
	a.playEpisode("线路", model.Episode{Name: "1", URL: "https://x/1.m3u8"})
	if a.screen != screenDetail {
		t.Fatal("playback started during the scan")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.postMu.Lock()
		busy := a.offersScanning
		a.postMu.Unlock()
		if !busy {
			if a.offersBusy() {
				t.Fatal("lock cleared but offers are still being checked")
			}
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("offer scan never finished")
}

// A plain detail page from a site list has no cross-source scan, so its
// play button stays available.
func TestPlainDetailPlayStaysEnabled(t *testing.T) {
	a := detailScanApp()
	tt := ui.NewTester(a.View, 1200, 900)
	if !tt.HasText("播放 1") {
		t.Fatalf("missing play button: %q", tt.Texts())
	}
	if a.offersBusy() {
		t.Fatal("plain detail page should not wait for a scan")
	}
}
