// Package xbpq interprets csp_XBPQ rules: a JSON document of Chinese
// keys whose values are "left&&right" text cuts, not a jar.
package xbpq

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/harrylyx/mybox/internal/httpx"
	"github.com/harrylyx/mybox/internal/model"
)

// Rule is the subset of an XBPQ rule mybox understands.
type Rule struct {
	HomeURL        string            `json:"主页url"`
	Classes        string            `json:"分类"`
	CategoryURL    string            `json:"分类url"`
	Array          string            `json:"数组"`
	Title          string            `json:"标题"`
	Link           string            `json:"链接"`
	Pic            string            `json:"图片"`
	Subtitle       string            `json:"副标题"`
	LinkPrefix     string            `json:"链接前缀"`
	LinkSuffix     string            `json:"链接后缀"`
	SearchURL      string            `json:"搜索url"`
	SearchMode     string            `json:"搜索模式"`
	SearchArray    string            `json:"搜索数组"`
	SearchTitle    string            `json:"搜索标题"`
	SearchLink     string            `json:"搜索链接"`
	SearchPic      string            `json:"搜索图片"`
	SearchSubtitle string            `json:"搜索副标题"`
	Year           string            `json:"影片年代"`
	Year2          string            `json:"年代"`
	Area           string            `json:"影片地区"`
	Area2          string            `json:"地区"`
	Remarks        string            `json:"影片状态"`
	Remarks2       string            `json:"状态"`
	Content        string            `json:"简介"`
	PlayFromArray  string            `json:"线路数组"`
	PlayFromTitle  string            `json:"线路标题"`
	PlayArray      string            `json:"播放数组"`
	PlayList       string            `json:"播放列表"`
	PlayTitle      string            `json:"播放标题"`
	PlayLink       string            `json:"播放链接"`
	DirectPlay     string            `json:"直接播放"`
	Headers        map[string]string `json:"-"`
	HeaderRaw      json.RawMessage   `json:"请求头"`
}

// Parse reads a rule. Comments and a few illegal escapes are tolerated.
func Parse(data []byte) (*Rule, error) {
	clean := stripComments(data)
	var r Rule
	if err := json.Unmarshal(clean, &r); err != nil {
		return nil, fmt.Errorf("解析 XBPQ 规则失败: %w", err)
	}
	r.Headers = parseHeaders(r.HeaderRaw)
	if strings.TrimSpace(r.HomeURL) == "" && strings.TrimSpace(r.CategoryURL) == "" {
		return nil, fmt.Errorf("XBPQ 规则缺少 主页url 或 分类url")
	}
	if strings.TrimSpace(r.HomeURL) == "" {
		r.HomeURL = originOf(r.CategoryURL)
	}
	return &r, nil
}

// Engine runs a rule.
type Engine struct {
	Rule *Rule
}

func New(r *Rule) *Engine { return &Engine{Rule: r} }

func (e *Engine) Categories() []model.Category {
	var out []model.Category
	for _, part := range strings.Split(e.Rule.Classes, "#") {
		name, id, ok := strings.Cut(strings.TrimSpace(part), "$")
		if !ok {
			id = name
		}
		name, id = strings.TrimSpace(name), strings.TrimSpace(id)
		if name == "" || id == "" {
			continue
		}
		out = append(out, model.Category{ID: id, Name: name})
	}
	return out
}

func (e *Engine) List(ctx context.Context, cat string, page int) (model.Page, error) {
	if page <= 0 {
		page = 1
	}
	raw, err := e.get(ctx, expand(e.Rule.CategoryURL, cat, page, ""))
	if err != nil {
		return model.Page{}, err
	}
	items := e.items(string(raw), e.Rule.Array, e.Rule.Title, e.Rule.Link, e.Rule.Pic, e.Rule.Subtitle)
	return model.Page{Items: items, Page: page, PageCount: 0}, nil
}

func (e *Engine) Search(ctx context.Context, q string, page int) (model.Page, error) {
	if strings.TrimSpace(e.Rule.SearchURL) == "" {
		return model.Page{}, fmt.Errorf("该规则没有搜索地址")
	}
	if page <= 0 {
		page = 1
	}
	u := expand(e.Rule.SearchURL, "", page, q)
	var body []byte
	var err error
	if strings.TrimSpace(e.Rule.SearchMode) == "1" && !strings.Contains(u, url.QueryEscape(q)) && !strings.Contains(e.Rule.SearchURL, "{wd}") && !strings.Contains(e.Rule.SearchURL, "**") {
		body, err = httpx.PostForm(ctx, nil, e.abs(u), "wd="+url.QueryEscape(q), e.header(), 4<<20)
	} else {
		body, err = e.get(ctx, u)
	}
	if err != nil {
		return model.Page{}, err
	}
	items := e.items(string(body),
		first(e.Rule.SearchArray, e.Rule.Array),
		first(e.Rule.SearchTitle, e.Rule.Title),
		first(e.Rule.SearchLink, e.Rule.Link),
		first(e.Rule.SearchPic, e.Rule.Pic),
		first(e.Rule.SearchSubtitle, e.Rule.Subtitle))
	return model.Page{Items: items, Page: page}, nil
}

