package app

import "github.com/egoist/mygo/ui"

// Themes are original. The structure follows mygo's own theme object
// (Background, Surface, Accent, Radius, Spacing): a dark cinema palette
// by default, and a warm paper palette when light mode is on.
func cinemaDark() *ui.Theme {
	t := *ui.DarkTheme()
	t.Background = ui.Hex("#09090b")
	t.Surface = ui.Hex("#16161a")
	t.SurfaceHover = ui.Hex("#222228")
	t.SurfacePressed = ui.Hex("#2a2a32")
	t.Border = ui.RGBA(255, 255, 255, 0.08)
	t.Text = ui.Hex("#f4f4f5")
	t.TextMuted = ui.Hex("#a1a1aa")
	t.Accent = ui.Hex("#e4c295")
	t.AccentHover = ui.Hex("#f0d4b0")
	t.AccentPressed = ui.Hex("#c9a36e")
	t.AccentText = ui.Hex("#1c140c")
	t.InactiveAccent = t.Accent
	t.Danger = ui.Hex("#f07178")
	t.Warning = ui.Hex("#e8b86d")
	t.Success = ui.Hex("#7dcea0")
	t.Selection = ui.RGBA(228, 194, 149, 0.28)
	t.Focus = ui.RGBA(228, 194, 149, 0.7)
	t.FocusRingWidth = 2
	t.FocusRingOffset = 2
	t.Radius = 10
	t.Spacing = 4
	t.FontSize = 14
	t.Inverse = ui.Hex("#f4f4f5")
	t.InverseText = ui.Hex("#18181b")
	return &t
}

func cinemaLight() *ui.Theme {
	t := *ui.LightTheme()
	t.Background = ui.Hex("#f6f3ee")
	t.Surface = ui.Hex("#fffcf8")
	t.SurfaceHover = ui.Hex("#f0ebe3")
	t.SurfacePressed = ui.Hex("#e6dfd4")
	t.Border = ui.RGBA(28, 25, 23, 0.1)
	t.Text = ui.Hex("#1c1917")
	t.TextMuted = ui.Hex("#78716c")
	t.Accent = ui.Hex("#8c6239")
	t.AccentHover = ui.Hex("#6f4b2a")
	t.AccentPressed = ui.Hex("#5c3d22")
	t.AccentText = ui.Hex("#fffaf3")
	t.InactiveAccent = t.Accent
	t.Danger = ui.Hex("#b42318")
	t.Radius = 10
	t.Spacing = 4
	t.FontSize = 14
	return &t
}

func (a *App) useTheme(c *ui.Context) *ui.Theme {
	t := cinemaDark()
	if a.light {
		t = cinemaLight()
	}
	c.SetTheme(t)
	return t
}

func sidebarInk(light bool) ui.Color {
	if light {
		return ui.Hex("#efeae3")
	}
	return ui.Hex("#101013")
}

func stageInk(light bool) ui.Color {
	if light {
		return ui.Hex("#1c1917")
	}
	return ui.Hex("#050506")
}
