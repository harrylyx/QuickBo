package app

import "github.com/egoist/mygo/ui"

// Themes follow mygo's theme object. Dark mode is an OLED cinema field:
// near-black surfaces, neutral selection, and a single play-red accent
// reserved for the primary action, the active mark, and progress.
// Light mode keeps the same accent, darkened so white label text stays
// above 4.5:1.
func cinemaDark() *ui.Theme {
	t := *ui.DarkTheme()
	t.Background = ui.Hex("#0A0A0A")
	t.Surface = ui.Hex("#141414")
	t.SurfaceHover = ui.Hex("#1C1C1C")
	t.SurfacePressed = ui.Hex("#262626")
	t.Border = ui.RGBA(255, 255, 255, 0.10)
	t.Text = ui.Hex("#F5F5F5")
	t.TextMuted = ui.Hex("#A1A1AA")
	t.Accent = ui.Hex("#E11D48")
	t.AccentHover = ui.Hex("#BE123C")
	t.AccentPressed = ui.Hex("#9F1239")
	t.AccentText = ui.Hex("#FFFFFF")
	t.InactiveAccent = t.Accent
	t.Danger = ui.Hex("#F87171")
	t.Warning = ui.Hex("#FBBF24")
	t.Success = ui.Hex("#34D399")
	t.Selection = ui.RGBA(255, 255, 255, 0.08)
	t.Focus = ui.RGBA(225, 29, 72, 0.90)
	t.FocusRingWidth = 2
	t.FocusRingOffset = 2
	t.Radius = 8
	t.Spacing = 4
	t.FontSize = 15
	t.Inverse = ui.Hex("#F5F5F5")
	t.InverseText = ui.Hex("#0A0A0A")
	return &t
}

func cinemaLight() *ui.Theme {
	t := *ui.LightTheme()
	t.Background = ui.Hex("#F4F4F5")
	t.Surface = ui.Hex("#FFFFFF")
	t.SurfaceHover = ui.Hex("#E7E7EA")
	t.SurfacePressed = ui.Hex("#DCDCE1")
	t.Border = ui.RGBA(24, 24, 27, 0.10)
	t.Text = ui.Hex("#18181B")
	t.TextMuted = ui.Hex("#52525B")
	t.Accent = ui.Hex("#BE123C")
	t.AccentHover = ui.Hex("#9F1239")
	t.AccentPressed = ui.Hex("#881337")
	t.AccentText = ui.Hex("#FFFFFF")
	t.InactiveAccent = t.Accent
	t.Danger = ui.Hex("#B42318")
	t.Warning = ui.Hex("#B45309")
	t.Success = ui.Hex("#047857")
	t.Selection = ui.RGBA(24, 24, 27, 0.06)
	t.Focus = ui.RGBA(190, 18, 60, 0.85)
	t.FocusRingWidth = 2
	t.FocusRingOffset = 2
	t.Radius = 8
	t.Spacing = 4
	t.FontSize = 15
	t.Inverse = ui.Hex("#18181B")
	t.InverseText = ui.Hex("#FAFAFA")
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
		return ui.Hex("#FFFFFF")
	}
	return ui.Hex("#050505")
}

func stageInk(light bool) ui.Color {
	if light {
		return ui.Hex("#1c1917")
	}
	return ui.Hex("#050506")
}
