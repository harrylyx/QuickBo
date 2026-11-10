// Package config parses TVBox / FongMi subscription JSON.
//
// The wire format is the public subscription shape used across that
// ecosystem: a JSON object (sometimes base64 or gzip wrapped, sometimes
// with comments) describing sites, lives and parses. QuickBo ships with
// no subscriptions of its own.
package config

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// Site types as used by TVBox-style configs.
const (
	TypeXML    = 0 // MacCMS XML provide/vod
	TypeJSON   = 1 // MacCMS JSON provide/vod
	TypeSpider = 3 // jar, js, or a rule engine such as XBPQ
	TypeRemote = 4
)

// Level is how far QuickBo can drive a site.
type Level int

const (
	LevelYes Level = iota
	LevelPartial
	LevelNo
)

func (l Level) String() string {
	switch l {
	case LevelYes:
		return "可用"
	case LevelPartial:
		return "部分支持"
	default:
		return "不支持"
	}
}

// FlexInt reads a JSON number, a numeric string, or a bool. Subscriptions
// often write "2" where the field is specified as a number. A value that
// is not a number becomes 0 instead of failing the whole document.
type FlexInt int

func (n *FlexInt) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var s string
		if json.Unmarshal(data, &s) != nil {
			return nil
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		if i, err := strconv.Atoi(s); err == nil {
			*n = FlexInt(i)
			return nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			*n = FlexInt(int(f))
		}
		return nil
	}
	if data[0] == 't' || data[0] == 'f' {
		var b bool
		if json.Unmarshal(data, &b) == nil && b {
			*n = 1
		}
		return nil
	}
	var f float64
	if json.Unmarshal(data, &f) == nil {
		*n = FlexInt(int(f))
	}
	return nil
}

// Site is one on-demand source.
type Site struct {
	Key         string  `json:"key"`
	Name        string  `json:"name"`
	Type        FlexInt `json:"type"`
	API         string  `json:"api"`
	Searchable  FlexInt `json:"searchable"`
	QuickSearch FlexInt `json:"quickSearch"`
	PlayerType  FlexInt `json:"playerType"`
	Jar         string  `json:"jar"`
	Ext         any     `json:"ext"`
}

// ExtString returns ext when it is a string. Objects are JSON-encoded so
// a rule embedded as an object is not dropped.
func (s Site) ExtString() string {
	switch v := s.Ext.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// Live is a live-TV playlist entry. Channels are filled when the playlist
// is fetched, not when the subscription is parsed.
type Live struct {
	Name string  `json:"name"`
	Type FlexInt `json:"type"`
	URL  string  `json:"url"`
	EPG  string  `json:"epg"`
}

// LiveList skips malformed live entries instead of failing the whole file.
type LiveList []Live

func (l *LiveList) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" || data[0] != '[' {
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	out := make([]Live, 0, len(raw))
	for _, item := range raw {
		var live Live
		if json.Unmarshal(item, &live) != nil {
			continue
		}
		if strings.TrimSpace(live.URL) == "" && strings.TrimSpace(live.Name) == "" {
			continue
		}
		out = append(out, live)
	}
	*l = out
	return nil
}

// ParseItem is an online parse entry. Type 1 is a JSON parser; other
// types are recorded and not executed.
type ParseItem struct {
	Name string  `json:"name"`
	Type FlexInt `json:"type"`
	URL  string  `json:"url"`
	Ext  any     `json:"ext"`
}

// NamedURL is one entry in a subscription index (urls / storeHouse).
type NamedURL struct {
	Name string
	URL  string
}

func (n *NamedURL) UnmarshalJSON(data []byte) error {
	var s string
	if json.Unmarshal(data, &s) == nil {
		n.URL = strings.TrimSpace(s)
		n.Name = n.URL
		return nil
	}
	var o struct {
		URL        string `json:"url"`
		Name       string `json:"name"`
		SourceURL  string `json:"sourceUrl"`
		SourceName string `json:"sourceName"`
	}
	if err := json.Unmarshal(data, &o); err != nil {
		return err
	}
	n.URL = strings.TrimSpace(o.URL)
	if n.URL == "" {
		n.URL = strings.TrimSpace(o.SourceURL)
	}
	n.Name = strings.TrimSpace(o.Name)
	if n.Name == "" {
		n.Name = strings.TrimSpace(o.SourceName)
	}
	if n.Name == "" {
		n.Name = n.URL
	}
	return nil
}

// Config is a parsed subscription.
type Config struct {
	Spider     string      `json:"spider"`
	Wallpaper  string      `json:"wallpaper"`
	Sites      SiteList    `json:"sites"`
	Lives      LiveList    `json:"lives"`
	Parses     []ParseItem `json:"parses"`
	URLs       []NamedURL  `json:"urls"`
	StoreHouse []NamedURL  `json:"storeHouse"`
	// SourceURL is the address this document was loaded from. Relative
	// api/ext/live paths are resolved against it.
	SourceURL string `json:"sourceUrl,omitempty"`
}

// SiteList skips entries that are not site objects, so one broken item
// does not reject the subscription.
type SiteList []Site

func (l *SiteList) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" || data[0] != '[' {
		return nil
	}
	var raw []json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return nil
	}
	out := make([]Site, 0, len(raw))
	for _, item := range raw {
		var site Site
		if json.Unmarshal(item, &site) != nil {
			continue
		}
		if strings.TrimSpace(site.Key) == "" && strings.TrimSpace(site.Name) == "" && strings.TrimSpace(site.API) == "" {
			continue
		}
		out = append(out, site)
	}
	*l = out
	return nil
}

// IndexEntries returns warehouse links when the document is an index
// rather than a terminal subscription.
func (c *Config) IndexEntries() []NamedURL {
	if c == nil {
		return nil
	}
	out := make([]NamedURL, 0, len(c.URLs)+len(c.StoreHouse))
	for _, u := range c.URLs {
		if strings.TrimSpace(u.URL) != "" {
			out = append(out, u)
		}
	}
	for _, u := range c.StoreHouse {
		if strings.TrimSpace(u.URL) == "" {
			continue
		}
		if u.Name == "" {
			u.Name = u.URL
		}
		out = append(out, u)
	}
	return out
}
