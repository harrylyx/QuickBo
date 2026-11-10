package drpy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRuleObject(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/list/1/1.html", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<div class="row"><a title="海边" href="/v/9"><img data-src="/p.jpg"><span class="sub">HD</span></a></div>`))
	})
	mux.HandleFunc("/v/9", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<h1>海边</h1><div class="desc">2024</div><div class="content">一段简介</div>
<div class="tab">高清</div><div class="tab">备用</div>
<div class="eps"><a href="http://cdn/a.m3u8">第1集</a></div>
<div class="eps"><a href="http://cdn/b.mp4">第1集</a></div>`))
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "wd=%E6%B5%B7") && !strings.Contains(r.URL.Query().Get("wd"), "海") {
			w.Write([]byte(`<div class="row"></div>`))
			return
		}
		w.Write([]byte(`<div class="row"><a title="海边" href="/v/9"><img data-src="/p.jpg"><span class="sub">HD</span></a></div>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	script := `
	var rule = {
		title: 'demo',
		host: '` + srv.URL + `',
		url: '/list/fyclass/fypage.html',
		searchUrl: '/search?wd=**&pg=fypage',
		class_name: '电影&剧集',
		class_url: '1&2',
		一级: '.row;a&&title;img&&data-src;.sub&&Text;a&&href',
		搜索: '.row;a&&title;img&&data-src;.sub&&Text;a&&href',
		二级: {
			title: 'h1&&Text',
			img: 'img&&data-src',
			desc: '.desc&&Text',
			content: '.content&&Text',
			tabs: '.tab',
			lists: '.eps:eq(#id) a'
		}
	}`
	e, err := Compile(script)
	if err != nil {
		t.Fatal(err)
	}
	cats, err := e.Categories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) < 2 || cats[len(cats)-1].Name != "剧集" {
		t.Fatalf("cats %+v", cats)
	}
	page, err := e.List(context.Background(), "1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Title != "海边" || !strings.HasSuffix(page.Items[0].Pic, "/p.jpg") {
		t.Fatalf("%+v", page.Items)
	}
	d, err := e.Detail(context.Background(), page.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "海边" || d.Content != "一段简介" || len(d.Lines) != 2 {
		t.Fatalf("%+v", d)
	}
	if d.Lines[0].Episodes[0].URL != "http://cdn/a.m3u8" {
		t.Fatalf("%+v", d.Lines[0])
	}
	found, err := e.Search(context.Background(), "海", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Items) != 1 {
		t.Fatalf("search %+v", found.Items)
	}
}

func TestFunctionSpider(t *testing.T) {
	script := `
	function home() {
		return JSON.stringify({class:[{type_id:'1', type_name:'电影'}]});
	}
	function category(tid, pg) {
		return JSON.stringify({list:[{vod_id:'9', vod_name:'示例', vod_pic:'', vod_remarks:'HD'}]});
	}
	function detail(id) {
		return JSON.stringify({list:[{vod_id:id, vod_name:'示例', vod_play_from:'线路', vod_play_url:'第1集$http://cdn/a.m3u8'}]});
	}
	function search(wd) {
		return JSON.stringify({list:[{vod_id:'9', vod_name:wd}]});
	}
	function play(flag, id) {
		return JSON.stringify({url:'http://cdn/a.m3u8', parse:0});
	}
	`
	e, err := Compile(script)
	if err != nil {
		t.Fatal(err)
	}
	cats, err := e.Categories(context.Background())
	if err != nil || len(cats) != 1 || cats[0].Name != "电影" {
		t.Fatalf("cats %+v %v", cats, err)
	}
	page, err := e.List(context.Background(), "1", 1)
	if err != nil || page.Items[0].Title != "示例" || page.Items[0].ID != "9" {
		t.Fatalf("%+v %v", page, err)
	}
	d, err := e.Detail(context.Background(), "9")
	if err != nil || len(d.Lines) != 1 || d.Lines[0].Episodes[0].URL != "http://cdn/a.m3u8" {
		t.Fatalf("%+v %v", d, err)
	}
	pb, err := e.Resolve(context.Background(), "线路", d.Lines[0].Episodes[0])
	if err != nil || pb.URL != "http://cdn/a.m3u8" || pb.NeedsParse {
		t.Fatalf("%+v %v", pb, err)
	}
	got, err := e.Search(context.Background(), "关键字", 1)
	if err != nil || got.Items[0].Title != "关键字" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestJSONListSelector(t *testing.T) {
	body := `{"data":{"list":[{"vod_name":"甲","vod_pic":"http://i/a","vod_remarks":"HD","vod_id":"1"}]}}`
	items := parseList(body, "json:data.list;vod_name;vod_pic;vod_remarks;vod_id", "http://h")
	if len(items) != 1 || items[0].Title != "甲" || items[0].ID != "1" {
		t.Fatalf("%+v", items)
	}
}
