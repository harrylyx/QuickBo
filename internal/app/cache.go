package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

const cacheVersion = 2

// cacheMaxAge is how long a collected cache may be served before it is
// considered stale and a fresh collection runs. A collection that fails
// (network down, every source dead) writes nothing, so the old cache would
// otherwise be replayed indefinitely.
const cacheMaxAge = 24 * time.Hour

// cacheData is the on-disk result of a collection pass. It stores raw
// category -> item facts rather than homepage rows, so a later rule edit can
// be recomputed entirely offline, including keywords that matched no category
// when the collection was made.
type cacheData struct {
	Version int         `json:"version"`
	Ref     string      `json:"ref"`
	SavedAt time.Time   `json:"savedAt"`
	Sites   []cacheSite `json:"sites"`
	Lives   []string    `json:"lives,omitempty"`
}

type cacheSite struct {
	Key   string       `json:"key"`
	Name  string       `json:"name"`
	Voter string       `json:"voter,omitempty"`
	Hot   []model.Item `json:"hot,omitempty"`
	Cats  []cacheCat   `json:"cats"`
}

type cacheCat struct {
	Name  string       `json:"name"`
	Items []model.Item `json:"items"`
}

// An empty settingsPath disables the cache. Tests use that to stay offline
// and to avoid writing into the working directory.
func (a *App) cachePath() string {
	if a.settingsPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(a.settingsPath), "cache.json")
}

func (a *App) readCache(ref string) *cacheData {
	path := a.cachePath()
	if path == "" || strings.TrimSpace(ref) == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c cacheData
	if json.Unmarshal(b, &c) != nil {
		return nil
	}
	if c.Version != cacheVersion || c.Ref != ref || len(c.Sites) == 0 {
		return nil
	}
	if time.Since(c.SavedAt) > cacheMaxAge {
		return nil
	}
	if a.cacheIsStale(ref, c.SavedAt) {
		return nil
	}
	return &c
}

func (a *App) writeCache(c *cacheData) {
	path := a.cachePath()
	if path == "" || c == nil || len(c.Sites) == 0 {
		return
	}
	c.Version = cacheVersion
	c.SavedAt = time.Now()
	b, err := json.Marshal(c)
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

func (a *App) clearCache() {
	if path := a.cachePath(); path != "" {
		_ = os.Remove(path)
	}
}

func (c *cacheData) hasSite(key string) bool {
	for _, s := range c.Sites {
		if s.Key == key {
			return true
		}
	}
	return false
}

func (c *cacheData) hasLive(raw string) bool {
	raw = strings.TrimSpace(raw)
	for _, u := range c.Lives {
		if strings.TrimSpace(u) == raw {
			return true
		}
	}
	return false
}

// ballots turns cached facts into votes. Hot candidates keep a fixed bucket;
// category ballots leave Bucket empty so expandBallots can apply the current
// rules to Cat. A non-nil allowed set drops sources removed since caching.
func (c *cacheData) ballots(allowed map[string]bool) []ballot {
	var out []ballot
	for _, s := range c.Sites {
		if allowed != nil && !allowed[s.Key] {
			continue
		}
		for _, it := range s.Hot {
			if !usableTitle(it.Title) {
				continue
			}
			out = append(out, ballot{
				SiteKey: s.Key, SiteName: s.Name, Voter: s.Voter,
				Bucket: hotLaneID, Item: it,
			})
		}
		for _, cat := range s.Cats {
			for _, it := range cat.Items {
				out = append(out, ballot{
					SiteKey: s.Key, SiteName: s.Name, Voter: s.Voter,
					Cat: cat.Name, Item: it,
				})
			}
		}
	}
	return out
}

// restoreFromCache rebuilds the sidebar and the home screen with no network
// requests. The catalog still comes from the freshly parsed subscription, so
// playback and browsing use live source drivers.
func (a *App) restoreFromCache(c *cacheData, full *source.Catalog) {
	a.catalog = &source.Catalog{Parses: full.Parses, Index: full.Index}
	for _, s := range full.Sites {
		m := s.Meta()
		if !c.hasSite(m.Key) || a.hidden[sourceHideID(s)] {
			continue
		}
		a.catalog.Sites = append(a.catalog.Sites, s)
	}
	for _, lv := range full.Lives {
		if !c.hasLive(lv.URL) || a.hidden[liveHideID(lv.URL)] {
			continue
		}
		a.catalog.Lives = append(a.catalog.Lives, lv)
	}
	visible := map[string]bool{}
	for _, s := range a.catalog.Sites {
		visible[s.Meta().Key] = true
	}
	a.ballots = c.ballots(visible)
	a.rebuildHome()
	a.loading = false
	a.loadNote = ""
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
	a.doneLoad()
}
