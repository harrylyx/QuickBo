package app

import (
	"fmt"
	"strings"
)

// homeRule is one homepage row. Keywords are matched against the category
// names a source already publishes. Name is the row title.
type homeRule struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
}

func defaultHomeRules() []homeRule {
	return []homeRule{
		{ID: "new", Name: "最新影视", Keywords: []string{"最新", "最近", "新片"}},
		{ID: "movie", Name: "电影", Keywords: []string{"电影"}},
		{ID: "tv", Name: "电视剧", Keywords: []string{"电视剧", "连续剧", "剧集"}},
		{ID: "anime", Name: "动漫", Keywords: []string{"动漫", "动画", "国漫"}},
		{ID: "region:大陆", Name: "大陆", Keywords: []string{"大陆", "内地", "国产", "国漫"}},
		{ID: "region:香港", Name: "香港", Keywords: []string{"香港", "港剧", "港片", "港台"}},
		{ID: "region:台湾", Name: "台湾", Keywords: []string{"台湾", "台剧", "台片"}},
		{ID: "region:欧美", Name: "欧美", Keywords: []string{"欧美", "美国", "英国", "法国"}},
		{ID: "region:韩国", Name: "韩国", Keywords: []string{"韩国", "韩剧", "韩片", "韩综"}},
		{ID: "region:日本", Name: "日本", Keywords: []string{"日本", "日剧", "日片"}},
		{ID: "region:泰国", Name: "泰国", Keywords: []string{"泰国", "泰剧"}},
	}
}

func (a *App) activeRules() []homeRule {
	if a == nil || a.homeRules == nil {
		return defaultHomeRules()
	}
	return a.homeRules
}

func cloneRules(in []homeRule) []homeRule {
	out := make([]homeRule, len(in))
	for i, rule := range in {
		out[i] = rule
		out[i].Keywords = append([]string(nil), rule.Keywords...)
	}
	return out
}

func normalizeRules(in []homeRule) []homeRule {
	var out []homeRule
	seen := map[string]bool{}
	for _, rule := range in {
		rule.ID = strings.TrimSpace(rule.ID)
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.ID == "" || rule.ID == hotLaneID || rule.Name == "" || seen[rule.ID] {
			continue
		}
		var words []string
		seenWord := map[string]bool{}
		for _, word := range rule.Keywords {
			word = strings.TrimSpace(word)
			if word == "" || seenWord[word] {
				continue
			}
			seenWord[word] = true
			words = append(words, word)
		}
		rule.Keywords = words
		seen[rule.ID] = true
		out = append(out, rule)
	}
	if len(out) == 0 {
		return defaultHomeRules()
	}
	return out
}

// matchRules returns the homepage rows a source category votes in.
// Junk genres stay out of the built-in rows. A custom row can still name them.
func matchRules(name string, rules []homeRule) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if len(rules) == 0 {
		rules = defaultHomeRules()
	}
	junk := junkCategory(name)
	var out []string
	seen := map[string]bool{}
	for _, rule := range rules {
		if rule.ID == "" || seen[rule.ID] || len(rule.Keywords) == 0 {
			continue
		}
		if junk && !strings.HasPrefix(rule.ID, "custom-") {
			continue
		}
		if !keywordHit(name, rule.Keywords) {
			continue
		}
		seen[rule.ID] = true
		out = append(out, rule.ID)
	}
	return out
}

func keywordHit(name string, words []string) bool {
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word != "" && strings.Contains(name, word) {
			return true
		}
	}
	// A hierarchical name (电影/动作, 剧集>内地) should also match keywords
	// against each ancestor segment, not only the raw or leaf string.
	for _, seg := range categorySegments(name) {
		for _, word := range words {
			word = strings.TrimSpace(word)
			if word != "" && strings.Contains(seg, word) {
				return true
			}
		}
	}
	return false
}

// categorySegments splits a hierarchical category name into its ancestor
// chain, so a rule keyword can match any level of the path.
func categorySegments(name string) []string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		switch r {
		case '/', '>', '›', '|', '｜', '—', '-', '·':
			return true
		}
		return false
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && p != name {
			out = append(out, p)
		}
	}
	return out
}

func junkCategory(name string) bool {
	for _, word := range []string{
		"伦理", "倫理", "理论", "理論", "福利", "写真", "寫真", "里番", "裏番",
		"成人", "三级", "三級", "擦边", "擦邊", "解说", "解說", "预告", "預告",
		"体育", "體育", "短剧", "短劇", "直播", "爽文", "脑洞", "腦洞",
	} {
		if strings.Contains(name, word) {
			return true
		}
	}
	return false
}

func parseKeywords(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ',', '，', '、', ';', '；', '\n', '\t', ' ':
			return true
		default:
			return false
		}
	})
	var out []string
	seen := map[string]bool{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		out = append(out, field)
	}
	return out
}

