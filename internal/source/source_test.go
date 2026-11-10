package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/harrylyx/mybox/internal/config"
)

func TestJarDoesNotPanic(t *testing.T) {
	cfg, err := config.Parse([]byte(`{
		"spider": "https://example.com/spider.jar",
		"sites": [
			{"key":"ok","name":"CMS","type":1,"api":"https://cms.example/vod"},
			{"key":"jar","name":"蜘蛛","type":3,"api":"csp_AppYS","ext":"https://example.com/x"},
			{"key":"py","name":"py","type":3,"api":"https://example.com/a.py"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	cat := Open(cfg)
	if len(cat.Sites) != 3 {
		t.Fatalf("sites %d", len(cat.Sites))
	}
	jar := cat.Find("jar")
	if jar.Meta().Level != config.LevelNo || !strings.Contains(jar.Meta().Reason, "JAR") {
		t.Fatalf("%+v", jar.Meta())
	}
	if _, err := jar.Categories(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if _, err := jar.Detail(context.Background(), "1"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := jar.List(context.Background(), "", 1); err == nil {
		t.Fatal("list should fail")
	}
}

func TestCMSThroughCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ac") == "detail" {
			w.Write([]byte(`{"code":1,"list":[{"vod_id":"3","vod_name":"片","vod_play_from":"线","vod_play_url":"正片$http://cdn/a.mp4"}]}`))
			return
		}
		if r.URL.Query().Get("ac") == "list" {
			w.Write([]byte(`{"code":1,"class":[{"type_id":"1","type_name":"电影"}]}`))
			return
		}
		w.Write([]byte(`{"code":1,"page":1,"pagecount":1,"list":[{"vod_id":"3","vod_name":"片","vod_pic":"http://img/a.jpg"}]}`))
	}))
	defer srv.Close()
	cfg, err := config.Parse([]byte(`{"sites":[{"key":"c","name":"测","type":1,"api":"` + srv.URL + `"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	src := Open(cfg).Find("c")
	cats, err := src.Categories(context.Background())
	if err != nil || cats[0].Name != "最新" || cats[1].Name != "电影" {
		t.Fatalf("%+v %v", cats, err)
	}
	page, err := src.List(context.Background(), "", 1)
	if err != nil || page.Items[0].Title != "片" {
		t.Fatalf("%+v %v", page, err)
	}
	d, err := src.Detail(context.Background(), "3")
	if err != nil || d.Lines[0].Episodes[0].URL != "http://cdn/a.mp4" {
		t.Fatalf("%+v %v", d, err)
	}
}
