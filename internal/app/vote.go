package app

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

// ballot is one title a source listed inside a home bucket.
type ballot struct {
	SiteKey  string
	SiteName string
	Bucket   string
	Voter    string
	Cat      string
	Item     model.Item
}

func (b ballot) voter() string {
	if b.Voter != "" {
		return b.Voter
	}
	return b.SiteKey
}

// titleHit is one source that carries a title.
type titleHit struct {
	SiteKey  string
	SiteName string
	ItemID   string
	Title    string
	Year     string
	Pic      string
	Remarks  string
}

const titlePrefix = "title:"

func normTitle(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "/|"); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("·・-—_（）()[]【】", r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
	return s
}

func bucketsFor(name string) []string {
	return matchRules(name, nil)
}

// tally turns ballots into home rows and a title index.
// A title's rank in a row is how many different sources listed it there.
// Row order and names come from the homepage rules.
func tally(ballots []ballot) ([]homeLane, map[string][]titleHit) {
	return tallyRules(ballots, nil)
}

func tallyRules(ballots []ballot, rules []homeRule) ([]homeLane, map[string][]titleHit) {
	if len(rules) == 0 {
		rules = defaultHomeRules()
	}
	ballots = expandBallots(ballots, rules)
	lanes := make([]homeLane, 0, len(rules))
	lookup := map[string][]titleHit{}
	for _, rule := range rules {
		if rule.ID == "" || strings.TrimSpace(rule.Name) == "" {
			continue
		}
		lane, part := votedLane(rule.ID, rule.Name, filterBucket(ballots, rule.ID))
		if len(lane.Items) == 0 {
			continue
		}
		lanes = append(lanes, lane)
		mergeLookup(lookup, part)
	}
	return lanes, lookup
}

// expandBallots applies the current keywords to ballots that remember the
// source category. Ballots that already carry a bucket are left as they are.
func expandBallots(ballots []ballot, rules []homeRule) []ballot {
	out := make([]ballot, 0, len(ballots))
	for _, b := range ballots {
		if strings.TrimSpace(b.Cat) == "" {
			out = append(out, b)
			continue
		}
		for _, id := range matchRules(b.Cat, rules) {
			next := b
			next.Bucket = id
			out = append(out, next)
		}
	}
	return out
}

func filterBucket(ballots []ballot, bucket string) []ballot {
	out := make([]ballot, 0, len(ballots))
	for _, b := range ballots {
		if b.Bucket == bucket {
			out = append(out, b)
		}
	}
	return out
}

func votedLane(key, name string, ballots []ballot) (homeLane, map[string][]titleHit) {
	type acc struct {
		item   model.Item
		order  []string
		hits   map[string]titleHit
		voters map[string]struct{}
	}
	by := map[string]*acc{}
	var keys []string
	for _, b := range ballots {
		if !usableTitle(b.Item.Title) {
			continue
		}
		id := normTitle(b.Item.Title)
		if id == "" || b.SiteKey == "" {
			continue
		}
		a := by[id]
		if a == nil {
			a = &acc{hits: map[string]titleHit{}, voters: map[string]struct{}{}}
			by[id] = a
			keys = append(keys, id)
		}
		if v := b.voter(); v != "" {
			a.voters[v] = struct{}{}
		}
		if _, ok := a.hits[b.SiteKey]; ok {
			continue
		}
		hit := titleHit{
			SiteKey: b.SiteKey, SiteName: b.SiteName, ItemID: b.Item.ID,
			Title: b.Item.Title, Year: b.Item.Year, Pic: b.Item.Pic, Remarks: b.Item.Remarks,
		}
		a.hits[b.SiteKey] = hit
		a.order = append(a.order, b.SiteKey)
		if a.item.Title == "" {
			a.item = b.Item
		}
		if a.item.Pic == "" {
			a.item.Pic = b.Item.Pic
		}
		if a.item.Year == "" {
			a.item.Year = b.Item.Year
		}
	}
	sort.SliceStable(keys, func(i, j int) bool {
		ai, aj := by[keys[i]], by[keys[j]]
		if len(ai.voters) != len(aj.voters) {
			return len(ai.voters) > len(aj.voters)
		}
		return ai.item.Title < aj.item.Title
	})
	if len(keys) > 18 {
		keys = keys[:18]
	}
	lane := homeLane{Key: key, Name: name}
	part := map[string][]titleHit{}
	for _, id := range keys {
		a := by[id]
		item := a.item
		item.ID = titlePrefix + id
		lane.Items = append(lane.Items, item)
		hits := make([]titleHit, 0, len(a.order))
		for _, site := range a.order {
			hits = append(hits, a.hits[site])
		}
		part[id] = hits
	}
	return lane, part
}

