package player

import (
	"bytes"
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMeasureReadsMasterPlaylist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=1280x720\nlow.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=3000000,RESOLUTION=1920x1080\nhigh.m3u8\n"))
	}))
	defer srv.Close()
	w, h := Measure(context.Background(), srv.URL+"/a.m3u8", nil)
	if w != 1920 || h != 1080 {
		t.Fatalf("size %dx%d", w, h)
	}
}

func TestMP4BoxWalk(t *testing.T) {
	file := mp4File(1920, 1080)
	w, h := mp4Size(file)
	if w != 1920 || h != 1080 {
		t.Fatalf("direct %dx%d len %d", w, h, len(file))
	}
}

func TestMeasureReadsTrackHeader(t *testing.T) {
	file := mp4File(1920, 1080)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "a.mp4", time.Time{}, bytes.NewReader(file))
	}))
	defer srv.Close()
	w, h := Measure(context.Background(), srv.URL+"/a.mp4", nil)
	if w != 1920 || h != 1080 {
		t.Fatalf("size %dx%d", w, h)
	}
}

func TestMeasureFindsMoovAfterMediaData(t *testing.T) {
	head := append(box("ftyp", []byte("isom")), box("mdat", bytesOf(64))...)
	// Rewrite mdat size so the header claims the media sits before moov,
	// while the bytes we actually stored are only the header plus padding.
	moov := box("moov", box("trak", box("tkhd", tkhd(1280, 720))))
	gap := 200 << 10
	file := make([]byte, 0, len(head)+gap+len(moov))
	file = append(file, head...)
	file = append(file, make([]byte, gap)...)
	file = append(file, moov...)
	mdatSize := uint32(8 + 64 + gap)
	binary.BigEndian.PutUint32(file[len(box("ftyp", []byte("isom"))):], mdatSize)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "b.mp4", time.Time{}, bytes.NewReader(file))
	}))
	defer srv.Close()
	w, h := Measure(context.Background(), srv.URL+"/b.mp4", nil)
	if w != 1280 || h != 720 {
		t.Fatalf("size %dx%d", w, h)
	}
}

func mp4File(w, h int) []byte {
	return append(box("ftyp", []byte("isom")), box("moov", box("trak", box("tkhd", tkhd(w, h))))...)
}

func tkhd(w, h int) []byte {
	payload := make([]byte, 84)
	binary.BigEndian.PutUint32(payload[76:], uint32(w)<<16)
	binary.BigEndian.PutUint32(payload[80:], uint32(h)<<16)
	return payload
}

func box(kind string, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(out[:4], uint32(len(out)))
	copy(out[4:8], kind)
	copy(out[8:], payload)
	return out
}

func bytesOf(n int) []byte { return make([]byte, n) }
