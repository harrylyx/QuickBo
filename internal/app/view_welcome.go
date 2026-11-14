package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The welcome screen's copy. Kept in one place so the screen and its tests
// read the same words, and no line drifts from the facts in README:
//
//	Voice: technical, exact, no verdicts about the reader.
//	Facts: no built-in sources, subscriptions or parse APIs; JSON, XML,
//	       part of JS rules and XBPQ read directly; JAR/DEX listed, never run.
//	Shape: one claim a screen ("load a configuration"), one limit admitted,
//	       two actions whose labels stay the same everywhere they appear.
//	Never: stock launch lines, invented titles or artwork, a caption under
//	       the empty viewfinder, a second wording for the same action.
const (
	welcomeWordmark   = "QuickBo"
	welcomeBrandTag   = "TVBox / FongMi 配置 · 桌面播放器"
	welcomeEyebrow    = "当前没有可用配置"
	welcomeTitle      = "从一份配置开始播放。"
	welcomeLead       = "QuickBo 不内置任何片源、订阅地址或解析接口。填入配置 URL，或打开一份本地 JSON，站点和直播才会列出来。"
	welcomeFact       = "JSON · XML · JS 规则 · XBPQ 可直接读取；JAR/DEX 会列出，但不执行 jar。"
	welcomeNativeFact = "原生 GPU 界面 · 不依赖 WebView"

	welcomeAdd     = "添加订阅"
	welcomeFile    = "打开本地文件"
	welcomeRetry   = "重试订阅"
	welcomeLoading = "正在读取配置…"
)

// One grid for the whole screen: the brand row, the claim and the facts
// strip all start and end on the same left and right edges. The right-hand
// frame is an empty viewfinder rather than a mock of a programme, because
// the app deliberately ships with no content of its own.
const (
	welcomeEdgeTop   float32 = 34
	welcomeEdgeSide  float32 = 64
	welcomeEdgeBot   float32 = 26
	welcomeCopyWidth float32 = 500
	welcomeStageH    float32 = 330
	welcomeTitleSize float32 = 46
	welcomeSlotH     float32 = 52
	welcomeStagger           = 50 * time.Millisecond
)

// Shared by every rebuilt frame: the four blocks enter head to foot, each
// a little later, without allocating an Animation per frame. Every single
// entrance stays under 400ms so the screen never feels slow to appear.
var welcomeEnterAnims = [4]*ui.Animation{
	{Duration: 300 * time.Millisecond, Ease: ui.EaseOut},
	{Duration: 330 * time.Millisecond, Ease: ui.EaseOut, Delay: welcomeStagger},
	{Duration: 360 * time.Millisecond, Ease: ui.EaseOut, Delay: 2 * welcomeStagger},
	{Duration: 390 * time.Millisecond, Ease: ui.EaseOut, Delay: 3 * welcomeStagger},
}

func (a *App) viewWelcome(c *ui.Context, th *ui.Theme) {
	// The ground is the root element itself: a separate absolute wash would
	// paint over the flow and swallow clicks on the buttons. The shift is
	// neutral, so dark and light both stay the app's own colors.
	ui.Box(c).Fill().LinearGradient(ui.LinearGradient{
		From:  th.Background,
		To:    th.Background.MixOklab(th.Surface, 0.55),
		Angle: 145,
	}).Children(func() {
		ui.Column(c).Fill().Padding(welcomeEdgeTop, welcomeEdgeSide, welcomeEdgeBot, welcomeEdgeSide).Children(func() {
			a.welcomeHeader(c, th)
			ui.Box(c).Grow(1)
			a.welcomeBody(c, th)
			ui.Box(c).Grow(1)
			a.welcomeFooter(c, th)
		})
	})
	a.welcomeEntered = true
}

// welcomeEnter plays the entrance once, and rests when the desktop asks
// for less motion. welcomeEntered keeps it off every later rebuilt frame.
func (a *App) welcomeEnter(c *ui.Context, step int) ui.ElementTransition {
	if c.Preferences().ReduceMotion {
		return ui.ElementTransition{Duration: -1}
	}
	t := ui.ElementTransition{Animation: welcomeEnterAnims[step]}
	if !a.welcomeEntered {
		t.Enter = &ui.Motion{Y: 12, Opacity: 0, Scale: 1}
	}
	return t
}

func (a *App) welcomeHeader(c *ui.Context, th *ui.Theme) {
	ui.Column(c.Key("welcome-head")).FillWidth().Gap(16).Transition(a.welcomeEnter(c, 0)).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			logoMark(c, th, 26)
			ui.Text(c, welcomeWordmark).FontSize(fontTitle).FontWeight(weightTitle).SingleLine()
			ui.Box(c).Grow(1)
			ui.Text(c, welcomeBrandTag).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
		})
		ui.Box(c).FillWidth().Height(1).Background(th.Border)
	})
}

func (a *App) welcomeBody(c *ui.Context, th *ui.Theme) {
	ui.Row(c).FillWidth().Gap(56).AlignItems(ui.Center).Children(func() {
		a.welcomeCopy(c, th)
		a.welcomeStage(c, th)
	})
}

