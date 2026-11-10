package app

import "github.com/egoist/mygo/ui"

// Icons are original 24px drawings. Icon() paints them in the text color.
func svg(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">` + body + `</svg>`))
}

var (
	iconPlay         = svg(`<path d="M8 5.5v13l11-6.5z"/>`)
	iconPause        = svg(`<path d="M6 5h4v14H6zm8 0h4v14h-4z"/>`)
	iconPrev         = svg(`<path d="M6 6h2.2v12H6zm3.2 6 8.8 5.2V6.8z"/>`)
	iconNext         = svg(`<path d="M16 6h2.2v12H16zM6 6.8v10.4L14.8 12z"/>`)
	iconRew          = svg(`<path d="M11.5 7.2 6 12l5.5 4.8zm7 0L13 12l5.5 4.8z"/>`)
	iconFwd          = svg(`<path d="M5.5 7.2 11 12l-5.5 4.8zm7 0L18 12l-5.5 4.8z"/>`)
	iconVol          = svg(`<path d="M4 9h3.2L11 5.8v12.4L7.2 15H4zm9.2 1.2a3.2 3.2 0 0 1 0 3.6l1.3 1.1a4.8 4.8 0 0 0 0-5.8z"/>`)
	iconMute         = svg(`<path d="M4 9h3.2L11 5.8v12.4L7.2 15H4zm12.1.8 1.6 1.6-1.6 1.6 1.1 1.1 1.6-1.6 1.6 1.6 1.1-1.1-1.6-1.6 1.6-1.6-1.1-1.1-1.6 1.6-1.6-1.6z"/>`)
	iconFull         = svg(`<path d="M5 9V5h4v2H7v2zm10-4h4v4h-2V7h-2zm4 10v4h-4v-2h2v-2zM9 19H5v-4h2v2h2z"/>`)
	iconList         = svg(`<path d="M5 6h14v2H5zm0 5h14v2H5zm0 5h14v2H5z"/>`)
	iconHeart        = svg(`<path d="M12 19s-7-4.4-7-9a3.8 3.8 0 0 1 7-2 3.8 3.8 0 0 1 7 2c0 4.6-7 9-7 9z"/>`)
	iconBack         = svg(`<path d="M14.5 5.5 8 12l6.5 6.5-1.4 1.4L5.2 12 13.1 4.1z"/>`)
	iconSearch       = svg(`<path d="M10.5 4a6.5 6.5 0 1 1 0 13 6.5 6.5 0 0 1 0-13zm0 2a4.5 4.5 0 1 0 0 9 4.5 4.5 0 0 0 0-9zM15 15.6 19.2 20l-1.4 1.2L13.6 16.8z"/>`)
	iconHome         = svg(`<path d="M12 3.4 3.6 10.2V20c0 .6.5 1.1 1.1 1.1H9.2V14h5.6v7.1h4.5c.6 0 1.1-.5 1.1-1.1v-9.8z"/>`)
	iconClock        = svg(`<path d="M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zm.9 4.2V12l3.1 1.8-.8 1.4-4.1-2.4V7.2z"/>`)
	iconScreen       = svg(`<path d="M3.5 5.5h17a1 1 0 0 1 1 1v9.2a1 1 0 0 1-1 1h-17a1 1 0 0 1-1-1V6.5a1 1 0 0 1 1-1zM9 18.2h6V20H9z"/>`)
	iconSignal       = svg(`<path d="M12 16.4a1.5 1.5 0 1 0 .01 3 1.5 1.5 0 0 0-.01-3zM7.4 14.4a6 6 0 0 1 9.2 0l-1.4 1.3a4.2 4.2 0 0 0-6.4 0zm-2.5-2.5a9.2 9.2 0 0 1 14.2 0L17.7 13a7.4 7.4 0 0 0-11.4 0z"/>`)
	iconClose        = svg(`<path d="m6.7 5.3 5.3 5.3 5.3-5.3 1.4 1.4-5.3 5.3 5.3 5.3-1.4 1.4-5.3-5.3-5.3 5.3-1.4-1.4 5.3-5.3-5.3-5.3z"/>`)
	iconGear         = svg(`<path d="M19.4 13a7.7 7.7 0 0 0 0-2l2-1.6-2-3.4-2.4 1a7.7 7.7 0 0 0-1.7-1L14.9 3h-4l-.4 2.6a7.7 7.7 0 0 0-1.7 1l-2.4-1-2 3.4 2 1.6a7.7 7.7 0 0 0 0 2l-2 1.6 2 3.4 2.4-1a7.7 7.7 0 0 0 1.7 1l.4 2.6h4l.4-2.6a7.7 7.7 0 0 0 1.7-1l2.4 1 2-3.4zM12.9 15.2a3.2 3.2 0 1 1 0-6.4 3.2 3.2 0 0 1 0 6.4z"/>`)
	iconTrash        = svg(`<path d="M9 3h6l1 2h4v2H4V5h4zm-3 6h12l-.8 11H6.8zm4 2v7h2v-7zm4 0v7h2v-7z"/>`)
	iconChevronRight = svg(`<path d="m9.2 5.4 6.3 6.6-6.3 6.6-1.5-1.4 4.9-5.2-4.9-5.2z"/>`)
	iconRefresh      = svg(`<path d="M12 4a8 8 0 0 1 7.3 4.7l-1.9.8A6 6 0 1 0 18 12h2a8 8 0 1 1-8-8zm1 3v5l4 2-.8 1.6L11 13V7z"/>`)
	iconExternal     = svg(`<path d="M13 4h7v7h-2V7.4l-7.3 7.3-1.4-1.4L16.6 6H13zM5 6h5v2H6v10h10v-4h2v6H4V6z"/>`)
	iconUp           = svg(`<path d="m12 7 6 6-1.4 1.4L12 9.8l-4.6 4.6L6 13z"/>`)
	iconDown         = svg(`<path d="m12 17-6-6 1.4-1.4 4.6 4.6 4.6-4.6L18 11z"/>`)
)
