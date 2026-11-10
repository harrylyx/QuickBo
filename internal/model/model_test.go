package model

import "testing"

func TestParsePlay(t *testing.T) {
	lines := ParsePlay("高清$$$备用", "第1集$http://a/1.m3u8#第2集$http://a/2.m3u8$$$第1集$http://b/1.mp4")
	if len(lines) != 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	if lines[0].Name != "高清" || len(lines[0].Episodes) != 2 {
		t.Fatalf("line0 = %+v", lines[0])
	}
	if lines[0].Episodes[1].Name != "第2集" || lines[0].Episodes[1].URL != "http://a/2.m3u8" {
		t.Fatalf("ep = %+v", lines[0].Episodes[1])
	}
	if lines[1].Name != "备用" || lines[1].Episodes[0].URL != "http://b/1.mp4" {
		t.Fatalf("line1 = %+v", lines[1])
	}
}

func TestParsePlayBareURL(t *testing.T) {
	lines := ParsePlay("", "http://cdn/a.mp4")
	if len(lines) != 1 || len(lines[0].Episodes) != 1 {
		t.Fatalf("%+v", lines)
	}
	if lines[0].Episodes[0].URL != "http://cdn/a.mp4" {
		t.Fatalf("%+v", lines[0].Episodes[0])
	}
}

func TestResolveURLAndPreferredLine(t *testing.T) {
	if got := ResolveURL("https://api.example/vod", "//cdn.example/a.jpg"); got != "https://cdn.example/a.jpg" {
		t.Fatalf("protocol relative %q", got)
	}
	if got := ResolveURL("https://api.example/provide/vod/", "/cover/a.jpg"); got != "https://api.example/cover/a.jpg" {
		t.Fatalf("relative %q", got)
	}
	lines := ParsePlay("share$$$m3u8", "1$https://cdn/share/abc$$$1$https://cdn/a/index.m3u8")
	if PreferredLine(lines) != 1 {
		t.Fatalf("preferred %d", PreferredLine(lines))
	}
}

func TestStripTags(t *testing.T) {
	if got := StripTags("<p>你好&amp;世界</p>"); got != "你好&世界" {
		t.Fatalf("%q", got)
	}
}
