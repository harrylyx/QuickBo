package xbpq

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleRule = `{
  // 注释
  "主页url": "http://home",
  "分类": "电影$1#剧集$2",
  "分类url": "http://home/list/{cateId}/{catePg}.html",
  "数组": "class=\"item\"&&</div>",
  "标题": "title=\"&&\"",
  "图片": "data-src=\"&&\"",
  "链接": "href=\"&&\"",
  "副标题": "class=\"sub\">&&<",
  "搜索url": "http://home/search?wd={wd}",
  "线路数组": "class=\"from\"&&</div>",
  "线路标题": ">&&",
  "播放数组": "class=\"eps\"&&</div>",
  "播放列表": "<a&&</a>",
  "播放标题": ">&&",
  "播放链接": "href=\"&&\"",
  "请求头": "User-Agent$MOBILE_UA"
}`

func TestParseAndList(t *testing.T) {
	r, err := Parse([]byte(sampleRule))
	if err != nil {
		t.Fatal(err)
	}
	e := New(r)
	cats := e.Categories()
	if len(cats) != 2 || cats[0].ID != "1" || cats[1].Name != "剧集" {
		t.Fatalf("%+v", cats)
	}
	if r.Headers["User-Agent"] != "MOBILE_UA" && !strings.Contains(r.Headers["User-Agent"], "Mozilla") {
		t.Fatalf("headers %+v", r.Headers)
	}
	page := `<div class="item" title="片A" data-src="http://img/a.jpg"><a href="/v/1">x</a><span class="sub">HD</span></div>
<div class="item" title="片B" data-src="/b.jpg"><a href="/v/2">x</a><span class="sub">更新</span></div>`
	items := e.items(page, r.Array, r.Title, r.Link, r.Pic, r.Subtitle)
	if len(items) != 2 || items[0].Title != "片A" || items[0].ID != "http://home/v/1" {
		t.Fatalf("%+v", items)
	}
	if items[1].Pic != "http://home/b.jpg" || items[1].Remarks != "更新" {
		t.Fatalf("%+v", items[1])
	}
}

func TestBrowseDetailOverHTTP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/list/1/1.html", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<div class="item" title="海边" data-src="http://img/s.jpg"><a href="/play/9">x</a><span class="sub">4K</span></div>`))
	})
	mux.HandleFunc("/play/9", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<h1>海边</h1><div class="from">高清</div><div class="eps"><a href="http://cdn/a.m3u8">第1集</a><a href="/ep/2">第2集</a></div>`))
	})
	mux.HandleFunc("/ep/2", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`var url = "http://cdn/b.mp4";`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	raw := strings.ReplaceAll(sampleRule, "http://home", srv.URL)
	r, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	e := New(r)
	page, err := e.List(context.Background(), "1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Title != "海边" {
		t.Fatalf("%+v", page.Items)
	}
	d, err := e.Detail(context.Background(), page.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "海边" || len(d.Lines) != 1 || len(d.Lines[0].Episodes) != 2 {
		t.Fatalf("%+v", d)
	}
	if d.Lines[0].Name != "高清" || d.Lines[0].Episodes[0].URL != "http://cdn/a.m3u8" {
		t.Fatalf("%+v", d.Lines[0])
	}
	pb, err := e.Resolve(context.Background(), d.Lines[0].Episodes[1])
	if err != nil {
		t.Fatal(err)
	}
	if pb.URL != "http://cdn/b.mp4" {
		t.Fatalf("resolved %s", pb.URL)
	}
}
