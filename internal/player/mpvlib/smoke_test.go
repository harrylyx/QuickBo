package mpvlib

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestSmoke plays mpv's test pattern through the real libmpv. It needs libmpv
// installed, so it only runs with QUICKBO_LIBMPV_TEST=1.
func TestSmoke(t *testing.T) {
	if os.Getenv("QUICKBO_LIBMPV_TEST") != "1" {
		t.Skip("set QUICKBO_LIBMPV_TEST=1 to run against the installed libmpv")
	}
	path, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("libmpv: %s", path)
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	src := "av://lavfi:testsrc2=size=1280x720:rate=30:duration=20"
	if err := p.Load(context.Background(), src, map[string]string{"User-Agent": "QuickBo-test", "X-Test": "a, b"}); err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	p.SetVideoSize(963, 540) // odd width: must round to a multiple of 16

	deadline := time.Now().Add(10 * time.Second)
	frames, litFrames := 0, 0
	start := time.Now()
	for time.Since(start) < 3*time.Second && time.Now().Before(deadline) {
		if img := p.TakeFrame(); img != nil {
			frames++
			if img.Rect.Dx() != 960 || img.Rect.Dy() != 540 || img.Stride != 4*960 {
				t.Fatalf("frame %v stride %d", img.Rect, img.Stride)
			}
			{
				var lit, opaque int
				for i := 0; i < len(img.Pix); i += 4 {
					if img.Pix[i]|img.Pix[i+1]|img.Pix[i+2] != 0 {
						lit++
					}
					if img.Pix[i+3] == 255 {
						opaque++
					}
				}
				if opaque != len(img.Pix)/4 {
					t.Fatal("frame not opaque")
				}
				if lit > len(img.Pix)/8 {
					if litFrames == 0 {
						t.Logf("first picture after %v (frame %d): lit=%d/%d", time.Since(start), frames, lit, len(img.Pix)/4)
					}
					litFrames++
				}
			}
		}
		if err := p.Err(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Logf("frames in ~3s: %d, ready=%v playing=%v pos=%v dur=%v buf=%v",
		frames, p.Ready(), p.Playing(), p.Position(), p.Duration(), p.Buffered())
	if litFrames == 0 {
		t.Fatal("no frame showed the picture")
	}
	if frames < 30 {
		t.Fatalf("only %d frames", frames)
	}
	if p.Position() < 2*time.Second {
		t.Fatalf("position did not advance: %v", p.Position())
	}

	if err := p.Pause(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if p.Playing() {
		t.Fatal("still playing after Pause")
	}
	if err := p.Seek(15 * time.Second); err != nil {
		t.Fatal(err)
	}
	if err := p.SetRate(2); err != nil {
		t.Fatal(err)
	}
	if err := p.Play(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if pos := p.Position(); pos < 16*time.Second {
		t.Fatalf("after seek to 15s at 2x for 1s: %v", pos)
	}
	// Resizing re-renders even when paused.
	p.Pause()
	time.Sleep(200 * time.Millisecond)
	p.TakeFrame()
	p.SetVideoSize(320, 180)
	time.Sleep(300 * time.Millisecond)
	if img := p.TakeFrame(); img == nil || img.Rect.Dx() != 320 {
		t.Fatalf("no re-render at the new size while paused: %v", img)
	}

	// Reload on the same player, then close.
	if err := p.Load(context.Background(), "av://lavfi:testsrc=duration=2", nil); err != nil {
		t.Fatal(err)
	}
	p.SetVideoSize(640, 360)
	time.Sleep(3500 * time.Millisecond)
	if !p.Ended() {
		t.Fatalf("2s clip not ended: pos=%v", p.Position())
	}
	closeStart := time.Now()
	p.Close()
	t.Logf("close took %v", time.Since(closeStart))
}

// TestBadURL checks that an unreachable stream surfaces through Err.
func TestBadURL(t *testing.T) {
	if os.Getenv("QUICKBO_LIBMPV_TEST") != "1" {
		t.Skip("set QUICKBO_LIBMPV_TEST=1 to run against the installed libmpv")
	}
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Load(context.Background(), "http://127.0.0.1:1/nope.m3u8", nil); err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for i := 0; i < 100 && p.Err() == nil; i++ {
		time.Sleep(50 * time.Millisecond)
	}
	if p.Err() == nil {
		t.Fatal("no error for an unreachable stream")
	}
	t.Logf("err: %v", p.Err())
}

func TestFitRenderSize(t *testing.T) {
	for _, c := range []struct{ w, h, ww, wh int }{
		{0, 10, 0, 0}, {1, 1, 16, 16}, {1920, 1080, 1920, 1080}, {963, 540, 960, 540}, {7680, 4320, 3840, 2160},
	} {
		if w, h := fitRenderSize(c.w, c.h); w != c.ww || h != c.wh {
			t.Errorf("fitRenderSize(%d,%d) = %d,%d want %d,%d", c.w, c.h, w, h, c.ww, c.wh)
		}
	}
}

func TestFrameQueueRecycles(t *testing.T) {
	q := &frameQueue{}
	a := q.get(16, 16)
	q.put(a)
	if q.take() != a {
		t.Fatal("take a")
	}
	b := q.get(16, 16)
	if b == a {
		t.Fatal("shown frame reused while on screen")
	}
	q.put(b)
	q.take() // a goes back to free
	if c := q.get(16, 16); c != a {
		t.Fatal("expected a recycled")
	}
	if uintptr(len(a.Pix)) != 16*16*4 {
		t.Fatal("pix length")
	}
}
