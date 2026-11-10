package live

import "testing"

func TestM3U(t *testing.T) {
	raw := []byte("\uFEFF#EXTM3U\n#EXTINF:-1 tvg-id=\"cctv1\" tvg-logo=\"http://l/a.png\" group-title=\"央视\",CCTV-1\nhttp://cdn/1.m3u8\n#EXTINF:-1,CCTV-2\nhttp://cdn/2.m3u8\nhttp://orphan\n")
	ch := Parse(raw)
	if len(ch) != 2 {
		t.Fatalf("%+v", ch)
	}
	if ch[0].Name != "CCTV-1" || ch[0].Group != "央视" || ch[0].Logo != "http://l/a.png" || ch[0].URL != "http://cdn/1.m3u8" {
		t.Fatalf("%+v", ch[0])
	}
	if ch[1].Name != "CCTV-2" {
		t.Fatalf("%+v", ch[1])
	}
}

func TestTXTGenre(t *testing.T) {
	raw := []byte("央视,#genre#\n综合,http://a/1\n# comment\n卫视,#genre#\n湖南,http://b/2\n坏行,不是地址\n")
	ch := Parse(raw)
	if len(ch) != 2 {
		t.Fatalf("%+v", ch)
	}
	if ch[0].Group != "央视" || ch[0].Name != "综合" {
		t.Fatalf("%+v", ch[0])
	}
	if ch[1].Group != "卫视" || ch[1].URL != "http://b/2" {
		t.Fatalf("%+v", ch[1])
	}
}
