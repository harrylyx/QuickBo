// Package live parses M3U and the "name,url" TXT playlists TVBox uses,
// including "#genre#" group markers.
package live

import (
	"bufio"
	"bytes"
	"strings"
)

// Channel is one live stream.
type Channel struct {
	Name  string
	URL   string
	Logo  string
	Group string
}

// Parse picks M3U or TXT from the body.
func Parse(raw []byte) []Channel {
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if bytes.Contains(bytes.ToUpper(raw), []byte("#EXTM3U")) || bytes.Contains(raw, []byte("#EXTINF")) {
		return ParseM3U(raw)
	}
	return ParseTXT(raw)
}

// ParseM3U reads an extended M3U playlist.
func ParseM3U(raw []byte) []Channel {
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 2<<20)
	var out []Channel
	var cur *Channel
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#EXTM3U") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "#EXTINF") {
				e := parseExtinf(line)
				cur = &e
			}
			continue
		}
		if cur == nil {
			continue
		}
		cur.URL = line
		out = append(out, *cur)
		cur = nil
	}
	return out
}

func parseExtinf(line string) Channel {
	var e Channel
	rest := strings.TrimPrefix(line, "#EXTINF")
	if i := strings.Index(rest, ","); i >= 0 {
		e.Name = strings.TrimSpace(rest[i+1:])
		rest = rest[:i]
	}
	e.Logo = attr(rest, "tvg-logo")
	e.Group = attr(rest, "group-title")
	if id := attr(rest, "tvg-id"); e.Name == "" {
		e.Name = id
	}
	return e
}

func attr(s, key string) string {
	i := strings.Index(s, key+"=")
	if i < 0 {
		return ""
	}
	s = strings.TrimLeft(s[i+len(key)+1:], " \t")
	if strings.HasPrefix(s, `"`) {
		s = s[1:]
		if j := strings.IndexByte(s, '"'); j >= 0 {
			return s[:j]
		}
	}
	if j := strings.IndexAny(s, " \t"); j >= 0 {
		return s[:j]
	}
	return s
}

// ParseTXT reads "name,url" lines and "#genre#" group headers.
func ParseTXT(raw []byte) []Channel {
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 2<<20)
	var out []Channel
	group := ""
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") && !strings.Contains(strings.ToLower(line), "genre") {
			continue
		}
		if strings.Contains(strings.ToLower(line), "#genre#") {
			name := strings.TrimSpace(strings.Split(line, ",")[0])
			name = strings.TrimSuffix(name, "#genre#")
			name = strings.TrimSuffix(name, "#Genre#")
			group = strings.TrimSpace(name)
			continue
		}
		name, u, ok := strings.Cut(line, ",")
		name, u = strings.TrimSpace(name), strings.TrimSpace(u)
		if !ok || name == "" || !hasScheme(u) {
			continue
		}
		out = append(out, Channel{Name: name, URL: u, Group: group})
	}
	return out
}

func hasScheme(raw string) bool {
	i := strings.Index(raw, "://")
	if i <= 0 {
		return false
	}
	for j, r := range raw[:i] {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.') && j > 0:
		default:
			return false
		}
	}
	return true
}
