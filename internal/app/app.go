// Package app is the native shell: subscription loading, source browsing
// and playback. It draws with mygo's GPU UI and does not embed a webview.
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/httpx"
	"github.com/harrylyx/QuickBo/internal/live"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/player"
	"github.com/harrylyx/QuickBo/internal/source"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

const (
	screenWelcome     = "welcome"
	screenIndex       = "index"
	screenBrowse      = "browse"
	screenDetail      = "detail"
	screenPlay        = "play"
	screenLive        = "live"
	screenUnsupported = "unsupported"
	screenEmpty       = "empty"
	screenHome        = "home"
	screenHistory     = "history"
	screenFav         = "fav"
	screenSources     = "sources"
	screenRules       = "rules"
)

// App is the window state. Methods that change it run on the UI thread,
// except the goroutines that publish results through Window.Update.
type App struct {
	win *mygo.Window

	cfg      *config.Config
	catalog  *source.Catalog
	cfgRef   string
	loading  bool
	loadErr  string
	loadNote string
	adding   bool
	urlDraft string

	loadCtx    context.Context
	loadCancel context.CancelFunc
	// fetchConfig overrides subscription loading in tests.
	fetchConfig func(context.Context, string) (*config.Config, error)
	// probeSite overrides the empty-list check in tests.
	probeSite func(context.Context, source.Source) (model.Page, bool)
	afterLoad func()
	postMu    sync.Mutex

	screen    string
	sideSel   string
	lastSide  string
	siteKey   string
	cats      []model.Category
	catID     string
	page      model.Page
	pageNum   int
	query     string
	committed string
	listErr   string
	listLoad  bool

	detail        *model.Detail
	detailTitle   string
	detailErr     string
	detailLoading bool

	playTitle   string
	playURL     string
	playHeaders map[string]string
	playErr     string
	playNote    string
	playLoading bool
	player      *ui.VideoPlayer
	fromLive    bool

	channels    []live.Channel
	liveIndex   int
	liveGroup   string
	liveErr     string
	liveLoading bool

	preferExternal bool
	light          bool
	volume         float64
	rate           float64
	linePick       int
	playLine       string
	playIndex      int
	resumeAt       float64
	showDrawer     bool
	showRates      bool
	controlsUntil  time.Time
	ptrX, ptrY     float32
	ptrSeen        bool
	savedVolume    float64
	appliedVolume  float64
	lib            *library

	browseScroll ui.ScrollState
	appending    bool
	listAnchored bool

	grid     ui.GridState
	selected int
	chState  ui.ListState

	posters       map[string]*ui.Bitmap
	fetching      map[string]bool
	failed        map[string]bool
	hidden        map[string]bool
	homeRules     []homeRule
	ruleNames     []string
	ruleWords     []string
	ruleFocus     []bool
	ruleSeq       int
	home          []homeLane
	homeHits      []homeLane
	homeSearching bool
	ballots       []ballot
	lookup        map[string][]titleHit
	searchLookup  map[string][]titleHit
	offers        []playOffer
	offerPick     int
	stats         map[string]sourceStat

	gen          int
	settingsPath string
}

type savedSettings struct {
	Config         string     `json:"config"`
	PreferExternal bool       `json:"preferExternal"`
	Light          bool       `json:"light,omitempty"`
	Volume         *float64   `json:"volume,omitempty"`
	Hidden         []string   `json:"hidden,omitempty"`
	HomeRules      []homeRule `json:"homeRules,omitempty"`
}

// New returns an empty shell. No subscription is loaded until the user
// supplies one, or a previously saved address is restored.
func New() *App {
	a := &App{
		screen:        screenWelcome,
		volume:        1,
		savedVolume:   1,
		appliedVolume: 1,
		rate:          1,
		lib:           loadLibrary(filepath.Dir(settingsPath())),
		selected:      -1,
		posters:       map[string]*ui.Bitmap{},
		fetching:      map[string]bool{},
		failed:        map[string]bool{},
		hidden:        map[string]bool{},
		homeRules:     defaultHomeRules(),
		lookup:        map[string][]titleHit{},
		stats:         map[string]sourceStat{},
		settingsPath:  settingsPath(),
	}
	a.grid.Selected = &a.selected
	a.grid.Key = func(i int) any {
		if i < 0 || i >= len(a.page.Items) {
			return i
		}
		if a.page.Items[i].ID != "" {
			return a.page.Items[i].ID
		}
		return i
	}
	a.grid.Label = func(i int) string {
		if i < 0 || i >= len(a.page.Items) {
			return ""
		}
		return a.page.Items[i].Title
	}
	a.chState.Label = func(i int) string {
		chs := a.visibleChannels()
		if i < 0 || i >= len(chs) {
			return ""
		}
		return chs[i].Name
	}
	return a
}