func (e *Engine) Detail(ctx context.Context, id string) (model.Detail, error) {
	rawb, err := e.get(ctx, id)
	if err != nil {
		return model.Detail{}, err
	}
	raw := string(rawb)
	title := first(h1Text(raw), clean(cut(raw, e.Rule.Title)), titleTag(raw))
	pic := clean(cut(raw, e.Rule.Pic))
	if pic == "" {
		pic = ogImage(raw)
	}
	d := model.Detail{
		ID: id, Title: title, Pic: e.abs(pic),
		Year:    clean(cut(raw, first(e.Rule.Year, e.Rule.Year2))),
		Area:    clean(cut(raw, first(e.Rule.Area, e.Rule.Area2))),
		Remarks: clean(cut(raw, first(e.Rule.Remarks, e.Rule.Remarks2))),
		Content: clean(cut(raw, e.Rule.Content)),
	}
	var lines []model.Line
	froms := []string{}
	for _, seg := range cutList(raw, e.Rule.PlayFromArray) {
		if name := clean(cut(seg, e.Rule.PlayFromTitle)); name != "" {
			froms = append(froms, name)
		}
	}
	playList := first(e.Rule.PlayList, `<a&&</a>`)
	playTitle := first(e.Rule.PlayTitle, `>&&<`)
	playLink := first(e.Rule.PlayLink, `href="&&"`)
	i := 0
	for _, seg := range cutList(raw, e.Rule.PlayArray) {
		var eps []model.Episode
		for _, entry := range cutList(seg, playList) {
			name := clean(cut(entry, playTitle))
			href := clean(cut(entry, playLink))
			if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") || strings.HasPrefix(href, "#") {
				continue
			}
			if name == "" {
				name = fmt.Sprintf("第%d集", len(eps)+1)
			}
			eps = append(eps, model.Episode{Name: name, URL: e.abs(e.Rule.LinkPrefix + href + e.Rule.LinkSuffix)})
		}
		if len(eps) == 0 {
			continue
		}
		name := fmt.Sprintf("线路%d", i+1)
		if i < len(froms) {
			name = froms[i]
		}
		lines = append(lines, model.Line{Name: name, Episodes: eps})
		i++
	}
	if len(lines) == 0 && truthy(e.Rule.DirectPlay) {
		lines = []model.Line{{Name: "播放", Episodes: []model.Episode{{Name: "播放", URL: id}}}}
	}
	d.Lines = lines
	return d, nil
}

// Resolve returns a direct media URL when the episode already is one,
// otherwise it looks for a media address in the play page.
func (e *Engine) Resolve(ctx context.Context, ep model.Episode) (model.Playable, error) {
	u := ep.URL
	headers := e.header()
	if headers == nil {
		headers = map[string]string{}
	}
	if headers["Referer"] == "" {
		headers["Referer"] = originOf(e.Rule.HomeURL) + "/"
	}
	if headers["User-Agent"] == "" {
		headers["User-Agent"] = httpx.BrowserUA
	}
	if !looksMedia(u) {
		if page, err := e.get(ctx, u); err == nil {
			if found := findMedia(string(page)); found != "" {
				u = found
			}
		}
	}
	return model.Playable{URL: u, Headers: headers, NeedsParse: !looksMedia(u)}, nil
}

func (e *Engine) items(raw, array, title, link, pic, sub string) []model.Item {
	var out []model.Item
	for _, seg := range cutList(raw, array) {
		name := clean(cut(seg, title))
		href := clean(cut(seg, link))
		if name == "" || href == "" {
			continue
		}
		href = e.abs(e.Rule.LinkPrefix + href + e.Rule.LinkSuffix)
		out = append(out, model.Item{
			ID: href, Title: name, Pic: e.abs(clean(cut(seg, pic))), Remarks: clean(cut(seg, sub)),
		})
	}
	return out
}

func (e *Engine) get(ctx context.Context, raw string) ([]byte, error) {
	return bodyOnly(httpx.Get(ctx, nil, e.abs(raw), e.header(), 4<<20))
}

func bodyOnly(b []byte, _ string, err error) ([]byte, error) { return b, err }

