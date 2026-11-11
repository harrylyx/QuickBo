package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// subTombstones records when a subscription was removed. A cache written
// before a member subscription was removed is stale: it included the removed
// subscription's sources, and re-adding that subscription later must not replay
// the pre-removal cache.
type subTombstones map[string]time.Time

// subTombstoneTTL bounds how long a removal keeps invalidating caches. After
// this long the cache is old enough to be re-collected anyway.
const subTombstoneTTL = 30 * 24 * time.Hour

func (a *App) tombstonePath() string {
	if a.settingsPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(a.settingsPath), "sub_tombstones.json")
}

func (a *App) loadTombstones() subTombstones {
	path := a.tombstonePath()
	m := subTombstones{}
	if path == "" {
		return m
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	if json.Unmarshal(b, &m) != nil {
		return subTombstones{}
	}
	cutoff := time.Now().Add(-subTombstoneTTL)
	for ref, t := range m {
		if t.Before(cutoff) {
			delete(m, ref)
		}
	}
	return m
}

func (a *App) saveTombstones(m subTombstones) {
	path := a.tombstonePath()
	if path == "" || m == nil {
		return
	}
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

// tombstone records a subscription removal so its stale cache contribution is
// not resurrected when the subscription is later re-added.
func (a *App) tombstone(ref string) {
	ref = strings.TrimSpace(ref)
	if ref == "" || a.tombstonePath() == "" {
		return
	}
	m := a.loadTombstones()
	m[ref] = time.Now()
	a.saveTombstones(m)
}

// cacheIsStale reports whether a cache was written before any of the
// subscriptions that produced it was removed.
func (a *App) cacheIsStale(ref string, savedAt time.Time) bool {
	m := a.loadTombstones()
	for _, r := range strings.Split(ref, "\n") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if t, ok := m[r]; ok && t.After(savedAt) {
			return true
		}
	}
	return false
}