func sameKeywords(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// bindRuleEdits keeps the draft buffers aligned with homeRules. Every render
// indexes ruleNames[i] and ruleWords[i], so a shorter buffer would panic.
func (a *App) bindRuleEdits() {
	n := len(a.homeRules)
	if len(a.ruleNames) == n && len(a.ruleWords) == n && len(a.ruleFocus) == n*2 {
		return
	}
	a.resetRuleEdits()
}

// resetRuleEdits rebuilds the drafts from homeRules, dropping uncommitted text.
func (a *App) resetRuleEdits() {
	n := len(a.homeRules)
	a.ruleNames = make([]string, n)
	a.ruleWords = make([]string, n)
	a.ruleFocus = make([]bool, n*2)
	for i, rule := range a.homeRules {
		a.ruleNames[i] = rule.Name
		a.ruleWords[i] = strings.Join(rule.Keywords, "、")
	}
}

func (a *App) commitRule(i int) {
	a.bindRuleEdits()
	if i < 0 || i >= len(a.homeRules) {
		return
	}
	name := strings.TrimSpace(a.ruleNames[i])
	if name == "" {
		name = a.homeRules[i].Name
		a.ruleNames[i] = name
	}
	words := parseKeywords(a.ruleWords[i])
	if name == a.homeRules[i].Name && sameKeywords(words, a.homeRules[i].Keywords) {
		return
	}
	a.homeRules[i].Name = name
	a.homeRules[i].Keywords = words
	a.rulesDirty = true
}

func (a *App) moveRule(i, dir int) {
	j := i + dir
	if i < 0 || j < 0 || i >= len(a.homeRules) || j >= len(a.homeRules) {
		return
	}
	a.commitRule(i)
	a.commitRule(j)
	a.homeRules[i], a.homeRules[j] = a.homeRules[j], a.homeRules[i]
	a.ruleNames[i], a.ruleNames[j] = a.ruleNames[j], a.ruleNames[i]
	a.ruleWords[i], a.ruleWords[j] = a.ruleWords[j], a.ruleWords[i]
	for k := 0; k < 2; k++ {
		x, y := i*2+k, j*2+k
		if x < len(a.ruleFocus) && y < len(a.ruleFocus) {
			a.ruleFocus[x], a.ruleFocus[y] = a.ruleFocus[y], a.ruleFocus[x]
		}
	}
	a.rulesDirty = true
}

func (a *App) deleteRule(i int) {
	if i < 0 || i >= len(a.homeRules) {
		return
	}
	a.bindRuleEdits()
	a.homeRules = append(a.homeRules[:i], a.homeRules[i+1:]...)
	a.ruleNames = append(a.ruleNames[:i], a.ruleNames[i+1:]...)
	a.ruleWords = append(a.ruleWords[:i], a.ruleWords[i+1:]...)
	// Drop only this row's focus slots. Clearing the whole slice would make
	// bindRuleEdits rebuild every draft and discard uncommitted edits.
	if i*2+2 <= len(a.ruleFocus) {
		a.ruleFocus = append(a.ruleFocus[:i*2], a.ruleFocus[i*2+2:]...)
	}
	a.rulesDirty = true
}

func (a *App) addRule() {
	a.bindRuleEdits()
	id := a.nextRuleID()
	a.homeRules = append(a.homeRules, homeRule{ID: id, Name: "新分类"})
	a.ruleNames = append(a.ruleNames, "新分类")
	a.ruleWords = append(a.ruleWords, "")
	a.ruleFocus = append(a.ruleFocus, false, false)
	a.rulesDirty = true
}

func (a *App) resetRules() {
	a.homeRules = defaultHomeRules()
	// This frame still renders ruleRow, so the drafts must be usable now,
	// not only after the next bindRuleEdits call.
	a.resetRuleEdits()
	a.rulesDirty = true
}

// saveRules is the only point where a rule edit takes effect. It writes the
// settings and recomputes the home from cached facts, so saving never starts
// a new network collection.
func (a *App) saveRules() {
	a.bindRuleEdits()
	for i := range a.homeRules {
		name := strings.TrimSpace(a.ruleNames[i])
		if name == "" {
			name = a.homeRules[i].Name
		}
		a.homeRules[i].Name = name
		a.homeRules[i].Keywords = parseKeywords(a.ruleWords[i])
	}
	a.homeRules = normalizeRules(a.homeRules)
	a.resetRuleEdits()
	a.noteRuleSeq()
	a.rulesDirty = false
	a.saveSettings()
	if len(a.ballots) == 0 {
		// No cached facts to re-tally: collect fresh data so the new layout
		// actually shows up on the homepage.
		a.recollect()
	} else {
		a.rebuildHome()
	}
	a.homeHits = nil
	a.searchLookup = nil
	a.homeSearching = false
	a.screen = screenHome
	a.sideSel = "home"
	a.lastSide = "home"
}

func (a *App) nextRuleID() string {
	for {
		a.ruleSeq++
		id := fmt.Sprintf("custom-%d", a.ruleSeq)
		used := false
		for _, rule := range a.homeRules {
			if rule.ID == id {
				used = true
				break
			}
		}
		if !used {
			return id
		}
	}
}

func (a *App) noteRuleSeq() {
	for _, rule := range a.homeRules {
		var n int
		if _, err := fmt.Sscanf(rule.ID, "custom-%d", &n); err == nil && n > a.ruleSeq {
			a.ruleSeq = n
		}
	}
}
