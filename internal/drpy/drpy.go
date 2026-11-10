// Package drpy runs a practical subset of drpy-style scripts:
// a `rule` object with CSS / "selector&&attr" fields, and scripts that
// define home, category, detail, search and play. It is not a full
// drpy2 template engine.
package drpy

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/harrylyx/mybox/internal/httpx"
	"github.com/harrylyx/mybox/internal/model"
)

// Engine is one compiled script.
type Engine struct {
	mu   sync.Mutex
	vm   *goja.Runtime
	rule map[string]any
	ctx  context.Context
}

// Compile prepares a script. Inline JSON objects are accepted as rules.
func Compile(code string) (*Engine, error) {
	code = strings.TrimSpace(strings.TrimPrefix(code, "\uFEFF"))
	if code == "" {
		return nil, fmt.Errorf("脚本为空")
	}
	if strings.HasPrefix(code, "{") {
		var m map[string]any
		if err := json.Unmarshal([]byte(code), &m); err != nil {
			return nil, fmt.Errorf("规则 JSON 无效: %w", err)
		}
		return &Engine{rule: m, ctx: context.Background()}, nil
	}
	code = prepare(code)
	vm := goja.New()
	e := &Engine{vm: vm, ctx: context.Background()}
	vm.Set("MOBILE_UA", httpx.BrowserUA)
	vm.Set("PC_UA", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	vm.Set("log", func(goja.FunctionCall) goja.Value { return goja.Undefined() })
	vm.Set("print", func(goja.FunctionCall) goja.Value { return goja.Undefined() })
	vm.Set("req", e.jsRequest)
	vm.Set("request", e.jsRequest)
	if _, err := vm.RunString(code); err != nil {
		return nil, fmt.Errorf("执行脚本失败: %w", err)
	}
	if v := vm.Get("rule"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
		if m, ok := v.Export().(map[string]any); ok {
			e.rule = m
		}
	}
	if e.rule == nil && !e.has("home") && !e.has("category") {
		return nil, fmt.Errorf("脚本既没有 rule 对象，也没有 home/category 函数")
	}
	if e.has("init") {
		ext := ""
		if e.rule != nil {
			if b, err := json.Marshal(e.rule); err == nil {
				ext = string(b)
			}
		}
		_, _ = e.call("init", ext)
	}
	return e, nil
}

func prepare(code string) string {
	s := strings.TrimSpace(code)
	if strings.HasPrefix(s, "export default") {
		s = "var rule = " + strings.TrimSpace(strings.TrimPrefix(s, "export default"))
	}
	return s
}

func (e *Engine) has(name string) bool {
	if e == nil || e.vm == nil {
		return false
	}
	v := e.vm.Get(name)
	_, ok := goja.AssertFunction(v)
	return ok
}

// Categories lists classes from the rule or from home().
func (e *Engine) Categories(ctx context.Context) ([]model.Category, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ctx = ctx
	if e.rule != nil && !e.has("category") {
		return e.ruleCategories(), nil
	}
	if e.has("home") {
		v, err := e.call("home", true)
		if err != nil {
			v, err = e.call("home")
		}
		if err != nil {
			return nil, err
		}
		if cats := classesFrom(v); len(cats) > 0 {
			return cats, nil
		}
	}
	if e.rule != nil {
		return e.ruleCategories(), nil
	}
	return []model.Category{{ID: "", Name: "推荐"}}, nil
}

func (e *Engine) ruleCategories() []model.Category {
	var out []model.Category
	if e.str("推荐") != "" {
		out = append(out, model.Category{ID: "__rec__", Name: "推荐"})
	}
	names := strings.Split(e.str("class_name"), "&")
	ids := strings.Split(e.str("class_url"), "&")
	for i, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		id := name
		if i < len(ids) && strings.TrimSpace(ids[i]) != "" {
			id = strings.TrimSpace(ids[i])
		}
		out = append(out, model.Category{ID: id, Name: name})
	}
	if len(out) == 0 {
		out = append(out, model.Category{ID: "", Name: "全部"})
	}
	return out
}

