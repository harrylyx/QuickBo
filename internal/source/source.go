// Package source turns a parsed subscription into drivers the UI can call.
// JAR/DEX spiders are present in the catalog and report themselves as
// unsupported; calling them returns an error instead of loading a jar.
package source

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/harrylyx/QuickBo/internal/cms"
	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/drpy"
	"github.com/harrylyx/QuickBo/internal/httpx"
	"github.com/harrylyx/QuickBo/internal/live"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/xbpq"
)

// Meta describes a site for the source list.
type Meta struct {
	Key    string
	Name   string
	API    string
	Kind   string
	Level  config.Level
	Reason string
}

// Source is one on-demand site.
type Source interface {
	Meta() Meta
	Categories(ctx context.Context) ([]model.Category, error)
	List(ctx context.Context, cat string, page int) (model.Page, error)
	Search(ctx context.Context, q string, page int) (model.Page, error)
	Detail(ctx context.Context, id string) (model.Detail, error)
	Resolve(ctx context.Context, line string, ep model.Episode) (model.Playable, error)
}

// LiveEntry is one live playlist from the subscription.
type LiveEntry struct {
	Name string
	URL  string
	mu   sync.Mutex
	got  []live.Channel
	done bool
}

// Load fetches and parses the playlist once.
func (e *LiveEntry) Load(ctx context.Context) ([]live.Channel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.done {
		return e.got, nil
	}
	body, _, err := httpx.Get(ctx, nil, e.URL, map[string]string{"User-Agent": httpx.TVBoxUA}, 8<<20)
	if err != nil {
		return nil, err
	}
	e.got = live.Parse(body)
	e.done = true
	if len(e.got) == 0 {
		return nil, fmt.Errorf("播放列表里没有频道")
	}
	return e.got, nil
}

// Catalog is everything a subscription can drive.
type Catalog struct {
	Sites  []Source
	Lives  []*LiveEntry
	Parses []config.ParseItem
	Index  []config.NamedURL
}

// Open builds a catalog. Unsupported sites are included so the UI can
// show them; they are not contacted.
func Open(cfg *config.Config) *Catalog {
	cat := &Catalog{}
	if cfg == nil {
		return cat
	}
	cat.Parses = cfg.Parses
	cat.Index = cfg.IndexEntries()
	for _, site := range cfg.Sites {
		cat.Sites = append(cat.Sites, newSite(site))
	}
	for _, lv := range cfg.Lives {
		if strings.TrimSpace(lv.URL) == "" {
			continue
		}
		name := lv.Name
		if name == "" {
			name = lv.URL
		}
		cat.Lives = append(cat.Lives, &LiveEntry{Name: name, URL: lv.URL})
	}
	return cat
}

// Find returns the site with key, or nil.
func (c *Catalog) Find(key string) Source {
	if c == nil {
		return nil
	}
	for _, s := range c.Sites {
		if s.Meta().Key == key {
			return s
		}
	}
	return nil
}

func newSite(site config.Site) Source {
	cap := config.Classify(site)
	meta := Meta{
		Key: site.Key, Name: site.Name, API: site.API,
		Kind: cap.Kind, Level: cap.Level, Reason: cap.Reason,
	}
	if meta.Name == "" {
		meta.Name = meta.Key
	}
	if meta.Key == "" {
		meta.Key = meta.Name
	}
	switch cap.Kind {
	case config.KindCMSJSON, config.KindCMSXML:
		return &cmsSource{meta: meta, client: cms.New(site.API)}
	case config.KindXBPQ, config.KindXYQ:
		return &ruleSource{meta: meta, ext: site.ExtString()}
	case config.KindJS:
		return &jsSource{meta: meta, api: site.API, ext: site.ExtString()}
	default:
		return deadSource{meta: meta}
	}
}

type deadSource struct{ meta Meta }

func (d deadSource) Meta() Meta { return d.meta }

func (d deadSource) Categories(context.Context) ([]model.Category, error) {
	return nil, d.err()
}
func (d deadSource) List(context.Context, string, int) (model.Page, error) {
	return model.Page{}, d.err()
}
func (d deadSource) Search(context.Context, string, int) (model.Page, error) {
	return model.Page{}, d.err()
}
func (d deadSource) Detail(context.Context, string) (model.Detail, error) {
	return model.Detail{}, d.err()
}
func (d deadSource) Resolve(context.Context, string, model.Episode) (model.Playable, error) {
	return model.Playable{}, d.err()
}
func (d deadSource) err() error {
	if d.meta.Reason != "" {
		return fmt.Errorf("%s", d.meta.Reason)
	}
	return fmt.Errorf("不支持的站点")
}

type cmsSource struct {
	meta   Meta
	client *cms.Client
}

func (s *cmsSource) Meta() Meta { return s.meta }

func (s *cmsSource) Categories(ctx context.Context) ([]model.Category, error) {
	cats, err := s.client.Categories(ctx)
	if err != nil && len(cats) == 0 {
		return []model.Category{{ID: "", Name: "最新"}}, nil
	}
	out := []model.Category{{ID: "", Name: "最新"}}
	out = append(out, cats...)
	return out, nil
}

func (s *cmsSource) List(ctx context.Context, cat string, page int) (model.Page, error) {
	return s.client.List(ctx, cat, page)
}
func (s *cmsSource) Search(ctx context.Context, q string, page int) (model.Page, error) {
	return s.client.Search(ctx, q, page)
}
func (s *cmsSource) Detail(ctx context.Context, id string) (model.Detail, error) {
	return s.client.Detail(ctx, id)
}
func (s *cmsSource) Resolve(_ context.Context, _ string, ep model.Episode) (model.Playable, error) {
	return model.Playable{URL: ep.URL, Headers: map[string]string{"User-Agent": httpx.TVBoxUA}, NeedsParse: false}, nil
}

