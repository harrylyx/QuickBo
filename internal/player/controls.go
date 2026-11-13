package player

import (
	"strconv"
	"strings"
)

// Controls is a Player whose mpv properties and commands the app can reach:
// tracks, subtitles, picture and screenshots. The embedded libmpv player has
// it; an mpv process in its own window does not.
type Controls interface {
	SetProp(name, value string) error
	Prop(name string) (string, error)
	Command(args ...string) error
}

// Track is one audio, video or subtitle track of the stream.
type Track struct {
	ID       int
	Kind     string // "audio", "video" or "sub"
	Title    string
	Lang     string
	Codec    string
	Selected bool
	External bool
}

// Label names the track for a menu: its title, language and codec, or its
// number when it says nothing about itself.
func (t Track) Label() string {
	var parts []string
	if s := strings.TrimSpace(t.Title); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(t.Lang); s != "" {
		parts = append(parts, langName(s))
	}
	if len(parts) == 0 {
		parts = append(parts, "轨道 "+strconv.Itoa(t.ID))
	}
	if t.External {
		parts = append(parts, "外挂")
	}
	return strings.Join(parts, " · ")
}

func langName(code string) string {
	switch strings.ToLower(code) {
	case "chi", "zho", "zh", "chs", "sc", "zh-cn", "zh-hans":
		return "中文"
	case "cht", "tc", "zh-tw", "zh-hk", "zh-hant":
		return "繁体中文"
	case "eng", "en":
		return "英语"
	case "jpn", "ja":
		return "日语"
	case "kor", "ko":
		return "韩语"
	}
	return code
}

// Tracks lists the stream's tracks, read one property at a time from mpv's
// track-list so no JSON parsing depends on mpv's string form of nodes.
func Tracks(c Controls) []Track {
	n, err := c.Prop("track-list/count")
	if err != nil {
		return nil
	}
	count, _ := strconv.Atoi(strings.TrimSpace(n))
	var out []Track
	for i := 0; i < count && i < 64; i++ {
		get := func(field string) string {
			v, _ := c.Prop("track-list/" + strconv.Itoa(i) + "/" + field)
			return strings.TrimSpace(v)
		}
		id, err := strconv.Atoi(get("id"))
		if err != nil {
			continue
		}
		out = append(out, Track{
			ID: id, Kind: get("type"), Title: get("title"), Lang: get("lang"),
			Codec: get("codec"), Selected: get("selected") == "yes", External: get("external") == "yes",
		})
	}
	return out
}
