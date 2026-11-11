package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

const cachedCfg = `{"sites":[{"key":"a","name":"甲","type":1,"api":"https://example.com/api.php"}]}`

func configCacheApp(t *testing.T) *App {
	a := New()
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	return a
}

// counting serves body (or a 500 while fail is set) and counts requests.
func counting(t *testing.T, body string, fail *atomic.Bool) (*httptest.Server, *atomic.Int32) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if fail != nil && fail.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestConfigCacheServesARestartFromDisk(t *testing.T) {
	srv, hits := counting(t, cachedCfg, nil)
	a := configCacheApp(t)
	ctx := withUserRefs(context.Background(), []string{srv.URL})
	if cfg, err := a.fetch(ctx, srv.URL); err != nil || len(cfg.Sites) != 1 {
		t.Fatalf("first fetch: %v %v", cfg, err)
	}
	// A new App is a new launch: it must read the config from disk.
	b := New()
	b.settingsPath = a.settingsPath
	cfg, err := b.fetch(ctx, srv.URL)
	if err != nil || len(cfg.Sites) != 1 || cfg.SourceURL != srv.URL {
		t.Fatalf("cached fetch: %+v %v", cfg, err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("restart hit the network: %d requests", n)
	}
}

func TestConfigCacheSkipsAFailingWarehouseEntry(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	srv, hits := counting(t, cachedCfg, &fail)
	a := configCacheApp(t)
	nested := withUserRefs(context.Background(), []string{"https://user.example/sub.json"})
	if _, err := a.fetch(nested, srv.URL); err == nil {
		t.Fatal("expected the failing entry to fail")
	}
	b := New()
	b.settingsPath = a.settingsPath
	if _, err := b.fetch(nested, srv.URL); err != errSkippedConfig {
		t.Fatalf("second launch: %v, want it skipped", err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("failing warehouse entry was retried: %d requests", n)
	}
	// The user's own subscription is always tried again.
	own := withUserRefs(context.Background(), []string{srv.URL})
	b.fetch(own, srv.URL)
	if n := hits.Load(); n != 2 {
		t.Fatalf("user subscription was not retried: %d requests", n)
	}
	// 重新采集 forgets the failure.
	b.clearConfigCache()
	fail.Store(false)
	if cfg, err := b.fetch(nested, srv.URL); err != nil || len(cfg.Sites) != 1 {
		t.Fatalf("after clearing: %v %v", cfg, err)
	}
}

func TestConfigCacheFallsBackToAnOldCopy(t *testing.T) {
	var fail atomic.Bool
	srv, _ := counting(t, cachedCfg, &fail)
	a := configCacheApp(t)
	ctx := withUserRefs(context.Background(), []string{srv.URL})
	if _, err := a.fetch(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	// Age the copy past cacheMaxAge, then take the host down.
	a.cfgCacheMu.Lock()
	e := a.cfgCache[srv.URL]
	e.SavedAt = time.Now().Add(-2 * cacheMaxAge)
	a.cfgCache[srv.URL] = e
	a.cfgCacheMu.Unlock()
	fail.Store(true)
	cfg, err := a.fetch(ctx, srv.URL)
	if err != nil || len(cfg.Sites) != 1 {
		t.Fatalf("stale fallback: %v %v", cfg, err)
	}
}
