package cms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestJSONListDetailAndSearch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/vod", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("ac") == "list":
			w.Write([]byte(`{"code":1,"class":[{"type_id":1,"type_name":"电影"},{"type_id":"2","type_name":"剧集"}]}`))
		case q.Get("wd") != "":
			w.Write([]byte(`{"code":1,"page":"1","pagecount":"3","total":"40","list":[{"vod_id":"9","vod_name":"找到了","vod_pic":"http://img/a.jpg","vod_remarks":"HD"}]}`))
		case q.Get("ac") == "detail":
			if q.Get("ids") != "9" {
				http.Error(w, "bad id", 400)
				return
			}
			w.Write([]byte(`{"code":1,"list":[{"vod_id":9,"vod_name":"找到了","vod_pic":"http://img/a.jpg","vod_year":"2024","vod_area":"大陆","vod_content":"<p>简介</p>","vod_play_from":"线路A$$$线路B","vod_play_url":"第1集$http://cdn/1.m3u8#第2集$http://cdn/2.m3u8$$$第1集$http://cdn/1.mp4"}]}`))
		default:
			w.Write([]byte(`{"code":1,"page":1,"pagecount":2,"list":[{"vod_id":9,"vod_name":"示例","vod_pic":"http://img/a.jpg","vod_remarks":"更新"}]}`))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL + "/vod")
	cats, err := c.Categories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 2 || cats[0].ID != "1" || cats[1].ID != "2" {
		t.Fatalf("cats %+v", cats)
	}
	page, err := c.List(context.Background(), "1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Title != "示例" || page.PageCount != 2 {
		t.Fatalf("page %+v", page)
	}
	found, err := c.Search(context.Background(), "示例", 1)
	if err != nil {
		t.Fatal(err)
	}
	if found.Items[0].ID != "9" || found.PageCount != 3 {
		t.Fatalf("search %+v", found)
	}
	d, err := c.Detail(context.Background(), "9")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "找到了" || d.Content != "简介" || len(d.Lines) != 2 {
		t.Fatalf("detail %+v", d)
	}
	if d.Lines[0].Episodes[1].URL != "http://cdn/2.m3u8" {
		t.Fatalf("ep %+v", d.Lines[0].Episodes)
	}
}

func TestXMLAndHTMLError(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="utf-8"?>
<rss>
  <class><ty id="1">电影</ty><ty id="2">动漫</ty></class>
  <list page="1" pagecount="4" recordcount="10">
    <video>
      <id>7</id><name><![CDATA[XML片]]></name><pic>http://img/x.jpg</pic><note>HD</note>
      <des><![CDATA[一部片子]]></des>
      <dl>
        <dd flag="m3u8"><![CDATA[第1集$http://cdn/a.m3u8#第2集$http://cdn/b.m3u8]]></dd>
      </dl>
    </video>
  </list>
</rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "bad") {
			w.Write([]byte("<html><body>parked</body></html>"))
			return
		}
		w.Write([]byte(xmlBody))
	}))
	defer srv.Close()
	c := New(srv.URL)
	cats, err := c.Categories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 2 || cats[0].Name != "电影" {
		t.Fatalf("cats %+v", cats)
	}
	page, err := c.List(context.Background(), "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if page.Items[0].Title != "XML片" || page.PageCount != 4 {
		t.Fatalf("page %+v", page)
	}
	d, err := c.Detail(context.Background(), "7")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Lines) != 1 || d.Lines[0].Name != "m3u8" || len(d.Lines[0].Episodes) != 2 {
		t.Fatalf("detail %+v", d)
	}
	if d.Content != "一部片子" {
		t.Fatalf("content %q", d.Content)
	}

	bad := New(srv.URL + "/bad")
	if _, err := bad.List(context.Background(), "", 1); err == nil || !strings.Contains(err.Error(), "不是 JSON 或 XML") {
		t.Fatalf("err = %v", err)
	}
}

func TestGBKJSON(t *testing.T) {
	plain := []byte(`{"code":1,"page":1,"pagecount":1,"list":[{"vod_id":"1","vod_name":"中文片","vod_pic":"","vod_remarks":""}]}`)
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes(plain)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=gbk")
		w.Write(gbk)
	}))
	defer srv.Close()
	page, err := New(srv.URL).List(context.Background(), "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if page.Items[0].Title != "中文片" {
		t.Fatalf("title %q", page.Items[0].Title)
	}
}

func TestProtocolRelativePicture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":1,"page":1,"pagecount":1,"list":[{"vod_id":1,"vod_name":"相对","vod_pic":"//cdn.example/a.jpg","vod_play_from":"m","vod_play_url":"1$/v/1"}]}`))
	}))
	defer srv.Close()
	c := New(srv.URL + "/api.php/provide/vod/")
	page, err := c.List(context.Background(), "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if page.Items[0].Pic != "https://cdn.example/a.jpg" {
		t.Fatalf("pic %q", page.Items[0].Pic)
	}
	d, err := c.Detail(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d.Lines[0].Episodes[0].URL, "http") || !strings.HasSuffix(d.Lines[0].Episodes[0].URL, "/v/1") {
		t.Fatalf("ep %q", d.Lines[0].Episodes[0].URL)
	}
}

func TestStringIDsDoNotFailTheList(t *testing.T) {
	body := []byte(`{"code":1,"list":[{"vod_id":"tv-1","vod_name":"甲"},{"vod_id":2,"vod_name":"乙"}]}`)
	_, page, err := parseDocument(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Page.Items) != 2 || page.Page.Items[0].ID != "tv-1" || page.Page.Items[1].ID != "2" {
		t.Fatalf("%+v", page.Page.Items)
	}
}
