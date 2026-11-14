package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
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
	// Names is every usable category of the source, not only the ones whose
	// first page was stored in Cats.
	Names []cacheName `json:"names,omitempty"`
}

type cacheName struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
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

func cacheNames(cats []model.Category) []cacheName {
	if len(cats) == 0 {
		return nil
	}
	out := make([]cacheName, 0, len(cats))
	for _, cat := range cats {
		name := strings.TrimSpace(cat.Name)
		if name == "" {
			continue
		}
		out = append(out, cacheName{ID: cat.ID, Name: name})
	}
	return out
}

func factsFromCache(c *cacheData, allowed map[string]bool) []catFact {
	var out []catFact
	sites := 0
	for _, s := range c.Sites {
		if allowed != nil && !allowed[s.Key] {
			continue
		}
		if sites >= homeSourceLimit || len(out) >= homeFactLimit {
			break
		}
		sites++
		names := s.Names
		if len(names) == 0 {
			for _, cat := range s.Cats {
				names = append(names, cacheName{Name: cat.Name})
			}
		}
		for _, n := range names {
			if len(out) >= homeFactLimit {
				break
			}
			name := strings.TrimSpace(n.Name)
			if name == "" {
				continue
			}
			out = append(out, catFact{SiteKey: s.Key, SiteName: s.Name, Name: name, ID: n.ID})
		}
	}
	return out
}

func (a *App) clearCache() {
	if path := a.cachePath(); path != "" {
		_ = os.Remove(path)
	}
}

// fileSize is one cache file's size, or 0 when it is not there.
func (a *App) fileSize(path string) int64 {
	if path == "" {
		return 0
	}
	if st, err := os.Stat(path); err == nil {
		return st.Size()
	}
	return 0
}

// recordPaths are the bookkeeping records kept beside the settings: dead
// sources, the parse interfaces' record, and removed subscriptions. Each
// carries its own TTL; none of it is the user's own data.
func (a *App) recordPaths() []string {
	if a.settingsPath == "" {
		return nil
	}
	dir := filepath.Dir(a.settingsPath)
	return []string{
		filepath.Join(dir, "dead.json"),
		filepath.Join(dir, "parse_rank.json"),
		filepath.Join(dir, "sub_tombstones.json"),
	}
}

// clearRecords drops those files and the parse record in memory; the next
// collection and the next parse refill them. Call it on the UI thread.
func (a *App) clearRecords() {
	for _, p := range a.recordPaths() {
		if p != "" {
			_ = os.Remove(p)
		}
	}
	a.parseMu.Lock()
	a.parseRank = nil
	a.parseMu.Unlock()
}

// clearPosterCache drops the decoded posters and their blurred backdrops
// from memory; the covers on screen reload as they come into view again.
// In-flight downloads still land. Call it on the UI thread.
func (a *App) clearPosterCache() {
	a.posters = map[string]*ui.Bitmap{}
	a.posterUsed = map[string]int64{}
	a.art = map[string]artExtras{}
	a.fetching = map[string]bool{}
	a.failed = map[string]bool{}
	a.posterTries = map[string]int{}
	a.coverLaid = map[string]bool{}
}

// humanBytes sizes a cache the way the file dialog would.
func humanBytes(n int64) string {
	b := float64(n)
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", b/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", b/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", b/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
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
	sites := 0
	for _, s := range c.Sites {
		if allowed != nil && !allowed[s.Key] {
			continue
		}
		// Sites collected beyond the home limit are cached by key only so
		// restoring the cache still shows them in the sidebar.
		if len(s.Hot) == 0 && len(s.Cats) == 0 {
			continue
		}
		if sites >= homeSourceLimit || len(out) >= homeBallotLimit {
			break
		}
		sites++
		for _, it := range s.Hot {
			if len(out) >= homeBallotLimit {
				break
			}
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
				if len(out) >= homeBallotLimit {
					break
				}
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
// playback and browsing use live source drivers. After restoring from cache,
// it starts a background refresh to update the content.
func (a *App) restoreFromCache(c *cacheData, full *source.Catalog) {
	a.catalog = &source.Catalog{Parses: full.Parses, Index: full.Index}
	// Build the lookup once. hasSite/hasLive scan linearly, which made a
	// 10k-site restore quadratic on the UI thread.
	cachedSites := make(map[string]bool, len(c.Sites))
	for _, s := range c.Sites {
		cachedSites[s.Key] = true
	}
	cachedLives := make(map[string]bool, len(c.Lives))
	for _, u := range c.Lives {
		cachedLives[strings.TrimSpace(u)] = true
	}
	for _, s := range full.Sites {
		m := s.Meta()
		if !cachedSites[m.Key] || a.hidden[sourceHideID(s)] || a.disabled[sourceHideID(s)] || a.blacklisted(m.Name) {
			continue
		}
		a.catalog.Sites = append(a.catalog.Sites, s)
	}
	for _, lv := range full.Lives {
		if !cachedLives[strings.TrimSpace(lv.URL)] || a.hidden[liveHideID(lv.URL)] || a.blacklisted(lv.Name) {
			continue
		}
		a.catalog.Lives = append(a.catalog.Lives, lv)
	}
	visible := map[string]bool{}
	for _, s := range a.catalog.Sites {
		visible[s.Meta().Key] = true
	}
	a.ballots = c.ballots(visible)
	a.catFacts = factsFromCache(c, visible)
	a.rebuildHome()
	a.loading = false
	a.loadNote = ""
	if !a.keepScreen {
		a.screen = screenHome
		a.sideSel = "home"
		a.lastSide = "home"
	}
	a.doneLoad()
	// Refresh in the background without blanking the restored home. Only in a
	// real window: headless tests must not start network probes.
	if a.win != nil {
		a.keepWorkingMode(a.loadCtxOrBackground(), full, true)
	}
}
