// Package model holds the media records the source layer and the UI share.
package model

import (
	"fmt"
	"html"
	"net/url"
	"strings"
)

// Category is a browse group on a source.
type Category struct {
	ID   string
	Name string
}

// Item is one title in a list or search result.
type Item struct {
	ID      string
	Title   string
	Pic     string
	Remarks string
	Year    string
}

// Page is one page of items.
type Page struct {
	Items     []Item
	Page      int
	PageCount int
	Total     int
}

// HasNext reports whether another page is worth requesting.
func (p Page) HasNext() bool {
	if len(p.Items) == 0 {
		return false
	}
	if p.PageCount > 0 {
		return p.Page < p.PageCount
	}
	// A source may report a total without a page count: once every item has
	// been collected there is nothing left to fetch.
	if p.Total > 0 && len(p.Items) >= p.Total {
		return false
	}
	return true
}

// Episode is one playable entry.
type Episode struct {
	Name string
	URL  string
}

// Line is a named group of episodes (a "play from").
type Line struct {
	Name     string
	Episodes []Episode
}

// Detail is a title page.
type Detail struct {
	ID       string
	Title    string
	Pic      string
	Remarks  string
	Year     string
	Area     string
	TypeName string
	Director string
	Actors   string
	Content  string
	Lines    []Line
}

// Playable is a resolved stream.
type Playable struct {
	URL        string
	Headers    map[string]string
	NeedsParse bool
}

// ResolveURL turns a CMS media address into an absolute URL. Protocol-relative
// pictures (//cdn/a.jpg) become https, and paths are resolved against base.
func ResolveURL(base, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if strings.Contains(raw, "://") || strings.TrimSpace(base) == "" {
		return raw
	}
	bu, err := url.Parse(base)
	if err != nil {
		return raw
	}
	ru, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return bu.ResolveReference(ru).String()
}

// PreferredLine picks the first play line that already looks like a media
// file. CMS feeds often put a share page on line 1 and the m3u8 on line 2.
func PreferredLine(lines []Line) int {
	for i, ln := range lines {
		for _, ep := range ln.Episodes {
			if mediaSuffix(ep.URL) {
				return i
			}
		}
	}
	return 0
}

func mediaSuffix(raw string) bool {
	low := strings.ToLower(strings.TrimSpace(raw))
	if i := strings.IndexAny(low, "?#"); i >= 0 {
		low = low[:i]
	}
	for _, ext := range []string{".m3u8", ".m3u", ".mp4", ".m4v", ".mkv", ".flv", ".ts", ".mpd"} {
		if strings.HasSuffix(low, ext) {
			return true
		}
	}
	return false
}

// ParsePlay splits the CMS vod_play_from / vod_play_url pair.
// Lines are separated by $$$, episodes by #, and the first $ splits
// the episode name from its URL.
func ParsePlay(from, play string) []Line {
	play = strings.TrimSpace(play)
	if play == "" {
		return nil
	}
	froms := splitFrom(from)
	segs := strings.Split(play, "$$$")
	var lines []Line
	for i, seg := range segs {
		name := fmt.Sprintf("线路%d", i+1)
		if i < len(froms) && strings.TrimSpace(froms[i]) != "" {
			name = strings.TrimSpace(froms[i])
		}
		var eps []Episode
		for _, raw := range strings.Split(seg, "#") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			n, u, ok := strings.Cut(raw, "$")
			if !ok {
				u = raw
				n = fmt.Sprintf("第%d集", len(eps)+1)
			}
			n = strings.TrimSpace(html.UnescapeString(n))
			u = strings.TrimSpace(html.UnescapeString(u))
			if u == "" {
				continue
			}
			if n == "" {
				n = fmt.Sprintf("第%d集", len(eps)+1)
			}
			eps = append(eps, Episode{Name: n, URL: u})
		}
		if len(eps) > 0 {
			lines = append(lines, Line{Name: name, Episodes: eps})
		}
	}
	return lines
}

func splitFrom(from string) []string {
	from = strings.TrimSpace(from)
	if from == "" {
		return nil
	}
	if strings.Contains(from, "$$$") {
		return splitKeep(from, "$$$")
	}
	if strings.Contains(from, ",") && !strings.Contains(from, "://") {
		return splitKeep(from, ",")
	}
	return []string{from}
}

func splitKeep(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// StripTags removes a single level of HTML tags and unescapes entities.
func StripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(html.UnescapeString(b.String()))
}
