package app

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/source"
)

// videoFake is an embedded player (player.VideoSource) that hands out one
// solid red frame at whatever size the view asks for.
type videoFake struct {
	w, h  int
	taken bool
}

func (f *videoFake) Load(context.Context, string, map[string]string) error { return nil }
func (f *videoFake) Play() error                                           { return nil }
func (f *videoFake) Pause() error                                          { return nil }
func (f *videoFake) Seek(time.Duration) error                              { return nil }
func (f *videoFake) SetRate(float64) error                                 { return nil }
func (f *videoFake) SetVolume(float64) error                               { return nil }
func (f *videoFake) SetMuted(bool) error                                   { return nil }
func (f *videoFake) SetFullscreen(bool) error                              { return nil }
func (f *videoFake) Fullscreen() bool                                      { return false }
func (f *videoFake) Muted() bool                                           { return false }
func (f *videoFake) Ready() bool                                           { return true }
func (f *videoFake) Playing() bool                                         { return false }
func (f *videoFake) Ended() bool                                           { return false }
func (f *videoFake) Buffering() bool                                       { return false }
func (f *videoFake) Position() time.Duration                               { return 0 }
func (f *videoFake) Duration() time.Duration                               { return time.Minute }
func (f *videoFake) Buffered() time.Duration                               { return 0 }
func (f *videoFake) Err() error                                            { return nil }
func (f *videoFake) Close() error                                          { return nil }
func (f *videoFake) SetVideoSize(w, h int)                                 { f.w, f.h = w, h }

func (f *videoFake) TakeFrame() *image.RGBA {
	if f.taken || f.w == 0 {
		return nil
	}
	f.taken = true
	img := image.NewRGBA(image.Rect(0, 0, f.w, f.h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 0xff, 0xff
	}
	return img
}

func TestEmbeddedVideoDrawsInsideTheWindow(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}}
	a.screen = screenPlay
	fake := &videoFake{}
	a.player = fake
	tt := ui.NewTester(a.View, 1200, 800)
	tt.Frame()
	if fake.w == 0 || fake.h == 0 {
		t.Fatal("the view never told the player its video size")
	}
	if tt.HasText("视频在 mpv 窗口中播放") {
		t.Fatal("embedded player still shows the mpv-window placeholder")
	}
	img := tt.Image()
	b := img.Bounds()
	if c := img.RGBAAt(b.Dx()/2, b.Dy()/3); c != (color.RGBA{0xff, 0, 0, 0xff}) {
		t.Fatalf("centre of the stage is %v, want the video's red", c)
	}
	if !a.embedded() {
		t.Fatal("embedded() false for a VideoSource player")
	}
}
