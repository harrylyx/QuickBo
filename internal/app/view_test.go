package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

func TestWelcomeMentionsEmptyShellAndJar(t *testing.T) {
	a := New()
	tt := ui.NewTester(a.View, 960, 640)
	if !tt.HasText("QuickBo") || !tt.HasText("不内置任何片源") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if !tt.HasText("不执行 jar") {
		t.Fatalf("missing jar note: %q", tt.Texts())
	}
	joined := stringsJoin(tt.Texts())
	if contains(joined, "一台不带片源") || contains(joined, "自备片源") {
		t.Fatalf("welcome copy repeats the empty-shell claim: %q", tt.Texts())
	}
	if err := tt.Click("添加订阅"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("配置 URL") {
		t.Fatalf("dialog texts %q", tt.Texts())
	}
}

func TestUnsupportedJarRenders(t *testing.T) {
	a := New()
	a.catalog = &source.Catalog{Sites: []source.Source{deadFake{}}}
	a.siteKey = "jar"
	a.sideSel = "site:jar"
	a.screen = screenUnsupported
	tt := ui.NewTester(a.View, 1100, 720)
	joined := stringsJoin(tt.Texts())
	if !contains(joined, "不执行 jar") || !contains(joined, "蜘蛛") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestHomeShowsEverySource(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}, otherFake{}}}
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
	a.home = []homeLane{
		{Key: "c", Name: "演示源", Items: []model.Item{{ID: "1", Title: "示例影片"}}},
		{Key: "d", Name: "另一源", Items: []model.Item{{ID: "2", Title: "另一部"}}},
	}
	expandSideGroups(a)
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("示例影片") || !tt.HasText("另一部") || !tt.HasText("全部片源") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.RightClick("演示源"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if len(menu) != 1 || menu[0] != "删除" {
		t.Fatalf("menu %v", menu)
	}
	if err := tt.ChooseMenuItem("删除"); err != nil {
		t.Fatal(err)
	}
	if len(a.catalog.Sites) != 1 || a.catalog.Sites[0].Meta().Name != "另一源" {
		t.Fatalf("sites left: %d", len(a.catalog.Sites))
	}
	if len(a.home) != 1 || a.home[0].Name != "另一源" {
		t.Fatalf("rails %+v", a.home)
	}
}