// Attach binds the window that receives async updates.
func (a *App) Attach(win *mygo.Window) { a.win = win }

// Load fetches a subscription from a URL or a local path.
func (a *App) Load(ref string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		a.loadErr = "请输入配置地址"
		return
	}
	a.loading = true
	a.loadErr = ""
	a.loadNote = ""
	ctx, gen := a.beginLoad()
	go func() {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cfg, err := a.fetch(cctx, ref)
		a.post(func() {
			if a.gen != gen {
				return
			}
			if err != nil {
				a.loading = false
				a.loadErr = err.Error()
				a.doneLoad()
				return
			}
			a.apply(cfg, ref)
		})
	}()
}

// LoadSaved restores the last subscription, if the user saved one.
func (a *App) LoadSaved() {
	b, err := os.ReadFile(a.settingsPath)
	if err != nil {
		return
	}
	var s savedSettings
	if json.Unmarshal(b, &s) != nil {
		return
	}
	a.preferExternal = s.PreferExternal
	a.light = s.Light
	a.hidden = map[string]bool{}
	for _, id := range s.Hidden {
		if id != "" {
			a.hidden[id] = true
		}
	}
	if len(s.HomeRules) > 0 {
		a.homeRules = normalizeRules(s.HomeRules)
		a.ruleNames, a.ruleWords, a.ruleFocus = nil, nil, nil
		a.noteRuleSeq()
	}
	if s.Volume != nil {
		a.volume = *s.Volume
		a.savedVolume = a.volume
		a.appliedVolume = a.volume
	}
	if strings.TrimSpace(s.Config) != "" {
		a.Load(s.Config)
	}
}

func (a *App) apply(cfg *config.Config, ref string) {
	a.closePlayer()
	a.cfg = cfg
	a.cfgRef = ref
	a.catalog = source.Open(cfg)
	a.detail = nil
	a.page = model.Page{}
	a.query = ""
	a.saveSettings()
	if len(a.catalog.Sites) == 0 && len(a.catalog.Lives) == 0 {
		if len(a.catalog.Index) > 0 {
			a.screen = screenIndex
			a.sideSel = ""
			a.expandIndex(a.catalog.Index)
			return
		}
		a.loading = false
		a.screen = screenEmpty
		a.doneLoad()
		return
	}
	full := a.catalog
	a.catalog = &source.Catalog{Parses: full.Parses, Index: full.Index}
	a.keepWorking(a.loadCtxOrBackground(), full)
}

func (a *App) onSide() {
	if a.sideSel == a.lastSide {
		return
	}
	a.lastSide = a.sideSel
	switch {
	case a.sideSel == "home":
		a.query = ""
		a.committed = ""
		a.homeHits = nil
		a.homeSearching = false
		a.screen = screenHome
		a.closePlayer()
	case a.sideSel == "history":
		a.query = ""
		a.committed = ""
		a.screen = screenHistory
		a.closePlayer()
	case a.sideSel == "fav":
		a.query = ""
		a.committed = ""
		a.screen = screenFav
		a.closePlayer()
	case a.sideSel == "sources":
		a.query = ""
		a.committed = ""
		a.screen = screenSources
		a.closePlayer()
	case a.sideSel == "rules":
		a.query = ""
		a.committed = ""
		a.screen = screenRules
		a.closePlayer()
	case strings.HasPrefix(a.sideSel, "site:"):
		a.selectSite(strings.TrimPrefix(a.sideSel, "site:"))
	case strings.HasPrefix(a.sideSel, "live:"):
		n, _ := strconv.Atoi(strings.TrimPrefix(a.sideSel, "live:"))
		a.selectLive(n)
	}
}

func (a *App) selectSite(key string) {
	src := a.catalog.Find(key)
	if src == nil {
		return
	}
	a.siteKey = key
	a.sideSel = "site:" + key
	a.lastSide = a.sideSel
	a.closePlayer()
	a.fromLive = false
	a.query = ""
	a.committed = ""
	a.page = model.Page{}
	a.pageNum = 1
	a.detail = nil
	if src.Meta().Level == config.LevelNo {
		a.screen = screenUnsupported
		return
	}
	a.screen = screenBrowse
	a.cats = nil
	a.catID = ""
	a.listErr = ""
	a.listLoad = true
	gen := a.bump()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		cats, err := src.Categories(ctx)
		a.post(func() {
			if a.gen != gen || a.siteKey != key {
				return
			}
			if err != nil && len(cats) == 0 {
				a.listErr = err.Error()
				a.cats = []model.Category{{ID: "", Name: "最新"}}
				a.catID = ""
				a.loadList()
				return
			}
			if len(cats) == 0 {
				cats = []model.Category{{ID: "", Name: "全部"}}
			}
			a.cats = cats
			a.catID = cats[0].ID
			a.loadList()
		})
	}()
}

