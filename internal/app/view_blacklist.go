package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

// filteredBlacklist is the blacklist narrowed by the pane's search field,
// matching either the keyword or a source it hides.
func (a *App) filteredBlacklist() []string {
	q := strings.ToLower(strings.TrimSpace(a.blFilter))
	if q == "" {
		return a.blacklist
	}
	var out []string
	for _, kw := range a.blacklist {
		if strings.Contains(strings.ToLower(kw), q) ||
			slices.ContainsFunc(a.blacklistHits(kw), func(n string) bool { return strings.Contains(strings.ToLower(n), q) }) {
			out = append(out, kw)
		}
	}
	return out
}

func (a *App) selectedBlacklist() []string {
	var out []string
	for _, kw := range a.blacklist {
		if a.blSel[strings.ToLower(kw)] {
			out = append(out, kw)
		}
	}
	return out
}

// viewBlacklist manages blacklist keywords: add several at once, search,
// select and remove in bulk, and see which sources each keyword hides.
func (a *App) viewBlacklist(c *ui.Context, th *ui.Theme) {
	ui.Text(c, "名称含关键字的片源和直播会被隐藏。").TextColor(th.TextMuted).Wrap()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		field := ui.TextInput(c, &a.blacklistDraft).Placeholder("关键字，如「测试」「广告」").Label("黑名单关键字").Grow(1)
		add := ui.PrimaryButton(c, "加入黑名单").Disabled(strings.TrimSpace(a.blacklistDraft) == "").Clicked()
		if field.Submitted() || add {
			a.addBlacklist(a.blacklistDraft)
		}
	})
	if len(a.blacklist) == 0 {
		a.emptyState(c, th, "黑名单是空的", "在上面输入关键字，或在片源列表里批量选中片源后点「加黑名单」。")
		return
	}

	list := a.filteredBlacklist()
	all := ui.CheckOff
	selCount := 0
	for _, kw := range list {
		if a.blSel[strings.ToLower(kw)] {
			selCount++
		}
	}
	switch {
	case selCount == len(list) && len(list) > 0:
		all = ui.CheckOn
	case selCount > 0:
		all = ui.CheckMixed
	}
	stop := false
	ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
		ui.SearchField(c, &a.blFilter).Placeholder("搜索关键字或被隐藏的片源").Width(220).Label("搜索黑名单")
		if ui.CheckboxState(c, &all, "全选").Changed() {
			a.blSel = map[string]bool{}
			if all == ui.CheckOn {
				for _, kw := range list {
					a.blSel[strings.ToLower(kw)] = true
				}
			}
		}
		ui.Text(c, fmt.Sprintf("共 %d 个关键字，已选 %d 个", len(a.blacklist), selCount)).FontSize(fontCaption).TextColor(th.TextMuted)
		ui.Column(c).Grow(1)
		if ui.Button(c, "移出黑名单").Disabled(selCount == 0).Clicked() && selCount > 0 {
			a.removeBlacklists(a.selectedBlacklist())
			stop = true
		}
	})
	if stop {
		return
	}
	if len(list) == 0 {
		ui.Text(c, "没有符合搜索的关键字").TextColor(th.TextMuted)
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			for _, kw := range list {
				if a.blacklistRow(c, th, kw) {
					return
				}
			}
		})
	})
}

// blacklistRow renders one keyword with its checkbox and the sources it
// hides; it returns true when the keyword was removed.
func (a *App) blacklistRow(c *ui.Context, th *ui.Theme, kw string) bool {
	low := strings.ToLower(kw)
	hits := a.blacklistHits(kw)
	removed := false
	ui.Row(c.Key("bl:"+low)).Padding(10, 12).Radius(8).Background(th.Surface).Gap(10).AlignItems(ui.Center).Children(func() {
		checked := a.blSel[low]
		if ui.CheckboxBase(c, &checked).Label("选择 " + kw).Changed() {
			if a.blSel == nil {
				a.blSel = map[string]bool{}
			}
			if checked {
				a.blSel[low] = true
			} else {
				delete(a.blSel, low)
			}
		}
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, kw).FontSize(fontBody).FontWeight(weightStrong).SingleLine()
			note := "没有命中当前订阅里的片源"
			if n := len(hits); n > 0 {
				shown := hits
				if len(shown) > 3 {
					shown = shown[:3]
				}
				note = fmt.Sprintf("隐藏了 %d 个片源：%s", n, strings.Join(shown, "、"))
				if n > 3 {
					note += " 等"
				}
			}
			ui.Text(c, note).FontSize(fontCaption).TextColor(th.TextMuted).SingleLine()
		})
		a.iconBtn(c, "bl-drop:"+low, iconClose, "移出黑名单", 24, 12, th.TextMuted, func() {
			a.removeBlacklist(kw)
			removed = true
		})
	})
	return removed
}
