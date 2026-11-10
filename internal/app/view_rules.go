package app

import (
	"github.com/egoist/mygo/ui"
)

func (a *App) viewRules(c *ui.Context, th *ui.Theme) {
	a.bindRuleEdits()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Text(c, "名称是首页上的行标题。关键字对应资源里已经分好的分类，分类名里带这些字就会投票进这一行。改完后按回车，或点到别处。新关键字作用在已经读到的分类上，要带上还没读过的分类，重新加载配置。").TextColor(th.TextMuted).Shrink(1).Wrap()
				ui.Column(c).Grow(1)
				if ui.Button(c, "添加").Clicked() {
					a.addRule()
					return
				}
				if ui.Button(c, "恢复默认").Clicked() {
					a.resetRules()
					return
				}
			})
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

func (a *App) watchRule(field ui.Element, i, focus int) {
	focused := field.Focused()
	if field.Submitted() || (focus < len(a.ruleFocus) && a.ruleFocus[focus] && !focused) {
		a.commitRule(i)
	}
	if focus < len(a.ruleFocus) {
		a.ruleFocus[focus] = focused
	}
}