func (a *App) pickCat(id string) {
	a.catID = id
	a.pageNum = 1
	a.query = ""
	a.committed = ""
	a.loadList()
}

func (a *App) loadList() {
	src := a.current()
	if src == nil {
		return
	}
	a.listLoad = true
	a.listErr = ""
	a.appending = false
	a.page = model.Page{}
	a.selected = -1
	a.listAnchored = a.pageNum <= 1
	if a.pageNum <= 1 {
		a.browseScroll.Y = 0
	}
	cat, page, q := a.catID, a.pageNum, strings.TrimSpace(a.committed)
	key := a.siteKey
	gen := a.bump()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		var (
			pg  model.Page
			err error
		)
		if q != "" {
			pg, err = src.Search(ctx, q, page)
		} else {
			pg, err = src.List(ctx, cat, page)
		}
		a.post(func() {
			if a.gen != gen || a.siteKey != key {
				return
			}
			a.listLoad = false
			a.page = pg
			if err != nil {
				a.listErr = err.Error()
			}
		})
	}()
}

func (a *App) loadMore() {
	src := a.current()
	if src == nil || a.listLoad || a.appending || !a.page.HasNext() {
		return
	}
	a.appending = true
	next := a.pageNum + 1
	if next < 1 {
		next = 2
	}
	cat, q := a.catID, strings.TrimSpace(a.committed)
	key := a.siteKey
	gen := a.gen
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		var (
			pg  model.Page
			err error
		)
		if q != "" {
			pg, err = src.Search(ctx, q, next)
		} else {
			pg, err = src.List(ctx, cat, next)
		}
		a.post(func() {
			if a.gen != gen || a.siteKey != key {
				a.appending = false
				return
			}
			a.appending = false
			if err != nil {
				a.listErr = err.Error()
				return
			}
			if len(pg.Items) == 0 {
				a.page.PageCount = a.pageNum
				return
			}
			a.pageNum = next
			a.page.Items = append(a.page.Items, pg.Items...)
			if pg.Page > 0 {
				a.page.Page = pg.Page
			}
			if pg.PageCount > 0 {
				a.page.PageCount = pg.PageCount
			}
			if pg.Total > 0 {
				a.page.Total = pg.Total
			}
		})
	}()
}

func (a *App) maybeLoadMore() {
	if a.listLoad || a.appending || !a.page.HasNext() {
		return
	}
	near := a.browseScroll.MaxY > 48 && a.browseScroll.Y >= a.browseScroll.MaxY-320
	short := a.browseScroll.MaxY <= 1 && len(a.page.Items) > 0 && len(a.page.Items) < 36
	if near || short {
		a.loadMore()
	}
}

func (a *App) openDetail(item model.Item) {
	src := a.current()
	if src == nil {
		return
	}
	a.screen = screenDetail
	a.offers = nil
	a.detail = nil
	a.detailTitle = item.Title
	a.detailErr = ""
	a.detailLoading = true
	a.linePick = 0
	a.closePlayer()
	id := item.ID
	key := a.siteKey
	gen := a.bump()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		d, err := src.Detail(ctx, id)
		a.post(func() {
			if a.gen != gen || a.siteKey != key {
				return
			}
			a.detailLoading = false
			if err != nil {
				a.detailErr = err.Error()
				return
			}
			if d.Title == "" {
				d.Title = item.Title
			}
			if d.Pic == "" {
				d.Pic = item.Pic
			}
			a.detail = &d
			a.linePick = model.PreferredLine(d.Lines)
		})
	}()
}

