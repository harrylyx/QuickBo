package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// deadBlacklistTTL is how long a source that failed collection stays skipped.
// A dead host usually stays dead for a while; retrying it every launch wastes
// the whole collection pass on its timeout.
const deadBlacklistTTL = 10 * time.Minute

// deadList maps a hide ID (sourceHideID / liveHideID) to when it last failed.
type deadList map[string]time.Time

func (a *App) deadPath() string {
	if a.settingsPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(a.settingsPath), "dead.json")
}

func (a *App) loadDead() deadList {
	path := a.deadPath()
	m := deadList{}
	if path == "" {
		return m
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	if json.Unmarshal(b, &m) != nil {
		return deadList{}
	}
	now := time.Now()
	for id, t := range m {
		if now.Sub(t) > deadBlacklistTTL {
			delete(m, id)
		}
	}
	return m
}

func (a *App) saveDead(m deadList) {
	path := a.deadPath()
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

// isDead reports whether id failed recently and should be skipped.
func (a *App) isDead(id string, m deadList) bool {
	if m == nil {
		return false
	}
	t, ok := m[id]
	return ok && time.Since(t) < deadBlacklistTTL
}
