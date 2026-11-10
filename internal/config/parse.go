package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// Parse turns subscription bytes into a Config. The pipeline is
// Decode, then Lenient, then JSON.
func Parse(raw []byte) (*Config, error) {
	if len(bytesTrim(raw)) == 0 {
		return nil, fmt.Errorf("配置内容为空")
	}
	decoded, err := Decode(raw)
	if err != nil {
		return nil, err
	}
	cleaned := Lenient(decoded)
	cfg, err := unmarshal(cleaned)
	if err != nil {
		return nil, fmt.Errorf("JSON 解析: %w", err)
	}
	return cfg, nil
}

func bytesTrim(b []byte) string {
	return strings.TrimSpace(string(b))
}

func unmarshal(cleaned []byte) (*Config, error) {
	cleaned = []byte(strings.TrimSpace(string(cleaned)))
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("配置内容为空")
	}
	if cleaned[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(cleaned, &arr); err != nil {
			return nil, err
		}
		merged := &Config{}
		for _, item := range arr {
			var one Config
			if json.Unmarshal(item, &one) != nil {
				continue
			}
			merge(merged, &one)
		}
		if len(merged.Sites) == 0 && len(merged.Lives) == 0 && len(merged.IndexEntries()) == 0 {
			return nil, fmt.Errorf("配置数组里没有站点、直播或仓库")
		}
		return merged, nil
	}
	var cfg Config
	if err := json.Unmarshal(cleaned, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func merge(dst, src *Config) {
	dst.Sites = append(dst.Sites, src.Sites...)
	dst.Lives = append(dst.Lives, src.Lives...)
	dst.Parses = append(dst.Parses, src.Parses...)
	dst.URLs = append(dst.URLs, src.URLs...)
	dst.StoreHouse = append(dst.StoreHouse, src.StoreHouse...)
	if dst.Spider == "" {
		dst.Spider = src.Spider
	}
}

// ResolveRelatives rewrites ./ and similar references against the
// document's own URL. csp_* class names and inline JSON/JS are left alone.
func (c *Config) ResolveRelatives() {
	if c == nil || strings.TrimSpace(c.SourceURL) == "" {
		return
	}
	for i := range c.Sites {
		if rel := resolveRef(c.SourceURL, c.Sites[i].API); rel != "" && shouldResolveAPI(c.Sites[i].API) {
			c.Sites[i].API = rel
		}
		ext := c.Sites[i].ExtString()
		if shouldResolveExt(ext) {
			if rel := resolveRef(c.SourceURL, ext); rel != "" {
				c.Sites[i].Ext = rel
			}
		}
	}
	for i := range c.Lives {
		if shouldResolveAPI(c.Lives[i].URL) {
			if rel := resolveRef(c.SourceURL, c.Lives[i].URL); rel != "" {
				c.Lives[i].URL = rel
			}
		}
	}
	for i := range c.Parses {
		if shouldResolveAPI(c.Parses[i].URL) {
			if rel := resolveRef(c.SourceURL, c.Parses[i].URL); rel != "" {
				c.Parses[i].URL = rel
			}
		}
	}
	for i := range c.URLs {
		if shouldResolveAPI(c.URLs[i].URL) {
			if rel := resolveRef(c.SourceURL, c.URLs[i].URL); rel != "" {
				c.URLs[i].URL = rel
			}
		}
	}
	for i := range c.StoreHouse {
		if shouldResolveAPI(c.StoreHouse[i].URL) {
			if rel := resolveRef(c.SourceURL, c.StoreHouse[i].URL); rel != "" {
				c.StoreHouse[i].URL = rel
			}
		}
	}
}

func shouldResolveAPI(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "://") || strings.HasPrefix(strings.ToLower(s), "csp_") {
		return false
	}
	if strings.ContainsAny(s, " \t\n{}") {
		return false
	}
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") || strings.HasPrefix(s, "/") {
		return true
	}
	low := strings.ToLower(stripQuery(s))
	return strings.Contains(s, "/") || strings.HasSuffix(low, ".js") || strings.HasSuffix(low, ".json") ||
		strings.HasSuffix(low, ".txt") || strings.HasSuffix(low, ".m3u") || strings.HasSuffix(low, ".m3u8")
}

func shouldResolveExt(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" || strings.Contains(t, "://") {
		return false
	}
	if strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") || strings.HasPrefix(t, "var ") ||
		strings.HasPrefix(t, "function") || strings.HasPrefix(t, "export ") {
		return false
	}
	low := strings.ToLower(stripQuery(t))
	return strings.HasPrefix(t, "./") || strings.HasPrefix(t, "../") || strings.HasPrefix(t, "/") ||
		strings.HasSuffix(low, ".js") || strings.HasSuffix(low, ".json") || strings.HasSuffix(low, ".txt")
}

func resolveRef(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	bu, err := url.Parse(base)
	if err != nil {
		return ref
	}
	ru, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return bu.ResolveReference(ru).String()
}

// FileURL returns a file: URL for a local path.
func FileURL(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.ToSlash(abs)
	if !strings.HasPrefix(abs, "/") {
		abs = "/" + abs
	}
	return (&url.URL{Scheme: "file", Path: abs}).String()
}