type ruleSource struct {
	meta Meta
	ext  string
	once sync.Once
	eng  *xbpq.Engine
	err  error
}

func (s *ruleSource) Meta() Meta { return s.meta }

func (s *ruleSource) ensure(ctx context.Context) error {
	s.once.Do(func() {
		body, err := readExt(ctx, s.ext)
		if err != nil {
			s.err = err
			return
		}
		rule, err := xbpq.Parse(body)
		if err != nil {
			s.err = err
			return
		}
		s.eng = xbpq.New(rule)
	})
	return s.err
}

func (s *ruleSource) Categories(ctx context.Context) ([]model.Category, error) {
	if err := s.ensure(ctx); err != nil {
		return nil, err
	}
	cats := s.eng.Categories()
	if len(cats) == 0 {
		return nil, fmt.Errorf("规则没有分类")
	}
	return cats, nil
}

func (s *ruleSource) List(ctx context.Context, cat string, page int) (model.Page, error) {
	if err := s.ensure(ctx); err != nil {
		return model.Page{}, err
	}
	return s.eng.List(ctx, cat, page)
}
func (s *ruleSource) Search(ctx context.Context, q string, page int) (model.Page, error) {
	if err := s.ensure(ctx); err != nil {
		return model.Page{}, err
	}
	return s.eng.Search(ctx, q, page)
}
func (s *ruleSource) Detail(ctx context.Context, id string) (model.Detail, error) {
	if err := s.ensure(ctx); err != nil {
		return model.Detail{}, err
	}
	return s.eng.Detail(ctx, id)
}
func (s *ruleSource) Resolve(ctx context.Context, _ string, ep model.Episode) (model.Playable, error) {
	if err := s.ensure(ctx); err != nil {
		return model.Playable{}, err
	}
	return s.eng.Resolve(ctx, ep)
}

type jsSource struct {
	meta Meta
	api  string
	ext  string
	once sync.Once
	eng  *drpy.Engine
	err  error
}

func (s *jsSource) Meta() Meta { return s.meta }

func (s *jsSource) ensure(ctx context.Context) error {
	s.once.Do(func() {
		code, err := loadScript(ctx, s.api, s.ext)
		if err != nil {
			s.err = err
			return
		}
		s.eng, s.err = drpy.Compile(code)
	})
	return s.err
}

func (s *jsSource) Categories(ctx context.Context) ([]model.Category, error) {
	if err := s.ensure(ctx); err != nil {
		return nil, err
	}
	return s.eng.Categories(ctx)
}
func (s *jsSource) List(ctx context.Context, cat string, page int) (model.Page, error) {
	if err := s.ensure(ctx); err != nil {
		return model.Page{}, err
	}
	return s.eng.List(ctx, cat, page)
}
func (s *jsSource) Search(ctx context.Context, q string, page int) (model.Page, error) {
	if err := s.ensure(ctx); err != nil {
		return model.Page{}, err
	}
	return s.eng.Search(ctx, q, page)
}
func (s *jsSource) Detail(ctx context.Context, id string) (model.Detail, error) {
	if err := s.ensure(ctx); err != nil {
		return model.Detail{}, err
	}
	return s.eng.Detail(ctx, id)
}
func (s *jsSource) Resolve(ctx context.Context, line string, ep model.Episode) (model.Playable, error) {
	if err := s.ensure(ctx); err != nil {
		return model.Playable{}, err
	}
	return s.eng.Resolve(ctx, line, ep)
}

func readExt(ctx context.Context, ext string) ([]byte, error) {
	ext = strings.TrimSpace(ext)
	if ext == "" {
		return nil, fmt.Errorf("规则 ext 为空")
	}
	if strings.HasPrefix(ext, "{") || strings.HasPrefix(ext, "[") {
		return []byte(ext), nil
	}
	if strings.Contains(ext, "://") || strings.HasPrefix(ext, "file:") {
		b, _, err := httpx.Get(ctx, nil, ext, nil, 2<<20)
		return b, err
	}
	return nil, fmt.Errorf("无法读取规则: %s", ext)
}

func loadScript(ctx context.Context, api, ext string) (string, error) {
	ext = strings.TrimSpace(ext)
	if strings.HasPrefix(ext, "{") || strings.HasPrefix(ext, "var ") || strings.HasPrefix(ext, "function") || strings.HasPrefix(ext, "export ") {
		return ext, nil
	}
	target := strings.TrimSpace(api)
	if isJS(ext) && isDrpyLib(api) {
		target = ext
	} else if isJS(ext) && !isJS(api) {
		target = ext
	}
	if strings.HasPrefix(target, "{") || strings.HasPrefix(target, "var ") || strings.HasPrefix(target, "function") {
		return target, nil
	}
	if !strings.Contains(target, "://") && !strings.HasPrefix(target, "file:") {
		return "", fmt.Errorf("脚本地址无效: %s", target)
	}
	b, _, err := httpx.Get(ctx, nil, target, nil, 4<<20)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func isJS(raw string) bool {
	base := strings.ToLower(strings.TrimSpace(raw))
	if i := strings.IndexAny(base, "?#"); i >= 0 {
		base = base[:i]
	}
	return strings.HasSuffix(base, ".js")
}

func isDrpyLib(raw string) bool {
	base := strings.ToLower(raw)
	if i := strings.IndexAny(base, "?#"); i >= 0 {
		base = base[:i]
	}
	i := strings.LastIndex(base, "/")
	name := base
	if i >= 0 {
		name = base[i+1:]
	}
	return strings.HasPrefix(name, "drpy") && strings.HasSuffix(name, ".js")
}