func (a *App) playEpisode(line string, ep model.Episode) {
	src := a.current()
	if src == nil {
		return
	}
	a.screen = screenPlay
	a.playLoading = true
	a.playErr = ""
	a.playNote = ""
	a.playTitle = ep.Name
	a.fromLive = false
	a.playLine = line
	a.playIndex = episodeIndex(a.detail, line, ep)
	a.showDrawer = false
	a.showRates = false
	a.closePlayer()
	key := a.siteKey
	parses := []config.ParseItem(nil)
	if a.cfg != nil {
		parses = a.cfg.Parses
	}
	gen := a.bump()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		pb, err := src.Resolve(ctx, line, ep)
		if err == nil && (pb.NeedsParse || !player.IsDirect(pb.URL)) {
			if u, h, ok := player.ApplyParses(ctx, pb.URL, parses); ok {
				pb.URL = u
				pb.Headers = mergeHeaders(pb.Headers, h)
			}
		}
		if err == nil && !player.IsDirect(pb.URL) {
			if u, ok := player.Unwrap(ctx, pb.URL, pb.Headers); ok && u != "" {
				pb.URL = u
			}
		}
		title := ep.Name
		a.post(func() {
			if a.gen != gen || a.siteKey != key {
				return
			}
			a.playLoading = false
			if err != nil {
				a.playErr = err.Error()
				a.playTitle = title
				return
			}
			if a.detail != nil && a.detail.Title != "" {
				title = a.detail.Title + "  ·  " + ep.Name
			}
			a.startPlay(title, pb.URL, pb.Headers, false)
		})
	}()
}

func (a *App) playChannel(ch live.Channel) {
	a.playLine = ch.Group
	a.playTitle = ch.Name
	a.fromLive = true
	a.screen = screenLive
	a.playLoading = true
	a.playErr = ""
	a.closePlayer()
	headers := map[string]string{"User-Agent": httpx.TVBoxUA}
	raw := ch.URL
	name := ch.Name
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if !player.IsDirect(raw) {
			if u, ok := player.Unwrap(ctx, raw, headers); ok && u != "" {
				raw = u
			}
		}
		a.post(func() {
			a.playLoading = false
			a.startPlay(name, raw, headers, true)
			a.screen = screenLive
			a.fromLive = true
		})
	}()
}

func (a *App) startPlay(title, raw string, headers map[string]string, fromLive bool) {
	a.closePlayer()
	a.playTitle = title
	a.playURL = raw
	a.playHeaders = headers
	a.playErr = ""
	a.playNote = ""
	a.playLoading = false
	a.fromLive = fromLive
	a.screen = screenPlay
	if strings.TrimSpace(raw) == "" {
		a.playErr = "没有播放地址"
		return
	}
	var notes []string
	if !player.IsDirect(raw) {
		notes = append(notes, "这个地址不像直接的媒体流，仍会尝试播放。")
	}
	if hasRequestHeaders(headers) {
		notes = append(notes, "原生播放器不会附带 Referer / User-Agent，失败时用 mpv。")
	}
	if a.preferExternal {
		if err := player.LaunchMPV(raw, headers); err != nil {
			a.playErr = "无法启动 mpv：" + err.Error()
		} else {
			notes = append(notes, "已交给 mpv。")
		}
		a.playNote = strings.Join(notes, " ")
		return
	}
	p, err := player.OpenNative(raw)
	if err != nil {
		a.playErr = err.Error()
		a.playNote = strings.Join(notes, " ")
		return
	}
	p.SetVolume(a.volume)
	if a.rate <= 0 {
		a.rate = 1
	}
	p.SetRate(a.rate)
	a.player = p
	a.playNote = strings.Join(notes, " ")
	a.controlsUntil = time.Now().Add(4 * time.Second)
	a.rememberCurrent(raw)
}

func (a *App) openExternal() {
	if a.player != nil {
		a.player.Pause()
	}
	if strings.TrimSpace(a.playURL) == "" {
		a.playErr = "没有播放地址"
		return
	}
	if err := player.LaunchMPV(a.playURL, a.playHeaders); err != nil {
		a.playErr = "无法启动 mpv：" + err.Error() + "。请确认 mpv 在 PATH 里。"
		return
	}
	a.playErr = ""
	a.playNote = "已交给 mpv。"
}

func (a *App) selectLive(i int) {
	if a.catalog == nil || i < 0 || i >= len(a.catalog.Lives) {
		return
	}
	a.liveIndex = i
	a.sideSel = "live:" + strconv.Itoa(i)
	a.lastSide = a.sideSel
	a.query = ""
	a.committed = ""
	a.screen = screenLive
	a.closePlayer()
	a.fromLive = true
	a.channels = nil
	a.liveErr = ""
	a.liveLoading = true
	entry := a.catalog.Lives[i]
	gen := a.bump()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		ch, err := entry.Load(ctx)
		a.post(func() {
			if a.gen != gen || a.liveIndex != i {
				return
			}
			a.liveLoading = false
			a.channels = ch
			if err != nil {
				a.liveErr = err.Error()
			}
		})
	}()
}

