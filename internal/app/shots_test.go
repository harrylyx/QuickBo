package app

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

// TestShots renders the main screens to PNGs for eyeballing design changes
// without launching the app: QUICKBO_SHOTS=/some/dir go test ./internal/app
// -run TestShots. Posters are made-up gradients; nothing touches the network.
func TestShots(t *testing.T) {
	dir := os.Getenv("QUICKBO_SHOTS")
	if dir == "" {
		t.Skip("set QUICKBO_SHOTS=<dir> to render screenshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, light := range []bool{false, true} {
		mode := "dark"
		if light {
			mode = "light"
		}
		for _, sc := range []struct {
			name string
			set  func(a *App)
		}{
			{"home", func(a *App) {}},
			{"detail", shotDetail},
			{"sources", func(a *App) { a.screen, a.sideSel, a.lastSide = screenSources, "sources", "sources" }},
			{"play", shotPlay},
			{"settings", func(a *App) {
				a.settingsOpen = true
				for i := 1; i <= 40; i++ {
					a.blacklist = append(a.blacklist, "关键字"+itoa(i))
				}
			}},
		} {
			a := shotApp(light)
			sc.set(a)
			tt := ui.NewTester(a.View, 1280, 820)
			for i := 0; i < 4; i++ {
				tt.Advance(200 * time.Millisecond)
			}
			savePNG(t, filepath.Join(dir, mode+"-"+sc.name+".png"), tt.Image())
		}
	}
}

var shotTitles = []string{"长夜将明", "雾港", "北方的海", "失物招领", "第七个夏天", "银河旅馆", "山海之间", "灯塔守望者", "春日迟迟", "无声告白"}

func shotApp(light bool) *App {
	a := New()
	a.settingsPath = ""
	a.light = light
	a.lib = &library{}
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}, otherFake{}}}
	a.allSites = a.catalog.Sites
	a.subs = []subscription{{Ref: "https://example.invalid/tv.json", Name: "我的订阅", Enabled: true}}
	a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
	remarks := []string{"HD", "更新至12集", "1080P", "", "完结", "4K", "", "TC", "更新至3集", ""}
	for li, name := range []string{"最新影视", "电影", "电视剧", "动漫"} {
		lane := homeLane{Key: "lane" + string(rune('a'+li)), Name: name}
		for i, title := range shotTitles {
			pic := "https://example.invalid/p/" + name + title
			img := poster(li*len(shotTitles) + i)
			blurred, tint := blurImage(img)
			a.rememberImage(pic, img, blurred, tint)
			lane.Items = append(lane.Items, model.Item{ID: titlePrefix + title, Title: title, Pic: pic, Remarks: remarks[i], Year: "2025"})
		}
		a.home = append(a.home, lane)
	}
	a.stats = map[string]sourceStat{"c": {Delay: 182}, "d": {Err: "HTTP 404", Delay: -1}}
	return a
}

func shotDetail(a *App) {
	pic := a.home[0].Items[0].Pic
	d := model.Detail{
		Title: "长夜将明", Pic: pic, Year: "2025", Area: "大陆", TypeName: "剧情", Remarks: "更新至12集",
		Director: "某导演", Actors: "演员甲 / 演员乙 / 演员丙",
		Content: "一座沿海小城里，一桩尘封多年的旧案被重新翻开。刑警与记者在雨夜相遇，循着线索走进彼此的过去，也走进这座城市不愿被提起的秘密。",
	}
	for _, ln := range []string{"高清线路", "备用线路"} {
		line := model.Line{Name: ln}
		for i := 1; i <= 24; i++ {
			line.Episodes = append(line.Episodes, model.Episode{Name: "第" + itoa(i) + "集", URL: "https://example.invalid/" + itoa(i) + ".m3u8"})
		}
		d.Lines = append(d.Lines, line)
	}
	a.screen, a.detail, a.detailTitle = screenDetail, &d, d.Title
	a.offers = []playOffer{
		{SiteKey: "c", SiteName: "演示源", Clarity: 40, Width: 1920, Height: 1080, Detail: &d},
		{SiteKey: "x", SiteName: "极速资源", Clarity: 40, Width: 1920, Height: 1080, Detail: &d},
		{SiteKey: "y", SiteName: "量子云", Clarity: 30, Width: 1280, Height: 720, Detail: &d},
		{SiteKey: "z", SiteName: "失效站", Clarity: 10, Unplayable: true, Reason: "地址打不开（HTTP 404）"},
	}
}

func shotPlay(a *App) {
	shotDetail(a)
	a.screen = screenPlay
	a.playTitle = "长夜将明 第1集"
	a.player = &shotVideo{}
	a.controlsUntil = time.Now().Add(time.Hour)
}

// shotVideo is an embedded player paused 23 minutes into a 45-minute
// episode, showing a still frame.
type shotVideo struct{ videoFake }

func (*shotVideo) Ready() bool             { return true }
func (*shotVideo) Position() time.Duration { return 23 * time.Minute }
func (*shotVideo) Duration() time.Duration { return 45 * time.Minute }
func (*shotVideo) Buffered() time.Duration { return 3 * time.Minute }
func (v *shotVideo) TakeFrame() *image.RGBA {
	if v.taken || v.w == 0 {
		return nil
	}
	v.taken = true
	return still(v.w, v.h)
}

// poster is a 2:3 gradient, a different hue for each n.
func poster(n int) *image.RGBA {
	const w, h = 240, 360
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	hue := float64(n*47%360) / 360
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			fy := float64(y) / h
			glow := math.Exp(-math.Pow((float64(x)/w-0.6)*2.2, 2) - math.Pow((fy-0.35)*2.5, 2))
			r, g, b := hsl(hue+0.08*fy, 0.55, 0.18+0.42*glow)
			img.SetRGBA(x, y, color.RGBA{r, g, b, 255})
		}
	}
	return img
}

// still is a dusky landscape frame of w×h.
func still(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		fy := float64(y) / float64(h)
		for x := 0; x < w; x++ {
			fx := float64(x) / float64(w)
			sun := math.Exp(-math.Pow((fx-0.7)*4, 2) - math.Pow((fy-0.45)*6, 2))
			var r, g, b uint8
			if fy > 0.62+0.05*math.Sin(fx*9) {
				r, g, b = hsl(0.62, 0.35, 0.08+0.05*(1-fy))
			} else {
				r, g, b = hsl(0.05+0.5*fy, 0.6, 0.25+0.5*sun+0.15*(1-fy))
			}
			img.SetRGBA(x, y, color.RGBA{r, g, b, 255})
		}
	}
	return img
}

func hsl(h, s, l float64) (uint8, uint8, uint8) {
	h -= math.Floor(h)
	l = math.Min(math.Max(l, 0), 1)
	q := l + s - l*s
	if l < 0.5 {
		q = l * (1 + s)
	}
	p := 2*l - q
	f := func(t float64) uint8 {
		t -= math.Floor(t)
		var v float64
		switch {
		case t < 1.0/6:
			v = p + (q-p)*6*t
		case t < 0.5:
			v = q
		case t < 2.0/3:
			v = p + (q-p)*(2.0/3-t)*6
		default:
			v = p
		}
		return uint8(math.Round(v * 255))
	}
	return f(h + 1.0/3), f(h), f(h - 1.0/3)
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func savePNG(t *testing.T, path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
