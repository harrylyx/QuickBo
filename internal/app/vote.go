package app

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

const (
	// hotLaneID is the bucket of front-page candidates: what each source
	// lists first. They have no row of their own; they feed latestRuleID's.
	hotLaneID    = "hot"
	hotLaneLimit = 18
	// latestRuleID is the 最新影视 row. It merges the front-page candidates
	// with the categories its keywords match, and ranks them with
	// rankedLane: how many sources list a title, then sharpness, then year.
	latestRuleID = "new"
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
		if rule.ID == "" || rule.ID == hotLaneID || strings.TrimSpace(rule.Name) == "" {
			continue
		}
		var lane homeLane
		var part map[string][]titleHit
		if rule.ID == latestRuleID {
			lane, part = rankedLane(rule.ID, rule.Name, filterBucket(ballots, hotLaneID, rule.ID))
		} else {
			lane, part = votedLane(rule.ID, rule.Name, filterBucket(ballots, rule.ID))
		}
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

// filterBucket keeps the ballots cast in any of buckets.
func filterBucket(ballots []ballot, buckets ...string) []ballot {
	out := make([]ballot, 0, len(ballots))
	for _, b := range ballots {
		for _, bucket := range buckets {
			if b.Bucket == bucket {
				out = append(out, b)
				break
			}
		}
	}
	return out
}

type laneCandidate struct {
	item    model.Item
	order   []string
	hits    map[string]titleHit
	voters  map[string]struct{}
	clarity int
	year    int
}

func groupBallots(ballots []ballot) ([]string, map[string]*laneCandidate) {
	by := map[string]*laneCandidate{}
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
			a = &laneCandidate{hits: map[string]titleHit{}, voters: map[string]struct{}{}}
			by[id] = a
			keys = append(keys, id)
		}
		if v := b.voter(); v != "" {
			a.voters[v] = struct{}{}
		}
		if score := clarityScore(b.Item.Remarks); score > a.clarity {
			a.clarity = score
		}
		if year := itemYear(b.Item.Year); year > a.year {
			a.year = year
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
		if a.item.Remarks == "" {
			a.item.Remarks = b.Item.Remarks
		}
	}
	return keys, by
}

func itemYear(raw string) int {
	for i := 0; i+4 <= len(raw); i++ {
		n, err := strconv.Atoi(raw[i : i+4])
		if err == nil && n >= 1900 && n <= 2100 {
			return n
		}
	}
	return 0
}

func laneFromCandidates(key, name string, keys []string, by map[string]*laneCandidate, limit int) (homeLane, map[string][]titleHit) {
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
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

func votedLane(key, name string, ballots []ballot) (homeLane, map[string][]titleHit) {
	keys, by := groupBallots(ballots)
	sort.SliceStable(keys, func(i, j int) bool {
		ai, aj := by[keys[i]], by[keys[j]]
		if len(ai.voters) != len(aj.voters) {
			return len(ai.voters) > len(aj.voters)
		}
		return ai.item.Title < aj.item.Title
	})
	return laneFromCandidates(key, name, keys, by, 18)
}

// rankedLane aggregates front-page candidates (and whatever else ballots
// holds) instead of depending on a source category being named 热门: titles
// listed by more sources come first (mirrors share one voter), then
// sharpness, recency and title provide deterministic tie-breakers.
func rankedLane(key, name string, ballots []ballot) (homeLane, map[string][]titleHit) {
	keys, by := groupBallots(ballots)
	sort.SliceStable(keys, func(i, j int) bool {
		ai, aj := by[keys[i]], by[keys[j]]
		if len(ai.voters) != len(aj.voters) {
			return len(ai.voters) > len(aj.voters)
		}
		if ai.clarity != aj.clarity {
			return ai.clarity > aj.clarity
		}
		if ai.year != aj.year {
			return ai.year > aj.year
		}
		if ai.item.Title != aj.item.Title {
			return ai.item.Title < aj.item.Title
		}
		return keys[i] < keys[j]
	})
	return laneFromCandidates(key, name, keys, by, hotLaneLimit)
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
	// Checking is set while the background pass is still reading this
	// source's detail and measuring its picture (loadOffers).
	Checking bool
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
			Remarks: h.Remarks, Clarity: clarityScore(h.Remarks), Checking: true,
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
	// Paint the best source's detail at once; the full scan (detail + speed
	// measurement of every offering source) keeps running in the background and
	// may later swap in a better source.
	if src := srcs[a.offers[0].SiteKey]; src != nil {
		a.fetchOfferDetail(src, a.offers[0].ItemID, hits[0].Title, hits[0].Pic, gen)
	}
	go a.loadOffers(gen, offers, srcs, parses)
}

// fetchOfferDetail resolves a single source's detail immediately so the
// detail page paints without waiting for the cross-source measurement pass.
// The full offer scan is the final authority and may override this later.
func (a *App) fetchOfferDetail(src source.Source, itemID, title, pic string, gen int) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		d, err := src.Detail(ctx, itemID)
		if err != nil {
			// Leave detailLoading set; loadOffers will surface the outcome.
			return
		}
		a.post(func() {
			if a.gen != gen || a.detail != nil {
				return
			}
			if d.Title == "" {
				d.Title = title
			}
			if d.Pic == "" {
				d.Pic = pic
			}
			a.detail = &d
			a.linePick = model.PreferredLine(d.Lines)
			a.detailLoading = false
		})
	}()
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
			// Report each source as it finishes, so the page shows the check
			// moving instead of sitting silent until every source is done.
			// The order and clarity groups change only at the end, so chips
			// do not jump around under the pointer meanwhile.
			defer func() {
				r := out[i]
				a.post(func() {
					if a.gen != gen {
						return
					}
					for k := range a.offers {
						if a.offers[k].SiteKey != o.SiteKey {
							continue
						}
						a.offers[k].Checking = false
						if r.measured {
							a.offers[k].Width, a.offers[k].Height = r.w, r.h
						}
						if r.err != "" {
							a.offers[k].Err = r.err
						}
					}
				})
			}()
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
			w, h := a.measureOffer(mctx, src, d, parses)
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
				a.offers[i].Checking = false
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
	if len(a.offers) == 0 {
		a.detail = nil
		return
	}
	idx := a.offerPick
	if idx < 0 || idx >= len(a.offers) || a.offers[idx].Detail == nil {
		// Prefer a source whose detail actually loaded; never blank a good
		// detail just because a higher-estimated source failed to resolve.
		idx = -1
		for i, o := range a.offers {
			if o.Detail != nil {
				idx = i
				break
			}
		}
		if idx < 0 {
			idx = 0
		}
	}
	o := a.offers[idx]
	a.offerPick = idx
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

const ballotCatLimit = 16

// selectBallotCats keeps one category per homepage rule. Front pages and junk
// genres are left out.
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
			if used[id] > 0 {
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

func (a *App) pickOffer(i int) {
	if i < 0 || i >= len(a.offers) {
		return
	}
	a.offerPick = i
	a.applyOffer()
}