func TestBrowseShowsPosterCard(t *testing.T) {
	a := New()
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.siteKey = "c"
	a.sideSel = "site:c"
	a.screen = screenBrowse
	a.cats = []model.Category{{ID: "", Name: "最新"}, {ID: "1", Name: "电影"}}
	a.catID = ""
	a.page = model.Page{Items: []model.Item{{ID: "1", Title: "示例影片", Remarks: "HD"}}, Page: 1, PageCount: 1}
	tt := ui.NewTester(a.View, 1100, 720)
	if !tt.HasText("示例影片") || !tt.HasText("演示源") || !tt.HasText("电影") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestHistoryCardDeletesWithoutOpeningPlayback(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{}
	a.lib = &library{Watch: []watchItem{{Key: "c\n1", Title: "海边的风"}}}
	a.screen = screenHistory
	a.sideSel = "history"
	a.lastSide = "history"
	tt := ui.NewTester(a.View, 1200, 800)
	r, ok := tt.Find("海边的风")
	if !ok {
		t.Fatalf("missing history title: %q", tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if !tt.HasText("删除历史") {
		t.Fatalf("missing delete badge: %q", tt.Texts())
	}
	if err := tt.Click("删除历史"); err != nil {
		t.Fatal(err)
	}
	if len(a.lib.watches()) != 0 {
		t.Fatalf("watch remains: %+v", a.lib.watches())
	}
	if a.screen != screenHistory || a.playTitle != "" {
		t.Fatalf("delete also opened playback: screen %q title %q", a.screen, a.playTitle)
	}
}

func TestDetailCanSortOffersByClarityOrDelay(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{}
	a.screen = screenDetail
	a.detailTitle = "庆余年"
	a.stats = map[string]sourceStat{"slow": {Delay: 800}, "fast": {Delay: 40}}
	a.offers = []playOffer{
		{SiteKey: "slow", SiteName: "慢源", Clarity: 50},
		{SiteKey: "fast", SiteName: "快源", Clarity: 10},
	}
	a.sortOffers()
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("4K") || !tt.HasText("清晰度未知") || !tt.HasText("慢源") || !tt.HasText("快源 · 40 ms") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if a.offers[0].SiteKey != "slow" {
		t.Fatalf("sharper group should come first, got %s", a.offers[0].SiteKey)
	}
}

type deadFake struct{}

func (deadFake) Meta() source.Meta {
	return source.Meta{Key: "jar", Name: "蜘蛛", API: "csp_AppYS", Kind: config.KindJar, Level: config.LevelNo, Reason: "JAR/DEX 爬虫（csp_AppYS）本版不执行 jar。"}
}
func (deadFake) Categories(context.Context) ([]model.Category, error) {
	return nil, errFake("不执行 jar")
}
func (deadFake) List(context.Context, string, int) (model.Page, error) {
	return model.Page{}, errFake("不执行 jar")
}
func (deadFake) Search(context.Context, string, int) (model.Page, error) {
	return model.Page{}, errFake("不执行 jar")
}
func (deadFake) Detail(context.Context, string) (model.Detail, error) {
	return model.Detail{}, errFake("不执行 jar")
}
func (deadFake) Resolve(context.Context, string, model.Episode) (model.Playable, error) {
	return model.Playable{}, errFake("不执行 jar")
}

type cmsFake struct{}

func (cmsFake) Meta() source.Meta {
	return source.Meta{Key: "c", Name: "演示源", API: "https://example.invalid/vod", Kind: config.KindCMSJSON, Level: config.LevelYes}
}
func (cmsFake) Categories(context.Context) ([]model.Category, error) {
	return []model.Category{{ID: "", Name: "最新"}}, nil
}
func (cmsFake) List(context.Context, string, int) (model.Page, error) { return model.Page{}, nil }
func (cmsFake) Search(context.Context, string, int) (model.Page, error) {
	return model.Page{}, nil
}
func (cmsFake) Detail(context.Context, string) (model.Detail, error) { return model.Detail{}, nil }
func (cmsFake) Resolve(context.Context, string, model.Episode) (model.Playable, error) {
	return model.Playable{}, nil
}

type otherFake struct{}

func (otherFake) Meta() source.Meta {
	return source.Meta{Key: "d", Name: "另一源", API: "https://example.invalid/other", Kind: config.KindCMSJSON, Level: config.LevelYes}
}
func (otherFake) Categories(context.Context) ([]model.Category, error) { return nil, nil }
func (otherFake) List(context.Context, string, int) (model.Page, error) {
	return model.Page{}, nil
}
func (otherFake) Search(context.Context, string, int) (model.Page, error) {
	return model.Page{}, nil
}
func (otherFake) Detail(context.Context, string) (model.Detail, error) { return model.Detail{}, nil }
func (otherFake) Resolve(context.Context, string, model.Episode) (model.Playable, error) {
	return model.Playable{}, nil
}

type errFake string

func (e errFake) Error() string { return string(e) }

func TestSidebarRemoveIsOnlyAHoverIcon(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
	expandSideGroups(a)
	tt := ui.NewTester(a.View, 1200, 800)
	if _, ok := tt.Find("移除"); ok {
		t.Fatalf("sidebar still shows a permanent remove label: %q", tt.Texts())
	}
	r, ok := tt.Find("演示源")
	if !ok {
		t.Fatalf("missing sidebar source: %q", tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if _, ok := tt.Find("移除源"); !ok {
		t.Fatalf("missing hover remove icon: %q", tt.Texts())
	}
	if err := tt.Click("移除源"); err != nil {
		t.Fatal(err)
	}
	if len(a.catalog.Sites) != 0 {
		t.Fatalf("source was not removed: %d left", len(a.catalog.Sites))
	}
}

func TestHomeRailHasNoDeleteMenu(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
	a.home = []homeLane{{Key: "hot", Name: "热门电影", Items: []model.Item{{ID: "1", Title: "海边的风"}}}}
	tt := ui.NewTester(a.View, 1200, 800)
	if _, ok := tt.Find("热门电影"); !ok {
		t.Fatalf("missing rail: %q", tt.Texts())
	}
	if err := tt.RightClick("热门电影"); err != nil {
		t.Fatal(err)
	}
	if menu := tt.Menu(); len(menu) != 0 {
		t.Fatalf("home rail should not have a delete menu: %v", menu)
	}
}

func TestDetailHidesSearchField(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.siteKey = "c"
	a.sideSel = "site:c"
	a.lastSide = "site:c"
	a.screen = screenHome
	tt := ui.NewTester(a.View, 1200, 800)
	if _, ok := tt.Find("搜索"); !ok {
		t.Fatalf("home should show the search field: %q", tt.Texts())
	}
	a.screen = screenDetail
	a.detailTitle = "海边的风"
	a.detailItem = model.Item{ID: "1", Title: "海边的风"}
	a.detail = &model.Detail{Title: "海边的风", Lines: []model.Line{
		{Name: "默认", Episodes: []model.Episode{{Name: "第1集", URL: "about:blank"}}},
	}}
	a.playLine = "默认"
	tt.Frame()
	if _, ok := tt.Find("搜索"); ok {
		t.Fatalf("detail should not show an inert search field: %q", tt.Texts())
	}
	if !tt.HasText("第1集") {
		t.Fatalf("missing episode grid: %q", tt.Texts())
	}
}

func TestLoadErrorOffersRetryAndEmptyHistoryOffersHome(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{}
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
	a.loadErr = "读取失败"
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("重试订阅") {
		t.Fatalf("missing retry action: %q", tt.Texts())
	}
	if err := tt.Click("重试订阅"); err != nil {
		t.Fatal(err)
	}
	// With a configuration loaded, subscriptions live on the management page.
	if a.screen != screenSubs || a.sideSel != "manage" || a.loadErr != "" {
		t.Fatalf("retry did not open subscriptions: screen=%q side=%q err=%q", a.screen, a.sideSel, a.loadErr)
	}

	b := New()
	b.settingsPath = ""
	b.lib = &library{}
	b.catalog = &source.Catalog{}
	b.screen = screenHistory
	b.sideSel = "history"
	b.lastSide = "history"
	history := ui.NewTester(b.View, 1200, 800)
	if !history.HasText("去首页") {
		t.Fatalf("missing empty-state action: %q", history.Texts())
	}
	if err := history.Click("去首页"); err != nil {
		t.Fatal(err)
	}
	if b.screen != screenHome || b.sideSel != "home" {
		t.Fatalf("empty-state action did not go home: %q / %q", b.screen, b.sideSel)
	}
}

func TestHotRailHidesAllSourceLink(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.lib = &library{}
	a.catalog = &source.Catalog{}
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
	a.home = []homeLane{{
		Key: latestRuleID, Name: "最新影视",
		Items: []model.Item{{ID: titlePrefix + "hot-title", Title: "热播片"}},
	}}
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("最新影视") {
		t.Fatalf("missing hot rail: %q", tt.Texts())
	}
	if _, ok := tt.Find("全部"); ok {
		t.Fatalf("synthetic hot rail still has a dead all-source link: %q", tt.Texts())
	}
}

func TestStatColorEncodesLatencyAndFailure(t *testing.T) {
	th := cinemaDark()
	same := func(x, y ui.Color) bool {
		return x.R == y.R && x.G == y.G && x.B == y.B && x.A == y.A
	}
	cases := []struct {
		stat sourceStat
		want ui.Color
	}{
		{sourceStat{Delay: 120}, th.Success},
		{sourceStat{Delay: 500}, th.Warning},
		{sourceStat{Delay: 900}, th.TextMuted},
		{sourceStat{Delay: -1}, th.Danger},
		{sourceStat{Err: "超时"}, th.Danger},
		{sourceStat{}, th.TextMuted},
	}
	for _, tc := range cases {
		if got := statColor(th, tc.stat); !same(got, tc.want) {
			t.Fatalf("stat %+v -> %+v, want %+v", tc.stat, got, tc.want)
		}
	}
}

func TestChromeFadeIsBrief(t *testing.T) {
	fade := chromeFade(8)
	if fade.Duration != 200*time.Millisecond || fade.ExitDuration != 160*time.Millisecond {
		t.Fatalf("unexpected fade durations: %+v", fade)
	}
	if fade.Enter == nil || fade.Exit == nil || fade.Enter.Opacity != 0 || fade.Exit.Opacity != 0 {
		t.Fatalf("missing fade endpoints: %+v", fade)
	}
}

func TestDetailLoadingUsesDetailSkeletonNotListGrid(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{}
	a.screen = screenDetail
	a.sideSel = "home"
	a.lastSide = "home"
	a.siteKey = "c"
	a.detailTitle = "Doctor X：白色黑手党时代"
	a.detailItem = model.Item{ID: "1", Title: "Doctor X：白色黑手党时代"}
	a.detail = nil
	a.detailLoading = true
	a.offers = []playOffer{{SiteKey: "c", SiteName: "演示源", ItemID: "1", Clarity: 10}}
	tt := ui.NewTester(a.View, 1200, 800)
	if !tt.HasText("Doctor X：白色黑手党时代") {
		t.Fatalf("missing detail title: %q", tt.Texts())
	}
	if tt.HasText("正在加载…") {
		t.Fatalf("detail still renders the list skeleton: %q", tt.Texts())
	}
	if !tt.HasText("正在读取各片源的线路和清晰度…") || !tt.HasText("片源") || !tt.HasText("演示源") {
		t.Fatalf("missing detail loading state: %q", tt.Texts())
	}

	b := New()
	b.settingsPath = ""
	b.catalog = &source.Catalog{}
	b.screen = screenDetail
	b.sideSel = "home"
	b.lastSide = "home"
	b.siteKey = "c"
	b.detailTitle = "单源影片"
	b.detail = nil
	b.detailLoading = true
	single := ui.NewTester(b.View, 1200, 800)
	if single.HasText("正在加载…") {
		t.Fatalf("single-source detail still renders the list skeleton: %q", single.Texts())
	}
	if !single.HasText("正在读取影片详情…") {
		t.Fatalf("missing single-source loading state: %q", single.Texts())
	}
}

func stringsJoin(ss []string) string {
	out := ""
	for _, s := range ss {
		out += s + "\n"
	}
	return out
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// expandSideGroups opens every sidebar source group, which start collapsed.
func expandSideGroups(a *App) {
	for _, g := range a.sideGroups() {
		*a.sidebarOpenState(g.name, false) = true
	}
}

// Sidebar source groups start collapsed and say they hold sources; the group
// holding the source on screen starts open.
func TestSidebarGroupsStartCollapsedAndLabelled(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}, otherFake{}}}
	a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
	tt := ui.NewTester(a.View, 1200, 800)
	if _, ok := tt.Find("演示源"); ok {
		t.Fatalf("source groups must start collapsed: %q", tt.Texts())
	}
	found := false
	for _, txt := range tt.Texts() {
		if strings.HasPrefix(txt, "片源 · ") {
			found = true
		}
	}
	if !found {
		t.Fatalf("group title must say it holds sources: %q", tt.Texts())
	}

	b := New()
	b.settingsPath = ""
	b.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	b.screen, b.sideSel, b.lastSide = screenBrowse, "site:c", "site:c"
	tb := ui.NewTester(b.View, 1200, 800)
	if _, ok := tb.Find("演示源"); !ok {
		t.Fatalf("the group of the source on screen must start open: %q", tb.Texts())
	}
}

// The sidebar's one 管理 entry opens a page whose tabs reach subscriptions,
// sources, the blacklist and the home rows.
func TestManagePageTabs(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
	tt := ui.NewTester(a.View, 1200, 800)
	if err := tt.Click("管理"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.screen != screenSubs || a.sideSel != "manage" {
		t.Fatalf("screen %q side %q", a.screen, a.sideSel)
	}
	// In order: the last tab clicked is the one that must come back.
	for _, tab := range [][2]string{{"黑名单 0", screenBlacklist}, {"片源 0", screenSources}} {
		label, want := tab[0], tab[1]
		if err := tt.Click(label); err != nil {
			t.Fatalf("%s: %v (%q)", label, err, tt.Texts())
		}
		tt.Frame()
		if a.screen != want {
			t.Fatalf("%s opened %q", label, a.screen)
		}
	}
	// Leaving and coming back reopens the last tab.
	a.goHome()
	a.sideSel = "manage"
	a.onSide()
	if a.screen != screenSources {
		t.Fatalf("manage must reopen the last tab, got %q", a.screen)
	}
}

// The appearance follows the system unless light or dark is chosen; the
// choice persists, and settings from before the option keep their look.
func TestAppearanceFollowsSystemOrChoice(t *testing.T) {
	if !resolveLight("", false) || resolveLight("", true) || !resolveLight(appearanceSystem, false) {
		t.Fatal("no choice and 跟随系统 follow the system")
	}
	if !resolveLight(appearanceLight, true) || resolveLight(appearanceDark, false) {
		t.Fatal("a choice overrides the system")
	}

	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{}
	a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
	tt := ui.NewTester(a.View, 1200, 800)
	tt.SetDark(true)
	if a.light {
		t.Fatal("a dark system must show dark by default")
	}
	tt.SetDark(false)
	if !a.light {
		t.Fatal("a light system must show light by default")
	}
	a.openSettings()
	tt.Frame()
	for _, want := range []string{"跟随系统", "浅色", "深色"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %s: %q", want, tt.Texts())
		}
	}
	if tt.HasText("管理黑名单") {
		t.Fatal("the blacklist must be gone from the settings")
	}
	if err := tt.Click("深色"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.appearance != appearanceDark || a.light {
		t.Fatalf("dark chosen: %q light %v", a.appearance, a.light)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	for in, want := range map[string]string{
		`{"appearance":"system"}`: appearanceSystem,
		`{"light":true}`:          appearanceLight,
		`{"volume":0.5}`:          appearanceDark, // saved before the option: it was dark
	} {
		if err := os.WriteFile(path, []byte(in), 0o644); err != nil {
			t.Fatal(err)
		}
		b := New()
		b.settingsPath = path
		b.RestoreSettings()
		if b.appearance != want {
			t.Errorf("%s restored %q, want %q", in, b.appearance, want)
		}
	}
	if New().appearance != "" {
		t.Fatal("a fresh install follows the system")
	}
}

// Long episode names get wider buttons and show whole on hover.
func TestLongEpisodeNamesWidenAndShowOnHover(t *testing.T) {
	if episodeCell(longestEpisode([]model.Episode{{Name: "第12集"}})) != 104 {
		t.Fatal("short names keep narrow buttons")
	}
	long := "20241101期：嘉宾挑战极限任务"
	if episodeCell(longestEpisode([]model.Episode{{Name: "第1集"}, {Name: long}})) <= 104 {
		t.Fatal("a long name widens the buttons")
	}
	a := New()
	tt := ui.NewTester(func(c *ui.Context) {
		th := a.useTheme(c)
		ui.Column(c).Width(120).Children(func() {
			a.choice(c, th, "ep", long, false, func() {})
		})
	}, 400, 200)
	r, ok := tt.Find(long)
	if !ok {
		t.Fatalf("texts %q", tt.Texts())
	}
	tt.Move(r.X+10, r.Y+r.H/2)
	tt.Advance(time.Second)
	tt.Frame()
	n := 0
	for _, s := range tt.Texts() {
		if s == long {
			n++
		}
	}
	if n < 2 {
		t.Fatalf("hovering must show the whole name in a tooltip: %q", tt.Texts())
	}
}

// The embedded logo decodes, and shows in the sidebar and on the welcome
// screen in place of the old drawn mark.
func TestLogoShows(t *testing.T) {
	bmp := logo()
	if bmp == nil {
		t.Fatal("the embedded logo must decode")
	}
	if w, h := bmp.Size(); w != 128 || h != 128 {
		t.Fatalf("logo %dx%d", w, h)
	}
	a := New()
	a.settingsPath = ""
	tt := ui.NewTester(a.View, 1200, 800)
	if _, ok := tt.Access("QuickBo"); !ok {
		t.Fatal("the welcome screen must show the logo")
	}
	if dir := os.Getenv("QUICKBO_SHOT_DIR"); dir != "" {
		savePNG(t, filepath.Join(dir, "welcome.png"), tt.Image())
		a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
		a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
		tt.Frame()
		savePNG(t, filepath.Join(dir, "home.png"), tt.Image())
	}
}