// List loads one category page.
func (e *Engine) List(ctx context.Context, cat string, page int) (model.Page, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ctx = ctx
	if page <= 0 {
		page = 1
	}
	if e.has("category") && (e.rule == nil || e.str("url") == "") {
		v, err := e.call("category", cat, page, e.vm.NewObject(), "")
		if err != nil {
			return model.Page{}, err
		}
		return model.Page{Items: itemsFrom(v), Page: page}, nil
	}
	return e.ruleList(ctx, cat, page)
}

// Search runs the rule search or search().
func (e *Engine) Search(ctx context.Context, q string, page int) (model.Page, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ctx = ctx
	if page <= 0 {
		page = 1
	}
	if e.has("search") && e.str("searchUrl", "searchurl") == "" {
		v, err := e.call("search", q, false, page)
		if err != nil {
			return model.Page{}, err
		}
		return model.Page{Items: itemsFrom(v), Page: page}, nil
	}
	tpl := e.str("searchUrl", "searchurl")
	if tpl == "" {
		if e.has("search") {
			v, err := e.call("search", q, false, page)
			if err != nil {
				return model.Page{}, err
			}
			return model.Page{Items: itemsFrom(v), Page: page}, nil
		}
		return model.Page{}, fmt.Errorf("该脚本没有搜索")
	}
	u := expand(tpl, "", page, q)
	body, err := e.fetch(ctx, e.abs(u))
	if err != nil {
		return model.Page{}, err
	}
	spec := e.str("搜索", "一级")
	return model.Page{Items: parseList(string(body), spec, e.host()), Page: page}, nil
}

// Detail loads a title.
func (e *Engine) Detail(ctx context.Context, id string) (model.Detail, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ctx = ctx
	if e.has("detail") && e.rule == nil {
		v, err := e.call("detail", id)
		if err != nil {
			return model.Detail{}, err
		}
		return detailFromResult(id, v), nil
	}
	if e.rule != nil && !e.has("detail") {
		return e.ruleDetail(ctx, id)
	}
	if e.has("detail") {
		v, err := e.call("detail", id)
		if err == nil {
			d := detailFromResult(id, v)
			if d.Title != "" || len(d.Lines) > 0 {
				return d, nil
			}
		}
	}
	if e.rule != nil {
		return e.ruleDetail(ctx, id)
	}
	return model.Detail{}, fmt.Errorf("无法解析详情")
}

// Resolve turns an episode into a stream. A play() function wins.
func (e *Engine) Resolve(ctx context.Context, line string, ep model.Episode) (model.Playable, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ctx = ctx
	if e.has("play") {
		v, err := e.call("play", line, ep.URL, []any{})
		if err != nil {
			return model.Playable{}, err
		}
		return playableFrom(v, ep.URL), nil
	}
	u := ep.URL
	needs := !looksMedia(u)
	if needs {
		if body, err := e.fetch(ctx, u); err == nil {
			if found := findMedia(string(body)); found != "" {
				u = found
				needs = false
			}
		}
	}
	return model.Playable{
		URL: u, NeedsParse: needs,
		Headers: map[string]string{"User-Agent": httpx.BrowserUA, "Referer": e.host() + "/"},
	}, nil
}

func (e *Engine) ruleList(ctx context.Context, cat string, page int) (model.Page, error) {
	var u, spec string
	if cat == "__rec__" || (cat == "" && e.str("url") == "") {
		u = e.host()
		spec = e.str("推荐", "一级")
	} else {
		u = e.abs(expand(e.str("url"), cat, page, ""))
		spec = e.str("一级", "推荐")
	}
	if strings.TrimSpace(u) == "" {
		return model.Page{}, fmt.Errorf("规则没有分类地址")
	}
	body, err := e.fetch(ctx, u)
	if err != nil {
		return model.Page{}, err
	}
	return model.Page{Items: parseList(string(body), spec, e.host()), Page: page}, nil
}

