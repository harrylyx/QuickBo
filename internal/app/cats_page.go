package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/model"
)

// catFact is one category name a source published. The id is the source's
// own category id; opening the aggregate page looks the name up again.
type catFact struct {
	SiteKey  string
	SiteName string
	Name     string
	ID       string
}

// aggEntry is one row of the category directory.
type aggEntry struct {
	Key     string
	Name    string
	Sources int
}

// replaceCatFacts stores the usable category names of one source, replacing
// whatever that source contributed before.
func (a *App) replaceCatFacts(siteKey, siteName string, cats []model.Category) {
	out := make([]catFact, 0, len(a.catFacts)+len(cats))
	for _, f := range a.catFacts {
		if f.SiteKey != siteKey {
			out = append(out, f)
		}
	}
	seen := map[string]bool{}
	for _, cat := range cats {
		name := strings.TrimSpace(cat.Name)
		if name == "" || seen[name] || junkCategory(name) || !usableTitle(name) {
			continue
		}
		seen[name] = true
		out = append(out, catFact{SiteKey: siteKey, SiteName: siteName, Name: name, ID: cat.ID})
	}
	a.catFacts = out
}

// categoryDirectory folds recorded category names into the home rules, then
// adds a name of its own when at least two sources use it and no rule covers it.
func (a *App) categoryDirectory() []aggEntry {
	type pair struct{ site, name string }
	seen := map[pair]bool{}
	var facts []catFact
	for _, f := range a.catFacts {
		name := strings.TrimSpace(f.Name)
		if name == "" || junkCategory(name) || !usableTitle(name) {
			continue
		}
		p := pair{f.SiteKey, name}
		if seen[p] {
			continue
		}
		seen[p] = true
		facts = append(facts, catFact{SiteKey: f.SiteKey, Name: name, ID: f.ID})
	}
	rules := a.activeRules()
	covered := map[pair]bool{}
	var out []aggEntry
	for _, rule := range rules {
		sites := map[string]bool{}
		for _, f := range facts {
			ids := matchRules(f.Name, rules)
			hit := false
			for _, id := range ids {
				if id == rule.ID {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
			sites[f.SiteKey] = true
			covered[pair{f.SiteKey, f.Name}] = true
		}
		if len(sites) == 0 {
			continue
		}
		out = append(out, aggEntry{Key: rule.ID, Name: rule.Name, Sources: len(sites)})
	}
	byName := map[string]map[string]bool{}
	var order []string
	for _, f := range facts {
		if covered[pair{f.SiteKey, f.Name}] {
			continue
		}
		if byName[f.Name] == nil {
			byName[f.Name] = map[string]bool{}
			order = append(order, f.Name)
		}
		byName[f.Name][f.SiteKey] = true
	}
	type left struct {
		name string
		n    int
	}
	var extras []left
	for _, name := range order {
		if n := len(byName[name]); n >= 2 {
			extras = append(extras, left{name, n})
		}
	}
	sort.Slice(extras, func(i, j int) bool {
		if extras[i].n != extras[j].n {
			return extras[i].n > extras[j].n
		}
		return extras[i].name < extras[j].name
	})
	for _, e := range extras {
		out = append(out, aggEntry{Key: "agg:" + e.name, Name: e.name, Sources: e.n})
	}
	return out
}

// openCategory opens the aggregate list for a directory entry. A name no
// home rule covers becomes a one-keyword rule for that lookup.
func (a *App) openCategory(key, name string) {
	if strings.HasPrefix(key, "agg:") {
		a.aggExtra = &homeRule{ID: key, Name: name, Keywords: []string{name}}
	} else {
		a.aggExtra = nil
	}
	a.openMore(homeLane{Key: key, Name: name})
}

func (a *App) viewCats(c *ui.Context, th *ui.Theme) {
	dir := a.categoryDirectory()
	if len(dir) == 0 {
		if a.loading {
			ui.Text(c, "分类会在片源检查完后出现。").TextColor(th.TextMuted).Wrap()
			return
		}
		a.emptyState(c, th, "还没有分类", "片源检查完后，能归并的分类会出现在这里。")
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Row(c).Gap(8).Wrap().Children(func() {
			for _, e := range dir {
				e := e
				label := fmt.Sprintf("%s · %d 个源", e.Name, e.Sources)
				a.choice(c, th, "aggcat:"+e.Key, label, false, func() {
					a.openCategory(e.Key, e.Name)
				})
			}
		})
	})
}
