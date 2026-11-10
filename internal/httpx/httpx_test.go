package httpx

import (
	"bytes"
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

func TestToUTF8DecodesGBK(t *testing.T) {
	// "中" in GBK is D6 D0, which is not valid UTF-8.
	gbk := []byte{0xD6, 0xD0}
	got := ToUTF8(gbk, "text/html; charset=gbk")
	if string(got) != "中" {
		t.Fatalf("%q", got)
	}
}
