package app

import "github.com/egoist/mygo/ui"

// One type scale for the whole shell; the welcome wordmark is the only
// deliberate exception at 60px.
const (
	fontCaption  = 12
	fontBody     = 14
	fontTitle    = 16
	fontHeadline = 20
	fontHero     = 28
)

// Weights for the type scale: titles carry the hierarchy with weight more
// than size, as Apple's apps do; body text stays regular.
const (
	weightHeavy  = 800 // hero titles
	weightTitle  = 700 // section and page titles
	weightStrong = 600 // card titles, chips, buttons
)

// selectionColors is a choice's fill and text. The chosen one is tinted
// with the accent instead of filled with it, so the one filled control on a
// screen stays its primary action (播放).
func selectionColors(th *ui.Theme, on, hovered bool) (bg, fg ui.Color) {
	if on {
		return th.Accent.Alpha(0.2), accentInk(th)
	}
	bg, fg = th.Selection, th.Text
	if hovered {
		bg = th.SurfaceHover
	}
	return bg, fg
}

// accentInk is the accent as text on a tinted fill: lighter in the dark,
// darker in the light, so it reads at 4.5:1 or better.
func accentInk(th *ui.Theme) ui.Color {
	if th.Dark {
		return th.Accent.MixOklab(ui.Hex("#ffffff"), 0.45)
	}
	return th.Accent.MixOklab(ui.Hex("#000000"), 0.2)
}

// faint is the weakest text: placeholders, separators, counts.
func faint(th *ui.Theme) ui.Color {
	if th.Dark {
		return ui.RGBA(235, 235, 245, 0.32)
	}
	return ui.RGBA(60, 60, 67, 0.34)
}

func cardShadow(light, hot bool) (y, blur float32, ink ui.Color) {
	alpha := float32(0.28)
	if hot {
		alpha = 0.5
	}
	if light {
		alpha *= 0.4
	}
	if hot {
		return 14, 28, ui.RGBA(0, 0, 0, alpha)
	}
	return 6, 16, ui.RGBA(0, 0, 0, alpha)
}

// accentOf sets the theme's accent from one color, deriving hover and press
// in Oklab so they keep its hue: darker in the light, lighter on hover in
// the dark.
func accentOf(t *ui.Theme, accent ui.Color) {
	black, white := ui.Hex("#000000"), ui.Hex("#ffffff")
	t.Accent, t.AccentText = accent, white
	t.AccentHover, t.AccentPressed = accent.MixOklab(black, 0.1), accent.MixOklab(black, 0.2)
	if t.Dark {
		t.AccentHover, t.AccentPressed = accent.MixOklab(white, 0.12), accent.MixOklab(black, 0.15)
	}
	t.InactiveAccent = accent
	t.Focus = accent.Alpha(0.9)
}

// Themes follow mygo's theme object. Dark mode is a cinema: a near-black
// field, translucent whites for everything layered on it (so it sits as
// well on a blurred poster as on the page), and one rose accent for the
// primary action, the chosen item and progress.
func cinemaDark() *ui.Theme {
	t := *ui.DarkTheme()
	t.Background = ui.Hex("#0F0F12")
	t.Surface = ui.Hex("#18181C")
	t.SurfaceHover = ui.RGBA(255, 255, 255, 0.1)
	t.SurfacePressed = ui.RGBA(255, 255, 255, 0.14)
	t.Border = ui.RGBA(255, 255, 255, 0.08)
	t.Text = ui.Hex("#F5F5F7")
	t.TextMuted = ui.RGBA(235, 235, 245, 0.6)
	accentOf(&t, ui.Oklch(0.64, 0.21, 16))
	t.Danger = ui.Oklch(0.7, 0.17, 25)
	t.Warning = ui.Oklch(0.82, 0.15, 80)
	t.Success = ui.Oklch(0.78, 0.15, 160)
	t.Selection = ui.RGBA(255, 255, 255, 0.06)
	t.FocusRingWidth = 2
	t.FocusRingOffset = 2
	t.Radius = 10
	t.Spacing = 4
	t.FontSize = fontBody
	t.Inverse = ui.Hex("#F5F5F7")
	t.InverseText = ui.Hex("#0F0F12")
	return &t
}

// Light mode keeps the hue, its accent darkened so white text on it stays
// above 4.5:1.
func cinemaLight() *ui.Theme {
	t := *ui.LightTheme()
	t.Background = ui.Hex("#FFFFFF")
	t.Surface = ui.Hex("#F5F5F7")
	t.SurfaceHover = ui.RGBA(0, 0, 0, 0.06)
	t.SurfacePressed = ui.RGBA(0, 0, 0, 0.1)
	t.Border = ui.RGBA(0, 0, 0, 0.08)
	t.Text = ui.Hex("#1D1D1F")
	t.TextMuted = ui.RGBA(60, 60, 67, 0.64)
	accentOf(&t, ui.Oklch(0.56, 0.21, 16))
	t.Danger = ui.Oklch(0.55, 0.2, 27)
	t.Warning = ui.Oklch(0.6, 0.14, 65)
	t.Success = ui.Oklch(0.55, 0.13, 160)
	t.Selection = ui.RGBA(0, 0, 0, 0.04)
	t.FocusRingWidth = 2
	t.FocusRingOffset = 2
	t.Radius = 10
	t.Spacing = 4
	t.FontSize = fontBody
	t.Inverse = ui.Hex("#1D1D1F")
	t.InverseText = ui.Hex("#FFFFFF")
	return &t
}

// sidebarTheme is the theme inside the sidebar. mygo tints sidebar icons
// with the accent, as AppKit does; the app's rose made every icon shout,
// so the sidebar gets a quiet, desaturated one of the same hue, and its
// list no background of its own (the sidebar column's shows).
func sidebarTheme(th *ui.Theme, bg ui.Color) *ui.Theme {
	t := *th
	if t.Dark {
		accentOf(&t, ui.Oklch(0.58, 0.08, 16))
	} else {
		accentOf(&t, ui.Oklch(0.5, 0.09, 16))
	}
	t.Surface = bg
	return &t
}

// Appearances the settings offer.
const (
	appearanceSystem = "system"
	appearanceLight  = "light"
	appearanceDark   = "dark"
)

func validAppearance(s string) bool {
	return s == appearanceSystem || s == appearanceLight || s == appearanceDark
}

// resolveLight tells whether the chosen appearance shows light, given
// whether the system is dark. No choice yet follows the system.
func resolveLight(appearance string, systemDark bool) bool {
	switch appearance {
	case appearanceLight:
		return true
	case appearanceDark:
		return false
	}
	return !systemDark
}

func (a *App) useTheme(c *ui.Context) *ui.Theme {
	// Before SetTheme, the frame's theme is the toolkit's default, which
	// follows the system's appearance (and its changes).
	a.light = resolveLight(a.appearance, c.Theme().Dark)
	t := cinemaDark()
	if a.light {
		t = cinemaLight()
	}
	c.SetTheme(t)
	return t
}

func sidebarInk(light bool) ui.Color {
	if light {
		return ui.Hex("#F5F5F7")
	}
	return ui.Hex("#16161A")
}

func stageInk(light bool) ui.Color {
	if light {
		return ui.Hex("#1c1917")
	}
	return ui.Hex("#050506")
}
