package app

import "github.com/egoist/mygo/ui"

// Icons are original 24px drawings. Icon() paints them in the text color.
func svg(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">` + body + `</svg>`))
}

var (
	iconPlay   = svg(`<path d="M8 5.5v13l11-6.5z"/>`)
	iconPause  = svg(`<path d="M6 5h4v14H6zm8 0h4v14h-4z"/>`)
	iconPrev   = svg(`<path d="M6 6h2.2v12H6zm3.2 6 8.8 5.2V6.8z"/>`)
	iconNext   = svg(`<path d="M16 6h2.2v12H16zM6 6.8v10.4L14.8 12z"/>`)
	iconRew    = svg(`<path d="M11.5 7.2 6 12l5.5 4.8zm7 0L13 12l5.5 4.8z"/>`)
	iconFwd    = svg(`<path d="M5.5 7.2 11 12l-5.5 4.8zm7 0L18 12l-5.5 4.8z"/>`)
	iconVol    = svg(`<path d="M4 9h3.2L11 5.8v12.4L7.2 15H4zm9.2 1.2a3.2 3.2 0 0 1 0 3.6l1.3 1.1a4.8 4.8 0 0 0 0-5.8z"/>`)
	iconMute   = svg(`<path d="M4 9h3.2L11 5.8v12.4L7.2 15H4zm12.1.8 1.6 1.6-1.6 1.6 1.1 1.1 1.6-1.6 1.6 1.6 1.1-1.1-1.6-1.6 1.6-1.6-1.1-1.1-1.6 1.6-1.6-1.6z"/>`)
	iconFull   = svg(`<path d="M5 9V5h4v2H7v2zm10-4h4v4h-2V7h-2zm4 10v4h-4v-2h2v-2zM9 19H5v-4h2v2h2z"/>`)
	iconList   = svg(`<path d="M5 6h14v2H5zm0 5h14v2H5zm0 5h14v2H5z"/>`)
	iconHeart  = svg(`<path d="M12 19s-7-4.4-7-9a3.8 3.8 0 0 1 7-2 3.8 3.8 0 0 1 7 2c0 4.6-7 9-7 9z"/>`)
	iconBack   = svg(`<path d="M14.5 5.5 8 12l6.5 6.5-1.4 1.4L5.2 12 13.1 4.1z"/>`)
	iconSearch = svg(`<path d="M10.5 4a6.5 6.5 0 1 1 0 13 6.5 6.5 0 0 1 0-13zm0 2a4.5 4.5 0 1 0 0 9 4.5 4.5 0 0 0 0-9zM15 15.6 19.2 20l-1.4 1.2L13.6 16.8z"/>`)
)
