package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/harrylyx/QuickBo/internal/model"
)

// library is the local continue-watching list and favorites. It is a JSON
// file next to settings, not a media catalog.
type library struct {
	mu    sync.Mutex
	path  string
	Watch []watchItem `json:"watch"`
	Favs  []favItem   `json:"favs"`
}

type watchItem struct {
	Key      string    `json:"key"`
	SiteKey  string    `json:"siteKey"`
	SiteName string    `json:"siteName"`
	VodID    string    `json:"vodId"`
	Title    string    `json:"title"`
	Pic      string    `json:"pic"`
	Episode  string    `json:"episode"`
	Line     string    `json:"line"`
	URL      string    `json:"url"`
	Position float64   `json:"position"`
	Duration float64   `json:"duration"`
	Updated  time.Time `json:"updated"`
}

type favItem struct {
	Key     string `json:"key"`
	SiteKey string `json:"siteKey"`
	VodID   string `json:"vodId"`
	Title   string `json:"title"`
	Pic     string `json:"pic"`
	Remarks string `json:"remarks"`
	Year    string `json:"year"`
}

func loadLibrary(dir string) *library {
	lib := &library{path: filepath.Join(dir, "library.json")}
	b, err := os.ReadFile(lib.path)
	if err != nil {
		return lib
	}
	_ = json.Unmarshal(b, lib)
	return lib
}

func (l *library) save() {
	if l == nil || l.path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(l.path), 0o755)
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(l.path, b, 0o644)
}

func (l *library) watches() []watchItem {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := append([]watchItem(nil), l.Watch...)
	return out
}

func (l *library) favs() []favItem {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]favItem(nil), l.Favs...)
}

func (l *library) upsertWatch(item watchItem) {
	if l == nil || item.Key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	item.Updated = time.Now()
	for i := range l.Watch {
		if l.Watch[i].Key == item.Key {
			if item.Position == 0 {
				item.Position = l.Watch[i].Position
			}
			if item.URL == "" {
				item.URL = l.Watch[i].URL
			}
			l.Watch[i] = item
			l.save()
			return
		}
	}
	l.Watch = append([]watchItem{item}, l.Watch...)
	if len(l.Watch) > 40 {
		l.Watch = l.Watch[:40]
	}
	l.save()
}

func (l *library) touchProgress(key string, pos, dur float64) {
	if l == nil || key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.Watch {
		if l.Watch[i].Key == key {
			l.Watch[i].Position = pos
			if dur > 0 {
				l.Watch[i].Duration = dur
			}
			l.Watch[i].Updated = time.Now()
			l.save()
			return
		}
	}
}

func (l *library) removeWatch(key string) {
	if l == nil || key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.Watch {
		if l.Watch[i].Key != key {
			continue
		}
		l.Watch = append(l.Watch[:i], l.Watch[i+1:]...)
		l.save()
		return
	}
}

func (l *library) hasFav(key string) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, f := range l.Favs {
		if f.Key == key {
			return true
		}
	}
	return false
}

func (l *library) toggleFav(item favItem) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.Favs {
		if l.Favs[i].Key == item.Key {
			l.Favs = append(l.Favs[:i], l.Favs[i+1:]...)
			l.save()
			return false
		}
	}
	l.Favs = append([]favItem{item}, l.Favs...)
	l.save()
	return true
}

func (l *library) removeFav(key string) {
	if l == nil || key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.Favs {
		if l.Favs[i].Key != key {
			continue
		}
		l.Favs = append(l.Favs[:i], l.Favs[i+1:]...)
		l.save()
		return
	}
}

func mediaKey(site, id string) string {
	return strings.TrimSpace(site) + "\n" + strings.TrimSpace(id)
}

func episodeIndex(d *model.Detail, line string, ep model.Episode) int {
	if d == nil {
		return 0
	}
	for _, ln := range d.Lines {
		if ln.Name != line {
			continue
		}
		for i, e := range ln.Episodes {
			if e.URL == ep.URL && e.Name == ep.Name {
				return i
			}
		}
	}
	return 0
}

func (a *App) watchKey() string {
	id := ""
	if a.detail != nil {
		id = a.detail.ID
	}
	if id == "" {
		id = a.playTitle
	}
	return mediaKey(a.siteKey, id)
}

func (a *App) rememberCurrent(raw string) {
	if a.lib == nil || a.fromLive {
		return
	}
	title := a.playTitle
	pic := ""
	id := ""
	if a.detail != nil {
		if a.detail.Title != "" {
			title = a.detail.Title
		}
		pic = a.detail.Pic
		id = a.detail.ID
	}
	ep := ""
	if a.detail != nil && a.playLine != "" && a.playIndex >= 0 {
		for _, ln := range a.detail.Lines {
			if ln.Name == a.playLine && a.playIndex < len(ln.Episodes) {
				ep = ln.Episodes[a.playIndex].Name
			}
		}
	}
	siteName := ""
	if m, ok := a.currentMeta(); ok {
		siteName = m.Name
	}
	a.lib.upsertWatch(watchItem{
		Key: mediaKey(a.siteKey, id), SiteKey: a.siteKey, SiteName: siteName,
		VodID: id, Title: title, Pic: pic, Episode: ep, Line: a.playLine, URL: raw,
	})
}

func (a *App) noteProgress() {
	if a.lib == nil || a.player == nil || a.fromLive {
		return
	}
	a.lib.touchProgress(a.watchKey(), a.player.Position().Seconds(), a.player.Duration().Seconds())
}