func mergeLookup(dst, src map[string][]titleHit) {
	for id, hits := range src {
		have := map[string]bool{}
		for _, h := range dst[id] {
			have[h.SiteKey] = true
		}
		for _, h := range hits {
			if have[h.SiteKey] {
				continue
			}
			dst[id] = append(dst[id], h)
			have[h.SiteKey] = true
		}
	}
}

func clarityScore(text string) int {
	s := strings.ToLower(text)
	switch {
	case strings.Contains(s, "4k") || strings.Contains(s, "2160"):
		return 50
	case strings.Contains(s, "1080") || strings.Contains(s, "蓝光") || strings.Contains(s, "bluray") || strings.Contains(s, "blu-ray"):
		return 40
	case strings.Contains(s, "720"):
		return 30
	case strings.Contains(s, "hd"):
		return 20
	case strings.Contains(s, "ts") || strings.Contains(s, "cam") || strings.Contains(s, "枪版"):
		return 5
	default:
		return 10
	}
}

func clarityLabel(n int) string {
	switch {
	case n >= 50:
		return "4K"
	case n >= 40:
		return "1080P"
	case n >= 30:
		return "720P"
	case n >= 20:
		return "HD"
	case n > 0 && n <= 5:
		return "低清"
	default:
		return ""
	}
}

// scoreFromPixels maps a measured frame to the same buckets as the text labels.
// Zero means the picture size was not readable.
func scoreFromPixels(w, h int) int {
	if w <= 0 || h <= 0 {
		return 0
	}
	long, short := w, h
	if h > w {
		long, short = h, w
	}
	switch {
	case long >= 3000 || short >= 1600:
		return 50
	case long >= 1600 || short >= 900:
		return 40
	case long >= 1200 || short >= 700:
		return 30
	case long >= 700 || short >= 400:
		return 20
	default:
		return 5
	}
}

// playOffer is one source's copy of the title open on the detail page.
type playOffer struct {
	SiteKey  string
	SiteName string
	ItemID   string
	Remarks  string
	Detail   *model.Detail
	Err      string
	Clarity  int
	Width    int
	Height   int
}

func (a *App) sortOffers() {
	picked := ""
	if a.offerPick >= 0 && a.offerPick < len(a.offers) {
		picked = a.offers[a.offerPick].SiteKey
	}
	sort.SliceStable(a.offers, func(i, j int) bool {
		if a.offers[i].Clarity != a.offers[j].Clarity {
			return a.offers[i].Clarity > a.offers[j].Clarity
		}
		di, dj := a.delayOf(a.offers[i].SiteKey), a.delayOf(a.offers[j].SiteKey)
		if di != dj {
			return di < dj
		}
		return a.offers[i].SiteName < a.offers[j].SiteName
	})
	if picked == "" {
		return
	}
	for i, o := range a.offers {
		if o.SiteKey == picked {
			a.offerPick = i
			return
		}
	}
}

func (a *App) delayOf(key string) int {
	st := a.stats[key]
	if st.Delay <= 0 {
		return 1 << 30
	}
	return st.Delay
}

func (a *App) mergeBallots(siteKey string, fresh []ballot) {
	kept := make([]ballot, 0, len(a.ballots)+len(fresh))
	for _, b := range a.ballots {
		if b.SiteKey != siteKey {
			kept = append(kept, b)
		}
	}
	a.ballots = append(kept, fresh...)
	a.rebuildHome()
}

