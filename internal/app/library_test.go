package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/harrylyx/QuickBo/internal/model"
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