func (e *Engine) ruleDetail(ctx context.Context, id string) (model.Detail, error) {
	body, err := e.fetch(ctx, e.abs(id))
	if err != nil {
		return model.Detail{}, err
	}
	d := parseDetail(string(body), e.rule["二级"], e.host())
	if d.ID == "" {
		d.ID = id
	}
	if d.Title == "" {
		d.Title = id
	}
	return d, nil
}

func (e *Engine) host() string {
	h := strings.TrimSpace(e.str("host"))
	if h == "" {
		return ""
	}
	return strings.TrimRight(h, "/")
}

func (e *Engine) str(keys ...string) string {
	if e.rule == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := e.rule[k]; ok {
			if s := asString(v); s != "" {
				return s
			}
		}
	}
	return ""
}

func (e *Engine) fetch(ctx context.Context, raw string) ([]byte, error) {
	headers := map[string]string{"User-Agent": httpx.BrowserUA}
	if h := e.host(); h != "" {
		headers["Referer"] = h + "/"
	}
	b, _, err := httpx.Get(ctx, nil, raw, headers, 4<<20)
	return b, err
}

func (e *Engine) call(name string, args ...any) (any, error) {
	fn, ok := goja.AssertFunction(e.vm.Get(name))
	if !ok {
		return nil, fmt.Errorf("没有函数 %s", name)
	}
	vals := make([]goja.Value, len(args))
	for i, a := range args {
		vals[i] = e.vm.ToValue(a)
	}
	v, err := fn(goja.Undefined(), vals...)
	if err != nil {
		return nil, err
	}
	return exportValue(v), nil
}

func (e *Engine) jsRequest(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		return e.vm.ToValue("")
	}
	raw := call.Arguments[0].String()
	method := "GET"
	var body string
	headers := map[string]string{"User-Agent": httpx.BrowserUA}
	if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
		if opt, ok := call.Arguments[1].Export().(map[string]any); ok {
			if m := asString(opt["method"]); m != "" {
				method = strings.ToUpper(m)
			}
			if b := asString(firstVal(opt["body"], opt["data"])); b != "" {
				body = b
			}
			if hm, ok := opt["headers"].(map[string]any); ok {
				for k, v := range hm {
					headers[k] = asString(v)
				}
			}
		}
	}
	ctx := e.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	var (
		b   []byte
		err error
	)
	if method == "POST" {
		b, err = httpx.PostForm(ctx, nil, raw, body, headers, 4<<20)
	} else {
		b, _, err = httpx.Get(ctx, nil, raw, headers, 4<<20)
	}
	if err != nil {
		panic(e.vm.ToValue(err.Error()))
	}
	return e.vm.ToValue(string(b))
}

func expand(tpl, cat string, page int, word string) string {
	pg := strconv.Itoa(page)
	return strings.NewReplacer(
		"fyclass", cat,
		"fypage", pg,
		"{cateId}", cat,
		"{catePg}", pg,
		"{pg}", pg,
		"**", url.QueryEscape(word),
		"{wd}", url.QueryEscape(word),
	).Replace(tpl)
}

func (e *Engine) abs(ref string) string {
	return absURL(e.host(), ref)
}

func absURL(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	if strings.HasPrefix(ref, "//") {
		return "https:" + ref
	}
	bu, err := url.Parse(base)
	if err != nil || bu.Scheme == "" {
		return ref
	}
	if !strings.HasSuffix(bu.Path, "/") {
		bu.Path += "/"
	}
	ru, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return bu.ResolveReference(ru).String()
}

func exportValue(v goja.Value) any {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	exp := v.Export()
	if s, ok := exp.(string); ok {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
			var anyv any
			if json.Unmarshal([]byte(s), &anyv) == nil {
				return anyv
			}
		}
		return s
	}
	return exp
}

