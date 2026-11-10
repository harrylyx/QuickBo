package app

import (
	"github.com/egoist/mygo/ui"
)

func (a *App) viewRules(c *ui.Context, th *ui.Theme) {
	a.bindRuleEdits()
	stopFrame := false
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Text(c, "名称是首页上的行标题，关键字匹配片源已经分好的分类名。改动先留在这里，点「保存」才写入设置并按本地缓存重算首页，不会重新联网采集。要更新片源内容，点「重新采集」。").TextColor(th.TextMuted).Shrink(1).Wrap()
				ui.Column(c).Grow(1)
				if a.rulesDirty {
					ui.Text(c, "有未保存的修改").TextColor(th.Accent).SingleLine()
				}
				if ui.Button(c, "添加").Clicked() {
					a.addRule()
					stopFrame = true
					return
				}
				if ui.Button(c, "恢复默认").Clicked() {
					a.resetRules()
					stopFrame = true
					return
				}
				if ui.Button(c, "重新采集").Clicked() {
					a.recollect()
					stopFrame = true
					return
				}
				if ui.PrimaryButton(c, "保存").Clicked() {
					a.saveRules()
					stopFrame = true
					return
				}
			})
			if stopFrame {
				// The row set or the screen changed mid-frame; let the next
				// frame rebuild with fresh draft pointers.
				return
			}
			if len(a.homeRules) == 0 {
				a.emptyState(c, th, "没有分类", "添加一个分类，写上名称和关键字。")
				return
			}
			for i := 0; i < len(a.homeRules); i++ {
				if !a.ruleRow(c, th, i) {
					return
				}
			}
		})
	})
}

func (a *App) ruleRow(c *ui.Context, th *ui.Theme, i int) bool {
	if i < 0 || i >= len(a.homeRules) || i >= len(a.ruleNames) || i >= len(a.ruleWords) {
		return false
	}
	id := a.homeRules[i].ID
	keep := true
	ui.Column(c.Key("rule:"+id)).FillWidth().Padding(12, 14).Radius(8).Background(th.Surface).Gap(8).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			name := ui.TextInput(c.Key("rule-name:"+id), &a.ruleNames[i]).Placeholder("分类名称").Width(220)
			a.watchRule(name, i, i*2)
			ui.Column(c).Grow(1)
			if ui.Button(c.Key("rule-up:"+id), "上移").Clicked() {
				a.moveRule(i, -1)
				keep = false
				return
			}
			if ui.Button(c.Key("rule-down:"+id), "下移").Clicked() {
				a.moveRule(i, 1)
				keep = false
				return
			}
			if ui.Button(c.Key("rule-drop:"+id), "删除").Clicked() {
				a.deleteRule(i)
				keep = false
			}
		})
		if !keep {
			return
		}
		words := ui.TextInput(c.Key("rule-words:"+id), &a.ruleWords[i]).Placeholder("关键字，用顿号或逗号分开").FillWidth()
		a.watchRule(words, i, i*2+1)
	})
	return keep
}

// TextInput edits the draft in place. Nothing is committed until 保存, so
// typing never triggers a home rebuild or a settings write.
func (a *App) watchRule(field ui.Element, i, focus int) {
	if focus < len(a.ruleFocus) {
		a.ruleFocus[focus] = field.Focused()
	}
}
