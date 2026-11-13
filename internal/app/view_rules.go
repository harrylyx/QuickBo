package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *App) viewRules(c *ui.Context, th *ui.Theme) {
	a.bindRuleEdits()
	stopFrame := false
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Text(c, "「最新影视」除按关键字匹配分类外，还会聚合各源首页候选，按多源共同收录、清晰度、年份排序。各行按关键字匹配片源分类名；改动先留在这里，点「保存」才写入设置并按本地缓存重算首页，不会重新联网采集。要更新首页候选和分类内容，点「重新采集」。").TextColor(th.TextMuted).Shrink(1).Wrap()
				ui.Column(c).Grow(1)
				if a.rulesDirty {
					ui.Text(c, "有未保存的修改").TextColor(th.Accent).SingleLine()
				}
				if ui.Button(c, "添加").Clicked() {
					a.addRule()
					stopFrame = true
					return
				}
				if ui.Button(c, "导入").Tooltip("从 JSON 文件导入分类，替换当前列表（保存后生效）").Clicked() {
					a.importRulesDialog()
				}
				if ui.Button(c, "导出").Tooltip("把当前分类导出为 JSON 文件").Clicked() {
					a.exportRulesDialog()
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
			if a.rulesNote != "" {
				ui.Text(c, a.rulesNote).FontSize(fontCaption).TextColor(th.TextMuted).Wrap()
			}
			if stopFrame {
				// The row set or the screen changed mid-frame; let the next
				// frame rebuild with fresh draft pointers.
				return
			}
			if len(a.homeRules) == 0 {
				a.emptyState(c, th, "没有分类", "添加一个分类，写上名称和关键字。",
					stateAction{label: "添加分类", primary: true, click: a.addRule})
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
	ui.Column(c.Key("rule:"+id)).FillWidth().Padding(10, 12).Radius(8).Background(th.Surface).Gap(6).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
			name := ui.TextInput(c.Key("rule-name:"+id), &a.ruleNames[i]).Placeholder("分类名称").Width(220)
			a.watchRule(name, i, i*2)
			ui.Column(c).Grow(1)
			if a.ruleIconBtn(c, th, "rule-up:"+id, iconUp, "上移", th.TextMuted).Clicked() {
				a.moveRule(i, -1)
				keep = false
				return
			}
			if a.ruleIconBtn(c, th, "rule-down:"+id, iconDown, "下移", th.TextMuted).Clicked() {
				a.moveRule(i, 1)
				keep = false
				return
			}
			if a.ruleIconBtn(c, th, "rule-drop:"+id, iconTrash, "删除", th.Danger).Clicked() {
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

func (a *App) ruleIconBtn(c *ui.Context, th *ui.Theme, key string, glyph *ui.SVG, tip string, fg ui.Color) ui.Element {
	b := ui.Column(c.Key(key)).Size(30, 30).Radius(15).Center().Cursor(ui.CursorPointer).Tooltip(tip).Label(tip)
	switch {
	case b.Pressed():
		b.Background(fg.Alpha(0.22))
	case b.Hovered():
		b.Background(fg.Alpha(0.12))
	}
	b.Transition(ui.ElementTransition{Duration: 150 * time.Millisecond}).Children(func() {
		ui.Icon(c, glyph).Size(16, 16).TextColor(fg)
	})
	return b
}

// TextInput edits the draft in place. Nothing is committed until 保存, so
// typing never triggers a home rebuild or a settings write.
func (a *App) watchRule(field ui.Element, i, focus int) {
	if focus < len(a.ruleFocus) {
		a.ruleFocus[focus] = field.Focused()
	}
}