func (a *App) rebuildHome() {
	lanes, lookup := tallyRules(a.ballots, a.activeRules())
	a.home = lanes
	if a.lookup == nil {
		a.lookup = map[string][]titleHit{}
	}
	a.lookup = lookup
}

func (a *App) openTitle(key string) {
	hits := a.hitsFor(key)
	if len(hits) == 0 {
		return
	}
	a.screen = screenDetail
	a.detailTitle = hits[0].Title
	a.detail = nil
	a.detailErr = ""
	a.detailLoading = true
	a.linePick = 0
	a.closePlayer()
	a.offers = make([]playOffer, len(hits))
	srcs := map[string]source.Source{}
	for i, h := range hits {
		a.offers[i] = playOffer{
			SiteKey: h.SiteKey, SiteName: h.SiteName, ItemID: h.ItemID,
			Remarks: h.Remarks, Clarity: clarityScore(h.Remarks),
		}
		if a.catalog != nil {
			if s := a.catalog.Find(h.SiteKey); s != nil {
				srcs[h.SiteKey] = s
			}
		}
	}
	a.sortOffers()
	a.offerPick = 0
	a.siteKey = a.offers[0].SiteKey
	offers := append([]playOffer(nil), a.offers...)
	var parses []config.ParseItem
	if a.cfg != nil {
		parses = a.cfg.Parses
	}
	gen := a.bump()
	go a.loadOffers(gen, offers, srcs, parses)
}

func (a *App) hitsFor(key string) []titleHit {
	if a.committed != "" {
		if hits := a.searchLookup[key]; len(hits) > 0 {
			return hits
		}
	}
	return a.lookup[key]
}

func (a *App) loadOffers(gen int, offers []playOffer, srcs map[string]source.Source, parses []config.ParseItem) {
	type result struct {
		detail   *model.Detail
		err      string
		score    int
		measured bool
		w        int
		h        int
	}
	out := make([]result, len(offers))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i, o := range offers {
		wg.Add(1)
		go func(i int, o playOffer) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			src := srcs[o.SiteKey]
			if src == nil {
				out[i].err = "这个源已经不在列表里"
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), indexProbeTimeout)
			d, err := src.Detail(ctx, o.ItemID)
			cancel()
			if err != nil {
				out[i].err = err.Error()
				return
			}
			out[i].detail = &d
			out[i].score = clarityScore(o.Remarks + " " + episodeText(d))
			mctx, mcancel := context.WithTimeout(context.Background(), 10*time.Second)
			w, h := measureOffer(mctx, src, d, parses)
			mcancel()
			if score := scoreFromPixels(w, h); score > 0 {
				out[i].score = score
				out[i].measured = true
				out[i].w, out[i].h = w, h
			}
		}(i, o)
	}
	wg.Wait()
	a.post(func() {
		if a.gen != gen {
			return
		}
		picked := ""
		if a.offerPick >= 0 && a.offerPick < len(a.offers) {
			picked = a.offers[a.offerPick].SiteKey
		}
		for i := range a.offers {
			for j, o := range offers {
				if a.offers[i].SiteKey != o.SiteKey {
					continue
				}
				a.offers[i].Detail = out[j].detail
				a.offers[i].Err = out[j].err
				if out[j].measured {
					a.offers[i].Clarity = out[j].score
					a.offers[i].Width = out[j].w
					a.offers[i].Height = out[j].h
				} else if out[j].score > a.offers[i].Clarity {
					a.offers[i].Clarity = out[j].score
				}
			}
		}
		a.sortOffers()
		a.detailLoading = false
		a.offerPick = 0
		for i, o := range a.offers {
			if o.SiteKey == picked {
				a.offerPick = i
				break
			}
		}
		a.applyOffer()
	})
}

