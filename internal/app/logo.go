package app

import (
	_ "embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

// logoPNG is the app's icon without its margin, for the sidebar and the
// welcome screen. The packaged apps take theirs from icon/icon.png.
//
//go:embed logo.png
var logoPNG []byte

var (
	logoOnce sync.Once
	logoBmp  *ui.Bitmap
)

// logo returns the decoded logo, or nil if it does not decode.
func logo() *ui.Bitmap {
	logoOnce.Do(func() {
		if bmp, err := ui.DecodeBitmap(logoPNG); err == nil {
			logoBmp = bmp
		}
	})
	return logoBmp
}

// logoMark draws the logo at size, falling back to a plain accent square.
func logoMark(c *ui.Context, th *ui.Theme, size float32) {
	if bmp := logo(); bmp != nil {
		ui.Image(c, bmp).Size(size, size).Fit(ui.Contain).Label("QuickBo")
		return
	}
	ui.Box(c).Size(size, size).Radius(size * 0.22).Background(th.Accent)
}
