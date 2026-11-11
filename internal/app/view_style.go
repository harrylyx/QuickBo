package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Motion shared by the controls: colors ease in 120ms, presses give with a
// short spring, and artwork fades in over 320ms once it loads.
var (
	colorEase = ui.Animation{Duration: 120 * time.Millisecond}
	artFade   = ui.Animation{Duration: 320 * time.Millisecond}
)

// pressPose makes an element give a little while pressed, springing back.
func pressPose(e ui.Element, scale float32) {
	pose := ui.Pose{}
	if e.Pressed() {
		pose.Scale = scale
	}
	e.AnimatePose(pose, ui.Snappy)
}

// fadeIn returns how far el's artwork has faded in: over artFade once it
// loads while el shows, at once if it was ready as el first showed.
func fadeIn(el ui.Element, ready bool) float32 {
	if !ready {
		return el.Jump("art", 0)
	}
	return el.AnimateTo("art", 1, artFade)
}

// posterBackdrop fills its (positioned) parent with pic's blurred copy,
// tinted with the poster's own color and fading into the page toward the
// bottom, or with a quiet gradient while the poster loads. key keeps one
// fade per poster.
func (a *App) posterBackdrop(c *ui.Context, th *ui.Theme, key, pic string, strength float32) {
	a.poster(pic) // starts loading it if needed
	art, ok := a.artOf(pic)
	bg := th.Background
	box := ui.Box(c.Key("backdrop:" + key + ":" + pic)).Absolute().Left(0).Right(0).Top(0).Bottom(0)
	in := fadeIn(box, ok)
	box.Children(func() {
		if ok && in > 0 {
			ui.Image(c, art.backdrop).Absolute().Left(0).Right(0).Top(0).Bottom(0).Fit(ui.Cover).Opacity(in * strength)
			// The poster's color, deepened, keeps the page tinted where the
			// scrim darkens it: a grey wash would kill the color.
			deep := art.tint.MixOklab(bg, 0.55)
			ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Background(deep.Alpha(0.45 * in))
		}
		ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 180,
			Stops: []ui.GradientStop{
				{Color: bg.Alpha(0.15), At: 0},
				{Color: bg.Alpha(0.55), At: 0.45},
				{Color: bg.Alpha(0.92), At: 0.8},
				{Color: bg, At: 1},
			},
		})
	})
}

// detailBackdrop is the detail page's posterBackdrop.
func (a *App) detailBackdrop(c *ui.Context, th *ui.Theme) {
	pic := ""
	if a.detail != nil {
		pic = a.detail.Pic
	}
	if pic == "" {
		return
	}
	a.posterBackdrop(c, th, "detail", pic, 1)
}

// primaryPill is the screen's one filled action: a tall rounded button with
// an icon, which brightens under the pointer and gives as it is pressed.
func primaryPill(c *ui.Context, th *ui.Theme, key string, icon *ui.SVG, label string) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Height(44).PaddingX(22).Gap(8).Radius(999).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label(label)
	fill := th.Accent
	switch {
	case b.Pressed():
		fill = th.AccentPressed
	case b.Hovered():
		fill = th.AccentHover
	}
	b.Background(b.AnimateColor("bg", fill, colorEase)).TextColor(th.AccentText).
		Shadow(0, 8, 20, -6, th.Accent.Alpha(0.6))
	pressPose(b, 0.96)
	b.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(14, 14)
		}
		ui.Text(c, label).FontSize(15).FontWeight(weightStrong).SingleLine()
	})
	return b
}

// glassButton is a round secondary action that sits on artwork: a
// translucent disc that lightens under the pointer.
func glassButton(c *ui.Context, th *ui.Theme, key string, icon *ui.SVG, tip string, size float32, fg ui.Color, click func()) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Size(size, size).Radius(999).Center().Cursor(ui.CursorPointer).Tooltip(tip).Label(tip).OnClick(click)
	fill := th.SurfaceHover
	if b.Hovered() {
		fill = th.SurfacePressed
	}
	b.Background(b.AnimateColor("bg", fill, colorEase)).Border(1, th.Border)
	pressPose(b, 0.9)
	b.Children(func() {
		ui.Icon(c, icon).Size(size*0.42, size*0.42).TextColor(fg)
	})
	return b
}

// metaPill is a quiet label for a fact (year, area, genre).
func metaPill(c *ui.Context, th *ui.Theme, text string, accent bool) {
	bg, fg := th.Selection, th.TextMuted
	if accent {
		bg, fg = th.Accent.Alpha(0.18), accentInk(th)
	}
	ui.Row(c).Padding(4, 10).Radius(999).Background(bg).Border(1, th.Border).Children(func() {
		ui.Text(c, text).FontSize(fontCaption).FontWeight(weightStrong).TextColor(fg).SingleLine()
	})
}