func episodeText(d model.Detail) string {
	var b strings.Builder
	for _, ln := range d.Lines {
		b.WriteString(ln.Name)
		b.WriteByte(' ')
		for _, ep := range ln.Episodes {
			b.WriteString(ep.Name)
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func (a *App) applyOffer() {
	if a.offerPick < 0 || a.offerPick >= len(a.offers) {
		a.detail = nil
		return
	}
	o := a.offers[a.offerPick]
	a.siteKey = o.SiteKey
	a.detail = o.Detail
	a.detailErr = o.Err
	a.linePick = 0
	if a.detail != nil && a.detail.Title != "" {
		a.detailTitle = a.detail.Title
	}
}

type catPick struct {
	cat     model.Category
	buckets []string
}

const (
	ballotCatLimit = 16
	ballotHotLimit = 4
)

// selectBallotCats keeps one category per homepage rule, and up to four
// for the hot rule. Front pages and junk genres are left out.
func selectBallotCats(cats []model.Category) []catPick {
	return selectBallotCatsRules(cats, nil)
}

func selectBallotCatsRules(cats []model.Category, rules []homeRule) []catPick {
	if len(rules) == 0 {
		rules = defaultHomeRules()
	}
	order := map[string]int{}
	for i, rule := range rules {
		if rule.ID != "" {
			if _, ok := order[rule.ID]; !ok {
				order[rule.ID] = i
			}
		}
	}
	type cand struct {
		cat model.Category
		b   []string
		pri int
	}
	var cs []cand
	for _, cat := range cats {
		b := matchRules(cat.Name, rules)
		if len(b) == 0 {
			continue
		}
		pri := order[b[0]]
		cs = append(cs, cand{cat: cat, b: b, pri: pri})
	}
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].pri < cs[j].pri })
	var out []catPick
	used := map[string]int{}
	for _, c := range cs {
		if len(out) >= ballotCatLimit {
			break
		}
		keep := make([]string, 0, len(c.b))
		for _, id := range c.b {
			limit := 1
			if id == "hot" {
				limit = ballotHotLimit
			}
			if used[id] >= limit {
				continue
			}
			keep = append(keep, id)
		}
		if len(keep) == 0 {
			continue
		}
		out = append(out, catPick{cat: c.cat, buckets: keep})
		for _, id := range keep {
			used[id]++
		}
	}
	return out
}

func voterFor(meta source.Meta) string {
	if meta.Kind == config.KindCMSJSON || meta.Kind == config.KindCMSXML {
		if h := cmsHost(meta.API); h != "" {
			return "host:" + h
		}
	}
	if meta.Key != "" {
		return "site:" + meta.Key
	}
	return ""
}

func cmsHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func usableTitle(s string) bool {
	s = strings.TrimSpace(s)
	switch s {
	case "", "首页", "更多", "登录", "注册", "广告", "测试", "undefined", "null", "排行榜":
		return false
	}
	n := 0
	letter := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
			if unicode.IsLetter(r) {
				letter = true
			}
		}
	}
	return letter && n >= 2
}

func collectBallots(ctx context.Context, s source.Source, rules []homeRule) []ballot {
	cats, err := s.Categories(ctx)
	if err != nil || len(cats) == 0 || s.Meta().Key == "" {
		return nil
	}
	m := s.Meta()
	picks := selectBallotCatsRules(cats, rules)
	if len(picks) == 0 {
		return nil
	}
	voter := voterFor(m)
	got := make([][]model.Item, len(picks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, p := range picks {
		wg.Add(1)
		go func(i int, p catPick) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			page, err := s.List(ctx, p.cat.ID, 1)
			if err != nil || len(page.Items) == 0 {
				return
			}
			items := page.Items
			if len(items) > 16 {
				items = items[:16]
			}
			got[i] = items
		}(i, p)
	}
	wg.Wait()
	var out []ballot
	for i, p := range picks {
		for _, bucket := range p.buckets {
			for _, it := range got[i] {
				if !usableTitle(it.Title) {
					continue
				}
				out = append(out, ballot{
					SiteKey: m.Key, SiteName: m.Name, Bucket: bucket, Voter: voter, Cat: p.cat.Name, Item: it,
				})
			}
		}
	}
	return out
}

func (a *App) pickOffer(i int) {
	if i < 0 || i >= len(a.offers) {
		return
	}
	a.offerPick = i
	a.applyOffer()
}
