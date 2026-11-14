package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/source"
)

// storageApp opens the settings modal over a home that needs no network.
func storageApp(t *testing.T) *App {
	t.Helper()
	a := New()
	a.settingsPath = filepath.Join(t.TempDir(), "quickbo", "settings.json")
	a.catalog = &source.Catalog{}
	a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
	a.openSettings()
	return a
}

func writeCacheFiles(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	names := append([]string{"cache.json", "configs.json"}, aRecordNames()...)
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{"x":1}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// aRecordNames are the record file names, for tests that write them.
func aRecordNames() []string {
	return []string{"dead.json", "parse_rank.json", "sub_tombstones.json"}
}

func TestClearEachCacheFromSettings(t *testing.T) {
	a := storageApp(t)
	dir := filepath.Dir(a.settingsPath)
	writeCacheFiles(t, dir)
	// The user's own data sits beside the caches and must survive.
	settings := a.settingsPath
	library := filepath.Join(dir, "library.json")
	for _, p := range []string{settings, library} {
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Memory caches too, which the poster row must drop.
	a.posters = map[string]*ui.Bitmap{"https://x/p.jpg": {}}
	a.failed = map[string]bool{"https://x/p.jpg": true}
	a.parseRank = map[string]parseRankEntry{"https://x/parse": {}}

	tt := ui.NewTester(a.View, 1200, 900)
	tt.Frame()
	for _, want := range []string{"存储", "图片缓存", "1 张", "首页采集", "订阅解析", "临时记录"} {
		if !tt.HasText(want) {
			t.Fatalf("settings missing %s: %q", want, tt.Texts())
		}
	}

	// The first row is the poster cache: a clear asks twice, and 取消
	// undoes it.
	if err := tt.Click("清除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.cacheConfirm != "posters" {
		t.Fatalf("clearing did not ask twice: %q", a.cacheConfirm)
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.cacheConfirm != "" || len(a.posters) != 1 {
		t.Fatalf("cancel did not undo: %q %d", a.cacheConfirm, len(a.posters))
	}
	if err := tt.Click("清除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("确认清除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.posters) != 0 || len(a.failed) != 0 {
		t.Fatalf("poster cache survived: %d %d", len(a.posters), len(a.failed))
	}
	if a.cacheConfirm != "" {
		t.Fatalf("confirm row did not settle: %q", a.cacheConfirm)
	}
	if !tt.HasText("0 张") {
		t.Fatalf("poster count not reset: %q", tt.Texts())
	}

	// The disk rows drop exactly their own files.
	a.clearCache()
	a.clearConfigCache()
	a.clearRecords()
	for _, name := range append([]string{"cache.json", "configs.json"}, aRecordNames()...) {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still on disk", name)
		}
	}
	if a.parseRank != nil {
		t.Fatal("parse record in memory survived")
	}
	for _, p := range []string{settings, library} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("user data did not survive: %v", err)
		}
	}
	// The rows read back as empty.
	tt.Frame()
	if !tt.HasText("0 B") {
		t.Fatalf("sizes not reset: %q", tt.Texts())
	}
}

func TestHumanBytes(t *testing.T) {
	for _, c := range [][2]string{
		{humanBytes(0), "0 B"},
		{humanBytes(512), "512 B"},
		{humanBytes(4<<10 + 512), "4.5 KB"},
		{humanBytes(3<<20 + 500<<10), "3.5 MB"},
		{humanBytes(5 << 30), "5.0 GB"},
	} {
		if c[0] != c[1] {
			t.Errorf("humanBytes: got %s, want %s", c[0], c[1])
		}
	}
}
