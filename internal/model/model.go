// Package model holds the media records the source layer and the UI share.
package model

import (
	"fmt"
	"html"
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
