package app

import (
	"path/filepath"
	"testing"
	"time"
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
