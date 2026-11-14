package app

import (
	"context"
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
	// Exact match by (site, id): preserve progress and timing facts the caller
	// may not have measured yet.
	for i := range l.Watch {
		if l.Watch[i].Key == item.Key {
			mergeWatch(&item, l.Watch[i])
			l.Watch[i] = item
			l.save()
			return
		}
	}
	// Cross-source: the same title opened from another source folds into one
	// continue-watching entry, keeping the newest source as the resume target.
	if t := watchTitle(item.Title); t != "" {
		for i := range l.Watch {
			if watchTitle(l.Watch[i].Title) == t {
				mergeWatch(&item, l.Watch[i])
				l.Watch[i] = item
				l.save()
				return
			}
		}
	}
	l.Watch = append([]watchItem{item}, l.Watch...)
	if len(l.Watch) > 40 {
		l.Watch = l.Watch[:40]
	}
	l.save()
}

// mergeWatch keeps the stored value for fields the new item left empty, so a
// re-open never clobbers progress, URL or a known duration with a zero value.
func mergeWatch(dst *watchItem, prev watchItem) {
	if dst.Position == 0 {
		dst.Position = prev.Position
	}
	if dst.URL == "" {
		dst.URL = prev.URL
	}
	// Resuming from history has no episode list to name the episode from.
	if dst.Episode == "" {
		dst.Episode = prev.Episode
	}
	if dst.Pic == "" {
		dst.Pic = prev.Pic
	}
	if dst.Duration == 0 {
		dst.Duration = prev.Duration
	}
}

// watchTitle is the loose title identity used to merge a title across sources.
func watchTitle(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
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

// playHistory resumes a history entry. The detail of whatever was opened
// last must not stay behind: the player title, the history record and the
// progress all key off a.detail, so a stale one sent every entry to the
// title watched before. Continue watching on the home page comes back to
// that title's detail, once the episode list has been filled in.
func (a *App) playHistory(w watchItem) {
	toDetail := a.screen == screenHome
	a.prepareHistory(w)
	if toDetail {
		a.noteDetailFrom()
		a.screen = screenDetail
	}
	a.startPlay(w.Title, w.URL, nil, false)
	a.fetchHistoryDetail(w)
}

// fetchHistoryDetail loads the title's lines so the player can change
// episode, and so back from continue watching has a detail page to show.
func (a *App) fetchHistoryDetail(w watchItem) {
	src := a.current()
	if src == nil || strings.TrimSpace(w.VodID) == "" {
		a.detailLoading = false
		return
	}
	a.historyToken++
	token := a.historyToken
	key, id := a.siteKey, w.VodID
	a.detailLoading = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		d, err := src.Detail(ctx, id)
		a.post(func() {
			if a.historyToken != token || a.siteKey != key {
				return
			}
			if a.detail == nil || a.detail.ID != id {
				return
			}
			a.detailLoading = false
			if err != nil {
				a.detailErr = err.Error()
				return
			}
			if d.ID == "" {
				d.ID = id
			}
			if d.Title == "" {
				d.Title = w.Title
			}
			if d.Pic == "" {
				d.Pic = w.Pic
			}
			a.detail = &d
			a.detailErr = ""
			a.placeHistoryEpisode(w)
		})
	}()
}

// placeHistoryEpisode points the player and the detail grid at the episode
// the history entry was watching.
func (a *App) placeHistoryEpisode(w watchItem) {
	if a.detail == nil || len(a.detail.Lines) == 0 {
		return
	}
	lineIdx := 0
	for i, ln := range a.detail.Lines {
		if w.Line != "" && ln.Name == w.Line {
			lineIdx = i
			break
		}
	}
	ln := a.detail.Lines[lineIdx]
	idx := 0
	found := false
	if w.Episode != "" {
		for i, ep := range ln.Episodes {
			if ep.Name == w.Episode {
				idx, found = i, true
				break
			}
		}
	}
	if !found && w.URL != "" {
		for i, ep := range ln.Episodes {
			if ep.URL == w.URL {
				idx = i
				break
			}
		}
	}
	a.linePick = lineIdx
	a.playLine = ln.Name
	a.playIndex = idx
	a.epLine = ln.Name
	a.epPage = idx / 30
	if idx < len(ln.Episodes) && ln.Episodes[idx].Name != "" {
		a.resumeEpisode = ln.Episodes[idx].Name
	}
}

func (a *App) prepareHistory(w watchItem) {
	a.resumeAt = w.Position
	a.siteKey = w.SiteKey
	a.playLine = w.Line
	a.playIndex = -1
	a.linePick = 0
	a.detail = &model.Detail{ID: w.VodID, Title: w.Title, Pic: w.Pic}
	a.detailTitle = w.Title
	a.resumeEpisode = w.Episode
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