func classesFrom(v any) []model.Category {
	m, _ := v.(map[string]any)
	if m == nil {
		return nil
	}
	raw, _ := m["class"].([]any)
	var out []model.Category
	for _, item := range raw {
		im, _ := item.(map[string]any)
		if im == nil {
			continue
		}
		name := asString(firstVal(im["type_name"], im["name"]))
		id := asString(firstVal(im["type_id"], im["id"], im["tid"]))
		if name == "" {
			continue
		}
		if id == "" {
			id = name
		}
		out = append(out, model.Category{ID: id, Name: name})
	}
	return out
}

func itemsFrom(v any) []model.Item {
	list := digList(v)
	var out []model.Item
	for _, item := range list {
		im, _ := item.(map[string]any)
		if im == nil {
			continue
		}
		title := asString(firstVal(im["vod_name"], im["name"], im["title"]))
		if title == "" {
			continue
		}
		out = append(out, model.Item{
			ID:      asString(firstVal(im["vod_id"], im["id"])),
			Title:   title,
			Pic:     asString(firstVal(im["vod_pic"], im["pic"])),
			Remarks: asString(firstVal(im["vod_remarks"], im["remarks"])),
			Year:    asString(im["vod_year"]),
		})
	}
	return out
}

func detailFromResult(id string, v any) model.Detail {
	var item any
	if list := digList(v); len(list) > 0 {
		item = list[0]
	} else {
		item = v
	}
	m, _ := item.(map[string]any)
	if m == nil {
		if s, ok := v.(string); ok && looksMedia(s) {
			return model.Detail{ID: id, Title: id, Lines: model.ParsePlay("", s)}
		}
		return model.Detail{ID: id}
	}
	d := model.Detail{
		ID:       asString(firstVal(m["vod_id"], m["id"], id)),
		Title:    asString(firstVal(m["vod_name"], m["name"])),
		Pic:      asString(firstVal(m["vod_pic"], m["pic"])),
		Remarks:  asString(m["vod_remarks"]),
		Year:     asString(m["vod_year"]),
		Area:     asString(m["vod_area"]),
		TypeName: asString(m["type_name"]),
		Content:  model.StripTags(asString(firstVal(m["vod_content"], m["desc"]))),
		Lines:    model.ParsePlay(asString(m["vod_play_from"]), asString(m["vod_play_url"])),
	}
	if d.ID == "" {
		d.ID = id
	}
	return d
}

func playableFrom(v any, fallback string) model.Playable {
	switch t := v.(type) {
	case string:
		u := t
		if u == "" {
			u = fallback
		}
		return model.Playable{URL: u, NeedsParse: !looksMedia(u)}
	case map[string]any:
		u := asString(firstVal(t["url"], t["playUrl"], t["play_url"]))
		if u == "" {
			u = fallback
		}
		headers := map[string]string{}
		if hm, ok := t["header"].(map[string]any); ok {
			for k, val := range hm {
				headers[k] = asString(val)
			}
		}
		if hm, ok := t["headers"].(map[string]any); ok {
			for k, val := range hm {
				headers[k] = asString(val)
			}
		}
		parse := asString(t["parse"])
		needs := parse == "1" || parse == "true" || !looksMedia(u)
		if parse == "0" {
			needs = !looksMedia(u)
		}
		return model.Playable{URL: u, Headers: headers, NeedsParse: needs}
	default:
		return model.Playable{URL: fallback, NeedsParse: !looksMedia(fallback)}
	}
}

func digList(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case map[string]any:
		if list, ok := t["list"].([]any); ok {
			return list
		}
		if data, ok := t["data"].(map[string]any); ok {
			if list, ok := data["list"].([]any); ok {
				return list
			}
		}
		if data, ok := t["data"].([]any); ok {
			return data
		}
	}
	return nil
}

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "1"
		}
		return ""
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func firstVal(vals ...any) any {
	for _, v := range vals {
		if asString(v) != "" {
			return v
		}
	}
	return nil
}

func looksMedia(raw string) bool {
	s := strings.ToLower(strings.TrimSpace(raw))
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	for _, ext := range []string{".m3u8", ".mp4", ".m4v", ".flv", ".mkv", ".webm", ".mov"} {
		if strings.HasSuffix(s, ext) {
			return true
		}
	}
	return false
}