func (a *App) welcomeCopy(c *ui.Context, th *ui.Theme) {
	ui.Column(c.Key("welcome-copy")).Width(welcomeCopyWidth).Shrink(1).Gap(18).Transition(a.welcomeEnter(c, 1)).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(6, 6).Radius(999).Background(accentInk(th))
			ui.Text(c, welcomeEyebrow).FontSize(fontCaption).FontWeight(weightStrong).TextColor(th.TextMuted).SingleLine()
		})
		ui.Text(c, welcomeTitle).FontSize(welcomeTitleSize).FontWeight(weightTitle).TextColor(th.Text).Wrap()
		ui.Text(c, welcomeLead).FontSize(fontTitle).TextColor(th.TextMuted.Alpha(0.9)).Wrap()
		ui.Row(c).Gap(10).Padding(6, 0, 0, 0).Children(func() {
			if primaryPill(c, th, "welcome-add", nil, welcomeAdd).Clicked() {
				a.openSubs()
			}
			if secondaryPill(c, th, "welcome-file", welcomeFile).Clicked() {
				a.pickFile()
			}
		})
		a.welcomeStates(c, th)
	})
}

// welcomeStates reserves its height even while nothing is happening, so a
// late load or a failed one cannot move the claim or the buttons.
func (a *App) welcomeStates(c *ui.Context, th *ui.Theme) {
	ui.Box(c).FillWidth().Height(welcomeSlotH).Center().Children(func() {
		switch {
		case a.loading:
			a.loadingRow(c, th, welcomeLoading)
		case a.loadErr != "":
			a.welcomeError(c, th)
		}
	})
}

// welcomeError is a compact line instead of a note card: the card would
// grow past the reserved slot and push the rest of the screen down.
func (a *App) welcomeError(c *ui.Context, th *ui.Theme) {
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Radius(999).Background(th.Danger)
		ui.Column(c).Grow(1).Shrink(1).Children(func() {
			ui.Text(c, a.loadErr).FontSize(fontBody).TextColor(th.Danger).SingleLine()
		})
		retry := ui.Row(c.Key("welcome-retry")).Height(44).Padding(0, 12).Radius(8).AlignItems(ui.Center).
			Cursor(ui.CursorPointer).OnClick(a.retrySubs).Label(welcomeRetry)
		fg := th.Danger
		if retry.Hovered() {
			fg = th.Danger.MixOklab(th.Text, 0.3)
		}
		retry.Children(func() {
			ui.Text(c, welcomeRetry).FontSize(fontBody).FontWeight(weightStrong).TextColor(fg).SingleLine()
		})
	})
}

func (a *App) welcomeStage(c *ui.Context, th *ui.Theme) {
	ui.Column(c.Key("welcome-stage")).Grow(1).Transition(a.welcomeEnter(c, 2)).Children(func() {
		frame := ui.Box(c).FillWidth().Height(welcomeStageH).Radius(18).Clip().
			Background(th.Surface).Border(1, th.Border)
		frame.Children(func() {
			ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).LinearGradient(ui.LinearGradient{
				From:  th.Surface,
				To:    th.Background.MixOklab(th.Accent, 0.045),
				Angle: 160,
			})
			// An inner frame and a thirding grid read as a viewfinder:
			// lines only, no pretend poster and no caption.
			ui.Box(c).Absolute().Left(20).Right(20).Top(20).Bottom(20).Border(1, th.Border)
			ui.Row(c).Absolute().Left(20).Right(20).Top(20).Bottom(20).Children(func() {
				ui.Box(c).Grow(1)
				ui.Box(c).Width(1).FillHeight().Background(th.Border)
				ui.Box(c).Grow(1)
				ui.Box(c).Width(1).FillHeight().Background(th.Border)
				ui.Box(c).Grow(1)
			})
			ui.Column(c).Absolute().Left(20).Right(20).Top(20).Bottom(20).Children(func() {
				ui.Box(c).Grow(1)
				ui.Box(c).Height(1).FillWidth().Background(th.Border)
				ui.Box(c).Grow(1)
				ui.Box(c).Height(1).FillWidth().Background(th.Border)
				ui.Box(c).Grow(1)
			})
			ui.Column(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Center().Children(func() {
				// A mark, not a control: an empty viewfinder has nothing
				// to play, so this must not read as a disabled button.
				// accentInk keeps it legible on both the near-black and the
				// white ground without adding a color to the theme.
				ink := accentInk(th)
				ui.Box(c).Size(84, 84).Radius(999).Center().
					Border(1, ink.Alpha(0.42)).Children(func() {
					ui.Icon(c, iconPlay).Size(22, 22).TextColor(ink)
				})
			})
		})
	})
}

func (a *App) welcomeFooter(c *ui.Context, th *ui.Theme) {
	ui.Column(c.Key("welcome-foot")).FillWidth().Gap(14).Transition(a.welcomeEnter(c, 3)).Children(func() {
		ui.Box(c).FillWidth().Height(1).Background(th.Border)
		ui.Row(c).Gap(20).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).Children(func() {
				ui.Text(c, welcomeFact).FontSize(fontCaption).TextColor(th.TextMuted.Alpha(0.92)).Wrap()
			})
			ui.Text(c, welcomeNativeFact).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
		})
	})
}
