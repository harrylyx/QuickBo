package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

func TestLibraryWatchAndFav(t *testing.T) {
	dir := t.TempDir()
	lib := loadLibrary(dir)
	lib.upsertWatch(watchItem{Key: "s\n1", Title: "海边的风", URL: "http://cdn/a.mp4", Episode: "正片"})
	lib.touchProgress("s\n1", 12, 90)
	lib.toggleFav(favItem{Key: "s\n1", Title: "海边的风"})
	again := loadLibrary(dir)
	got := again.watches()
	if len(got) != 1 || got[0].Title != "海边的风" || got[0].Position != 12 || got[0].URL == "" {
		t.Fatalf("%+v", got)
	}
	if !again.hasFav("s\n1") {
		t.Fatal("missing favorite")
	}
	if again.toggleFav(favItem{Key: "s\n1", Title: "海边的风"}) {
		t.Fatal("toggle should remove")
	}
	if time.Since(got[0].Updated) > time.Minute {
		t.Fatal("stamp")
	}
	if _, err := filepath.Abs(again.path); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryRemovesWatchAndFav(t *testing.T) {
	dir := t.TempDir()
	lib := loadLibrary(dir)
	lib.upsertWatch(watchItem{Key: "s\n1", Title: "海边的风", URL: "http://cdn/a.mp4"})
	lib.toggleFav(favItem{Key: "s\n1", Title: "海边的风"})
	lib.removeWatch("s\n1")
	lib.removeFav("s\n1")
	lib.removeWatch("missing")
	lib.removeFav("missing")

	again := loadLibrary(dir)
	if len(again.watches()) != 0 || len(again.favs()) != 0 {
		t.Fatalf("watch %+v favs %+v", again.watches(), again.favs())
	}
}

// Resuming the second history entry while the first title's detail is still
// loaded must play, record and track the second one.
func TestHistoryResumesTheClickedEntry(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.lib = loadLibrary(t.TempDir())
	first := watchItem{Key: mediaKey("s", "1"), SiteKey: "s", VodID: "1", Title: "第一部", URL: "http://cdn/1.m3u8", Episode: "第3集", Position: 30}
	second := watchItem{Key: mediaKey("s", "2"), SiteKey: "s", VodID: "2", Title: "第二部", URL: "http://cdn/2.m3u8", Episode: "第7集", Position: 70}
	a.lib.upsertWatch(second)
	a.lib.upsertWatch(first)
	a.detail = &model.Detail{ID: "1", Title: "第一部"} // left over from the last watch

	a.prepareHistory(second)
	a.playTitle, a.playURL = second.Title, second.URL
	a.rememberCurrent(second.URL)

	if a.watchKey() != second.Key || a.resumeAt != 70 {
		t.Fatalf("progress must track the clicked entry: key %q resume %v", a.watchKey(), a.resumeAt)
	}
	for _, w := range a.lib.watches() {
		switch w.Key {
		case first.Key:
			if w.URL != first.URL || w.Title != "第一部" {
				t.Fatalf("the first entry must stay untouched: %+v", w)
			}
		case second.Key:
			if w.URL != second.URL || w.Episode != "第7集" {
				t.Fatalf("the clicked entry must keep its episode: %+v", w)
			}
		}
	}
}

// Back returns to the screen the player or the detail page was opened from,
// whatever path got there.
func TestBackReturnsWhereItCameFrom(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.lib = loadLibrary(t.TempDir())
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	w := watchItem{Key: mediaKey("c", "1"), SiteKey: "c", VodID: "1", Title: "某片", Episode: "第2集"}

	// History page → player (no address: it fails) → retry → back.
	a.sideSel, a.lastSide, a.screen = "history", "history", screenHistory
	a.playHistory(w)
	if _, ep := a.overlayTitle(); ep != "第2集" {
		t.Fatalf("the resumed episode must show: %q", ep)
	}
	a.retryPlay()
	a.screen = screenPlay
	a.back()
	if a.screen != screenHistory {
		t.Fatalf("history → play → retry → back went to %q", a.screen)
	}

	// Home's 继续观看 → player → back, with a source's detail left over.
	a.sideSel, a.screen = "home", screenHome
	a.detail = &model.Detail{ID: "9", Title: "别的片", Lines: []model.Line{{Name: "l", Episodes: []model.Episode{{Name: "e"}}}}}
	a.playHistory(w)
	a.back()
	if a.screen != screenHome {
		t.Fatalf("continue watching → back went to %q", a.screen)
	}

	// Favorites → detail → back.
	a.sideSel, a.screen = "fav", screenFav
	a.openDetail(model.Item{ID: "1", Title: "某片"})
	waitDetail(t, a)
	a.back()
	if a.screen != screenFav {
		t.Fatalf("favorites → detail → back went to %q", a.screen)
	}

	// Source list → detail → episode → next episode → back → back.
	a.sideSel, a.siteKey, a.screen = "site:c", "c", screenBrowse
	a.openDetail(model.Item{ID: "1", Title: "某片"})
	waitDetail(t, a)
	a.detail = &model.Detail{ID: "1", Title: "某片", Lines: []model.Line{{Name: "l", Episodes: []model.Episode{{Name: "1"}, {Name: "2"}}}}}
	a.playEpisode("l", model.Episode{Name: "1"})
	waitPlayResolve(t, a)
	a.playEpisode("l", model.Episode{Name: "2"})
	waitPlayResolve(t, a)
	a.back()
	if a.screen != screenDetail {
		t.Fatalf("episode → back went to %q", a.screen)
	}
	a.back()
	if a.screen != screenBrowse {
		t.Fatalf("detail → back went to %q", a.screen)
	}
}

// Clicking through the window: the 继续观看 row on the home page and the
// history page both come back to where they were.
func TestHistoryCardsBackInTheWindow(t *testing.T) {
	for _, from := range []string{"home", "history"} {
		a := New()
		a.settingsPath = ""
		a.lib = loadLibrary(t.TempDir())
		a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
		a.lib.upsertWatch(watchItem{Key: mediaKey("c", "1"), SiteKey: "c", VodID: "1", Title: "某片", URL: "http://x.invalid/a.m3u8"})
		a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
		want := screenHome
		tt := ui.NewTester(a.View, 1200, 800)
		if from == "history" {
			if err := tt.Click("历史"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			want = screenHistory
		}
		if err := tt.Click("某片"); err != nil {
			t.Fatalf("%s: %v %q", from, err, tt.Texts())
		}
		tt.Frame()
		if a.screen != screenPlay {
			t.Fatalf("%s: card opened %q", from, a.screen)
		}
		if err := tt.Click("返回"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.screen != want {
			t.Fatalf("%s: back went to %q", from, a.screen)
		}
	}
}

// waitDetail waits for the detail load openDetail started. Without a window
// its result lands on the loading goroutine (post), so a test must let it
// finish before touching the app again.
func waitDetail(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.postMu.Lock()
		busy := a.detailLoading
		a.postMu.Unlock()
		if !busy {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("detail load did not finish")
}

// waitPlayResolve waits for the episode resolve playEpisode started, as
// waitDetail does for a detail.
func waitPlayResolve(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.postMu.Lock()
		busy := a.playLoading
		a.postMu.Unlock()
		if !busy {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("episode resolve did not finish")
}