func (e *Engine) header() map[string]string {
	if len(e.Rule.Headers) == 0 {
		return map[string]string{"User-Agent": httpx.BrowserUA}
	}
	out := map[string]string{}
	for k, v := range e.Rule.Headers {
		out[k] = resolveUA(v)
	}
	if out["User-Agent"] == "" {
		out["User-Agent"] = httpx.BrowserUA
	}
	return out
}

func (e *Engine) abs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	base, err := url.Parse(e.Rule.HomeURL)
	if err != nil {
		return raw
	}
	if !strings.HasSuffix(base.Path, "/") && !strings.Contains(strings.TrimPrefix(base.Path, "/"), "/") && raw[0] != '/' && !strings.Contains(e.Rule.HomeURL, "?") {
		base.Path = "/"
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return base.ResolveReference(ref).String()
}

func expand(tpl, cat string, page int, word string) string {
	pg := strconv.Itoa(page)
	r := strings.NewReplacer(
		"{cateId}", cat, "{catePg}", pg, "{pg}", pg,
		"fyclass", cat, "fypage", pg,
		"{wd}", url.QueryEscape(word), "**", url.QueryEscape(word),
		"{area}", "", "{by}", "", "{class}", "", "{lang}", "", "{year}", "", "{letter}", "",
	)
	return r.Replace(tpl)
}

// cut returns the text between the two sides of "left&&right".
// A spec without "&&" returns the text after the marker.
func cut(s, spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" || s == "" {
		return ""
	}
	left, right, ok := strings.Cut(spec, "&&")
	i := strings.Index(s, left)
	if i < 0 {
		return ""
	}
	rest := s[i+len(left):]
	if !ok || right == "" {
		return strings.TrimSpace(rest)
	}
	j := strings.Index(rest, right)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:j])
}

// cutList splits a page into chunks using "start&&end".
func cutList(s, spec string) []string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	left, right, ok := strings.Cut(spec, "&&")
	if !ok {
		right = ""
	}
	parts := strings.Split(s, left)
	if len(parts) <= 1 {
		return nil
	}
	out := make([]string, 0, len(parts)-1)
	for _, p := range parts[1:] {
		if right != "" {
			if j := strings.Index(p, right); j >= 0 {
				p = p[:j]
			}
		}
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

func clean(s string) string {
	return strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(s, "")))
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truthy(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "1" || s == "true" || s == "yes"
}

func originOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Scheme + "://" + u.Host
}

func parseHeaders(raw json.RawMessage) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	trim := strings.TrimSpace(string(raw))
	if trim == "" || trim == "null" {
		return nil
	}
	if strings.HasPrefix(trim, "{") {
		var m map[string]string
		if json.Unmarshal(raw, &m) == nil {
			return m
		}
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return nil
	}
	out := map[string]string{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == '#' || r == '\n' }) {
		k, v, ok := strings.Cut(part, "$")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(resolveUA(v))
		if k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

func resolveUA(v string) string {
	switch strings.TrimSpace(v) {
	case "MOBILE_UA":
		return httpx.BrowserUA
	case "PC_UA":
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	default:
		return v
	}
}

func stripComments(data []byte) []byte {
	var out []byte
	in := false
	esc := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if in {
			out = append(out, c)
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				in = false
			}
			continue
		}
		if c == '"' {
			in = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(data) && data[i+1] == '/' {
			for i < len(data) && data[i] != '\n' {
				i++
			}
			continue
		}
		out = append(out, c)
	}
	return out
}

var (
	h1Re    = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	ogRe    = regexp.MustCompile(`(?is)<meta[^>]*(?:property|name)\s*=\s*["']og:image["'][^>]*>`)
	cntRe   = regexp.MustCompile(`(?is)content\s*=\s*["']([^"']+)["']`)
	mediaRe = regexp.MustCompile(`(?i)https?://[^"'\\\s<>]+(?:\.m3u8|\.mp4|\.flv|\.m4v)(?:\?[^"'\\\s<>]*)?`)
)

func h1Text(s string) string {
	m := h1Re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return clean(m[1])
}

func titleTag(s string) string {
	m := titleRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return clean(m[1])
}

func ogImage(s string) string {
	m := ogRe.FindString(s)
	if m == "" {
		return ""
	}
	c := cntRe.FindStringSubmatch(m)
	if len(c) < 2 {
		return ""
	}
	return c[1]
}

func findMedia(page string) string { return mediaRe.FindString(page) }

func looksMedia(raw string) bool {
	s := strings.ToLower(strings.TrimSpace(raw))
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	for _, ext := range []string{".m3u8", ".mp4", ".m4v", ".flv", ".mkv", ".webm", ".mov", ".ts"} {
		if strings.HasSuffix(s, ext) {
			return true
		}
	}
	return false
}
