package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/harrylyx/QuickBo/internal/config"
)

// The config cache keeps the raw text of every subscription and warehouse
// entry fetched, so a start within cacheMaxAge of the last one reads them from
// disk instead of the network. Without it every launch re-downloaded each
// config before even looking at the collection cache, and a warehouse with
// one dead host waited out its whole fetch timeout every time.
//
// Fetch failures are remembered too, but only for nested warehouse entries:
// those are skipped for cacheMaxAge rather than retried on every launch. A
// subscription the user added is always tried, so a network blip at launch
// cannot blank the app for a day. recollect clears the cache.

// configCacheKeep is how long an entry not fetched again stays on disk, as a
// fallback for when the network fails.
const configCacheKeep = 7 * 24 * time.Hour

type configEntry struct {
	Body     []byte    `json:"body,omitempty"`
	Source   string    `json:"source,omitempty"`
	SavedAt  time.Time `json:"savedAt,omitzero"`
	FailedAt time.Time `json:"failedAt,omitzero"`
	Err      string    `json:"err,omitempty"`
}

func (a *App) configCachePath() string {
	if a.settingsPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(a.settingsPath), "configs.json")
}

// configCache returns the in-memory cache, reading it from disk once. The
// caller holds cfgCacheMu.
func (a *App) configCache() map[string]configEntry {
	if a.cfgCache != nil {
		return a.cfgCache
	}
	a.cfgCache = map[string]configEntry{}
	path := a.configCachePath()
	if path == "" {
		return a.cfgCache
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return a.cfgCache
	}
	var m map[string]configEntry
	if json.Unmarshal(b, &m) != nil {
		return a.cfgCache
	}
	for ref, e := range m {
		if time.Since(e.SavedAt) < configCacheKeep || time.Since(e.FailedAt) < cacheMaxAge {
			a.cfgCache[ref] = e
		}
	}
	return a.cfgCache
}

func (a *App) lookupConfig(ref string) (configEntry, bool) {
	a.cfgCacheMu.Lock()
	defer a.cfgCacheMu.Unlock()
	e, ok := a.configCache()[ref]
	return e, ok
}

// storeConfig records the outcome of fetching ref and writes the cache.
func (a *App) storeConfig(ref string, e configEntry) {
	path := a.configCachePath()
	if path == "" {
		return
	}
	a.cfgCacheMu.Lock()
	defer a.cfgCacheMu.Unlock()
	m := a.configCache()
	if len(e.Body) == 0 {
		// Keep the last good copy next to the failure, as a fallback.
		old := m[ref]
		e.Body, e.Source, e.SavedAt = old.Body, old.Source, old.SavedAt
	}
	m[ref] = e
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func (a *App) clearConfigCache() {
	a.cfgCacheMu.Lock()
	a.cfgCache = nil
	a.cfgCacheMu.Unlock()
	if path := a.configCachePath(); path != "" {
		_ = os.Remove(path)
	}
}

// errSkippedConfig marks a warehouse entry skipped because it failed lately.
var errSkippedConfig = errors.New("最近读取失败，已暂时跳过")

// fetchCached is fetch with the config cache in front of the network. nested
// is true for entries found inside a warehouse, as opposed to the user's own
// subscriptions.
func (a *App) fetchCached(ctx context.Context, ref string, nested bool) (*config.Config, error) {
	ref = strings.TrimSpace(ref)
	if a.configCachePath() == "" {
		return config.Load(ctx, ref)
	}
	e, have := a.lookupConfig(ref)
	cached := func() (*config.Config, bool) {
		if !have || len(e.Body) == 0 {
			return nil, false
		}
		cfg, err := config.FromBytes(e.Body, e.Source)
		return cfg, err == nil
	}
	if have && time.Since(e.SavedAt) < cacheMaxAge {
		if cfg, ok := cached(); ok {
			return cfg, nil
		}
	}
	if have && nested && time.Since(e.FailedAt) < cacheMaxAge {
		if cfg, ok := cached(); ok {
			return cfg, nil
		}
		return nil, errSkippedConfig
	}
	body, source, err := config.Fetch(ctx, ref)
	var cfg *config.Config
	if err == nil {
		cfg, err = config.FromBytes(body, source)
	}
	if err == nil {
		a.storeConfig(ref, configEntry{Body: body, Source: source, SavedAt: time.Now()})
		return cfg, nil
	}
	if ctx.Err() != nil && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, err // the load was cancelled: that says nothing about ref
	}
	a.storeConfig(ref, configEntry{FailedAt: time.Now(), Err: err.Error()})
	// Better an older copy than nothing.
	if cfg, ok := cached(); ok {
		return cfg, nil
	}
	return nil, err
}

type userRefsKey struct{}

// withUserRefs marks refs as the user's own subscriptions for the fetches
// made under ctx; every other ref fetched there is a warehouse entry.
func withUserRefs(ctx context.Context, refs []string) context.Context {
	set := make(map[string]bool, len(refs))
	for _, r := range refs {
		set[strings.TrimSpace(r)] = true
	}
	return context.WithValue(ctx, userRefsKey{}, set)
}

func isNestedRef(ctx context.Context, ref string) bool {
	set, _ := ctx.Value(userRefsKey{}).(map[string]bool)
	return !set[strings.TrimSpace(ref)]
}
