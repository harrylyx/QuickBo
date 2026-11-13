package player

import "testing"

func TestPlaylistSizeFromVariantLabels(t *testing.T) {
	body := []byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=800000,NAME=\"480p\"\n480/index.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=3000000,NAME=\"1080p\"\n1080/index.m3u8\n")
	w, h, ok := playlistSize(body)
	if !ok || h != 1080 || w != 1920 {
		t.Fatalf("size %dx%d ok=%v", w, h, ok)
	}
}

func TestPlaylistSizeFromVariantPath(t *testing.T) {
	body := []byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000000\n/video/720/index.m3u8\n")
	w, h, ok := playlistSize(body)
	if !ok || h != 720 || w != 1280 {
		t.Fatalf("size %dx%d ok=%v", w, h, ok)
	}
}

// A segment list with timestamps in its names must not be read as a size.
func TestPlaylistSizeIgnoresSegmentNumbers(t *testing.T) {
	body := []byte("#EXTM3U\n#EXTINF:10,\nseg720.ts\n#EXTINF:10,\nseg1080.ts\n")
	if _, h, _ := playlistSize(body); h != 0 {
		t.Fatalf("segment names read as size: %d", h)
	}
}
