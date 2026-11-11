package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/player"
)

// parseRankTTL is how long an online parser's track record is kept. Parser
// hosts and configs change often, so stale stats are dropped on load.
const parseRankTTL = 24 * time.Hour

// parseRankEntry is the smoothed track record of one online JSON parser.
type parseRankEntry struct {
	Success   int       `json:"success"`
	Fail      int       `json:"fail"`
	LatencyMS int64     `json:"latencyMs"`
	Last      time.Time `json:"last"`
}

func (a *App) parseRankPath() string {
	if a.settingsPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(a.settingsPath), "parse_rank.json")
}

func (a *App) loadParseRank() map[string]parseRankEntry {
	path := a.parseRankPath()
	m := map[string]parseRankEntry{}
	if path == "" {
		return m
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	if json.Unmarshal(b, &m) != nil {
		return map[string]parseRankEntry{}
	}
	cutoff := time.Now().Add(-parseRankTTL)
	for k, e := range m {
		if e.Last.Before(cutoff) {
			delete(m, k)
		}
	}
	return m
}

func (a *App) saveParseRank(m map[string]parseRankEntry) {
	path := a.parseRankPath()
	if path == "" {
		return
	}
	b, err := json.Marshal(m)
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

// rankParses returns the type-1 parsers ordered by track record: higher
// success rate first, then lower latency, then the order the config listed
// them. A parser with no record sits in the middle, so a fresh parser is still
// tried before known failures. Non-type-1 entries are appended unchanged.
func (a *App) rankParses(parses []config.ParseItem) []config.ParseItem {
	type slot struct {
		p config.ParseItem
		i int
	}
	var list []slot
	for i, p := range parses {
		if p.Type != 1 || strings.TrimSpace(p.URL) == "" {
			continue
		}
		list = append(list, slot{p: p, i: i})
	}

	a.parseMu.Lock()
	if a.parseRank == nil {
		a.parseRank = a.loadParseRank()
	}
	rank := a.parseRank
	a.parseMu.Unlock()

	sort.SliceStable(list, func(x, y int) bool {
		ex, ey := rank[list[x].p.URL], rank[list[y].p.URL]
		if rx, ry := parseSuccess(ex), parseSuccess(ey); rx != ry {
			return rx > ry
		}
		if lx, ly := parseLatency(ex), parseLatency(ey); lx != ly {
			return lx < ly
		}
		return list[x].i < list[y].i
	})

	out := make([]config.ParseItem, 0, len(parses))
	for _, s := range list {
		out = append(out, s.p)
	}
	for _, p := range parses {
		if p.Type != 1 || strings.TrimSpace(p.URL) == "" {
			out = append(out, p)
		}
	}
	return out
}

// applyParsesRanked resolves raw through the online parsers, ordered by their
// track record, and records the outcome for the next attempt.
func (a *App) applyParsesRanked(ctx context.Context, raw string, parses []config.ParseItem) (string, map[string]string, bool) {
	ordered := a.rankParses(parses)
	attempts, stream, headers, ok := player.ApplyParses(ctx, raw, ordered)
	a.recordParseAttempts(attempts)
	return stream, headers, ok
}

// recordParseAttempts folds the latest results into the persistent ranking.
func (a *App) recordParseAttempts(attempts []player.ParseAttempt) {
	if len(attempts) == 0 {
		return
	}
	a.parseMu.Lock()
	defer a.parseMu.Unlock()
	if a.parseRank == nil {
		a.parseRank = a.loadParseRank()
	}
	for _, at := range attempts {
		e := a.parseRank[at.Parser]
		e.Last = time.Now()
		if at.OK {
			e.Success++
			ms := at.Latency.Milliseconds()
			if e.LatencyMS <= 0 {
				e.LatencyMS = ms
			} else {
				// EMA, weighted toward history so one slow response does not
				// flip the ordering.
				e.LatencyMS = (e.LatencyMS*4 + ms) / 5
			}
		} else {
			e.Fail++
		}
		a.parseRank[at.Parser] = e
	}
	a.saveParseRank(a.parseRank)
}

// parseSuccess is the smoothed success rate. A parser with no record gets a
// neutral prior, so it ranks below proven parsers but above known failures.
func parseSuccess(e parseRankEntry) float64 {
	n := e.Success + e.Fail
	return float64(e.Success+1) / float64(n+2)
}

// parseLatency returns the smoothed average latency in ms, defaulting to a
// neutral value when a parser has no timing data yet.
func parseLatency(e parseRankEntry) float64 {
	if e.LatencyMS <= 0 {
		return 1500
	}
	return float64(e.LatencyMS)
}