func (a *App) back() {
	a.bump()
	a.playLoading = false
	a.detailLoading = false
	a.closePlayer()
	switch a.screen {
	case screenPlay:
		if a.fromLive {
			a.screen = screenLive
			return
		}
		if a.detail != nil {
			a.screen = screenDetail
			return
		}
		a.screen = screenBrowse
	case screenDetail:
		if a.sideSel == "home" || a.sideSel == "" {
			a.screen = screenHome
			return
		}
		a.screen = screenBrowse
	}
}

func (a *App) pickFile() {
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  win,
			Title:   "打开配置",
			Filters: []mygo.FileFilter{{Name: "配置", Extensions: []string{"json", "txt"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		path := paths[0]
		a.post(func() {
			a.adding = false
			a.urlDraft = path
			a.Load(path)
		})
	}()
}

func (a *App) poster(url string) *ui.Bitmap {
	url = model.ResolveURL("", url)
	if url == "" || a.failed[url] {
		return nil
	}
	if bmp, ok := a.posters[url]; ok {
		return bmp
	}
	if a.win == nil || a.fetching[url] || len(a.fetching) >= 8 {
		return nil
	}
	a.fetching[url] = true
	win := a.win
	headers := a.imageHeaders()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		body, _, err := httpx.Get(ctx, nil, url, headers, 4<<20)
		if err != nil && headers["Referer"] != "" {
			retry := map[string]string{"User-Agent": httpx.BrowserUA, "Accept": "image/*,*/*"}
			body, _, err = httpx.Get(ctx, nil, url, retry, 4<<20)
		}
		var img image.Image
		if err == nil {
			img, _, err = image.Decode(bytes.NewReader(body))
		}
		raw := body
		win.Update(func() {
			delete(a.fetching, url)
			if img != nil {
				a.remember(url, ui.NewBitmap(img))
				return
			}
			if err == nil {
				if bmp, decErr := ui.DecodeBitmap(raw); decErr == nil {
					a.remember(url, bmp)
					return
				}
			}
			a.failed[url] = true
		})
	}()
	return nil
}

func (a *App) imageHeaders() map[string]string {
	h := map[string]string{
		"User-Agent": httpx.BrowserUA,
		"Accept":     "image/avif,image/webp,image/*,*/*",
	}
	if m, ok := a.currentMeta(); ok {
		if u, err := url.Parse(m.API); err == nil && u.Scheme != "" && u.Host != "" {
			h["Referer"] = u.Scheme + "://" + u.Host + "/"
		}
	}
	return h
}

func (a *App) remember(url string, bmp *ui.Bitmap) {
	if len(a.posters) > 200 {
		a.posters = map[string]*ui.Bitmap{}
	}
	a.posters[url] = bmp
}

func (a *App) visibleChannels() []live.Channel {
	q := strings.ToLower(strings.TrimSpace(a.query))
	if q == "" {
		return a.channels
	}
	var out []live.Channel
	for _, ch := range a.channels {
		if strings.Contains(strings.ToLower(ch.Group+" "+ch.Name), q) {
			out = append(out, ch)
		}
	}
	return out
}

func (a *App) current() source.Source {
	if a.catalog == nil {
		return nil
	}
	return a.catalog.Find(a.siteKey)
}

func (a *App) currentMeta() (source.Meta, bool) {
	src := a.current()
	if src == nil {
		return source.Meta{}, false
	}
	return src.Meta(), true
}

func (a *App) bump() int {
	a.gen++
	return a.gen
}

func (a *App) post(fn func()) {
	if a.win == nil {
		a.postMu.Lock()
		fn()
		a.postMu.Unlock()
		return
	}
	a.win.Update(fn)
}

func (a *App) closePlayer() {
	if a.player != nil {
		a.player.Close()
		a.player = nil
	}
}

func (a *App) saveSettings() {
	if a.settingsPath == "" {
		return
	}
	vol := a.volume
	s := savedSettings{
		Config: a.cfgRef, PreferExternal: a.preferExternal, Light: a.light, Volume: &vol,
		Hidden:    hiddenIDs(a.hidden),
		HomeRules: cloneRules(a.homeRules),
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(a.settingsPath), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(a.settingsPath, b, 0o644)
}

func settingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "quickbo", "settings.json")
}

func mergeHeaders(a, b map[string]string) map[string]string {
	if len(a) == 0 {
		return b
	}
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func hasRequestHeaders(h map[string]string) bool {
	for k, v := range h {
		if strings.TrimSpace(v) == "" {
			continue
		}
		switch strings.ToLower(k) {
		case "referer", "referrer", "user-agent":
			return true
		}
	}
	return false
}
