package app

import (
	"image"
	"image/color"

	"github.com/egoist/mygo/ui"
)

// artExtras is what a poster gives besides itself: a blurred copy to fill a
// backdrop (the home banner, the detail page) and its average color, which
// tints the scrim over that backdrop so text stays legible without the
// picture turning grey.
type artExtras struct {
	backdrop *ui.Bitmap
	tint     ui.Color
}

// backdropWidth is the blurred copy's width. The GPU scales it up to fill
// the backdrop, which smooths it further, so it can be tiny.
const backdropWidth = 48

// rememberImage keeps a decoded poster with its backdrop and tint. Call it
// on the main thread; blur runs off it (see poster).
func (a *App) rememberImage(url string, img image.Image, blurred *image.RGBA, tint color.RGBA) {
	a.remember(url, ui.NewBitmap(img))
	if blurred != nil {
		a.art[url] = artExtras{backdrop: ui.NewBitmap(blurred), tint: ui.RGBA(tint.R, tint.G, tint.B, 1)}
	}
}

// artOf returns the backdrop and tint of a poster already loaded.
func (a *App) artOf(url string) (artExtras, bool) {
	e, ok := a.art[url]
	return e, ok
}

// blurImage shrinks img to backdropWidth wide by averaging, blurs it with
// three box passes (close to a Gaussian), and returns it with the image's
// average color.
func blurImage(img image.Image) (*image.RGBA, color.RGBA) {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, color.RGBA{}
	}
	w := backdropWidth
	h := max(1, w*b.Dy()/b.Dx())
	small := image.NewRGBA(image.Rect(0, 0, w, h))
	var tr, tg, tb, tn uint64
	for y := 0; y < h; y++ {
		y0, y1 := b.Min.Y+y*b.Dy()/h, b.Min.Y+(y+1)*b.Dy()/h
		for x := 0; x < w; x++ {
			x0, x1 := b.Min.X+x*b.Dx()/w, b.Min.X+(x+1)*b.Dx()/w
			var r, g, bl, n uint64
			// Sample at most a 4×4 grid per cell: averaging every pixel of
			// a large poster would cost more than the blur needs.
			sy, sx := max(1, (y1-y0)/4), max(1, (x1-x0)/4)
			for yy := y0; yy < max(y1, y0+1); yy += sy {
				for xx := x0; xx < max(x1, x0+1); xx += sx {
					cr, cg, cb, _ := img.At(xx, yy).RGBA()
					r, g, bl, n = r+uint64(cr>>8), g+uint64(cg>>8), bl+uint64(cb>>8), n+1
				}
			}
			small.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255})
			tr, tg, tb, tn = tr+r/n, tg+g/n, tb+bl/n, tn+1
		}
	}
	for i := 0; i < 3; i++ {
		boxBlur(small, 3)
	}
	return small, color.RGBA{uint8(tr / tn), uint8(tg / tn), uint8(tb / tn), 255}
}

// boxBlur blurs img in place, horizontally then vertically, with a box of
// 2r+1, clamping at the edges.
func boxBlur(img *image.RGBA, r int) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	tmp := make([]uint8, len(img.Pix))
	pass := func(src, dst []uint8, n, m int, at func(i, j int) int) {
		for j := 0; j < m; j++ {
			for i := 0; i < n; i++ {
				var s [3]int
				for k := -r; k <= r; k++ {
					p := at(min(max(i+k, 0), n-1), j)
					s[0], s[1], s[2] = s[0]+int(src[p]), s[1]+int(src[p+1]), s[2]+int(src[p+2])
				}
				p := at(i, j)
				d := 2*r + 1
				dst[p], dst[p+1], dst[p+2], dst[p+3] = uint8(s[0]/d), uint8(s[1]/d), uint8(s[2]/d), 255
			}
		}
	}
	stride := img.Stride
	pass(img.Pix, tmp, w, h, func(x, y int) int { return y*stride + 4*x })
	pass(tmp, img.Pix, h, w, func(y, x int) int { return y*stride + 4*x })
}
