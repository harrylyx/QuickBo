package httpx

import (
	"bytes"
	"strings"
	"testing"
)

func TestToUTF8LeavesPNG(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0xff, 0xfe, 0x00, 0x01}
	got := ToUTF8(png, "image/png")
	if !bytes.Equal(got, png) {
		t.Fatalf("png changed: %x", got)
	}
	got = ToUTF8(png, "")
	if !bytes.Equal(got, png) {
		t.Fatalf("png magic changed: %x", got)
	}
}

func TestRequestURLPunycode(t *testing.T) {
	got, err := requestURL("http://www.饭太硬.cc/tv")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "xn--") || !strings.HasSuffix(got, "/tv") {
		t.Fatalf("%s", got)
	}
	plain, err := requestURL("https://example.com/a b")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain, "example.com") {
		t.Fatalf("%s", plain)
	}
}

func TestToUTF8DecodesGBK(t *testing.T) {
	// "中" in GBK is D6 D0, which is not valid UTF-8.
	gbk := []byte{0xD6, 0xD0}
	got := ToUTF8(gbk, "text/html; charset=gbk")
	if string(got) != "中" {
		t.Fatalf("%q", got)
	}
}
