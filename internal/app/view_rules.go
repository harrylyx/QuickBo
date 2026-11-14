package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *App) viewRules(c *ui.Context, th *ui.Theme) {
	a.bindRuleEdits()
	a.applyRuleDrop()
	defer func() { a.ruleSettle = nil }()
	stopFrame := false
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Text(c, "按住左侧或右侧把手拖动可调整顺序。「最新影视」除按关键字匹配分类外，还会聚合各源首页候选，按多源共同收录、清晰度、年份排序。各行按关键字匹配片源分类名；改动先留在这里，点「保存」才写入设置并按本地缓存重算首页，不会重新联网采集。要更新首页候选和分类内容，点「重新采集」。").TextColor(th.TextMuted).Shrink(1).Wrap()
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
			ui.Column(c).Gap(ruleGap).Children(func() {
				for i := 0; i < len(a.homeRules); i++ {
					if !a.ruleRow(c, th, i) {
						return
					}
				}
			})
		})
	})
}

func (a *App) ruleRow(c *ui.Context, th *ui.Theme, i int) bool {
	if i < 0 || i >= len(a.homeRules) || i >= len(a.ruleNames) || i >= len(a.ruleWords) {
		return false
	}
	id := a.homeRules[i].ID
	keep := true
	held := false
	row := ui.Column(c.Key("rule:"+id)).FillWidth().Padding(10, 12).Radius(8).Background(th.Surface).Gap(6)
	if i == 0 {
		if b := row.LayoutBounds(); b.H > 0 {
			a.rulePitch = b.H + ruleGap
		}
	}
	row.Translate(0, a.ruleRowOffset(row, i, id))
	if a.ruleDragID == id {
		// Lifted: above its neighbours, with a deeper shadow.
		row.ZIndex(10).Background(th.SurfaceHover).Shadow(0, 14, 30, -6, ui.RGBA(0, 0, 0, 0.45))
	}
	row.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
			if a.ruleGrip(c, th, i, id, "l") {
				held = true
			}
			name := ui.TextInput(c.Key("rule-name:"+id), &a.ruleNames[i]).Placeholder("分类名称").Width(220)
			a.watchRule(name, i, i*2)
			ui.Column(c).Grow(1)
			if a.ruleIconBtn(c, th, "rule-drop:"+id, iconTrash, "删除", th.Danger).Clicked() {
				a.deleteRule(i)
				keep = false
			}
			if a.ruleGrip(c, th, i, id, "r") {
				held = true
			}
		})
		if !keep {
			return
		}
		words := ui.TextInput(c.Key("rule-words:"+id), &a.ruleWords[i]).Placeholder("关键字，用顿号或逗号分开").FillWidth()
		a.watchRule(words, i, i*2+1)
	})
	// Both handles report their own press. Drop only once neither is held,
	// or a drag on the right would end on the frame the left is idle.
	if a.ruleDragID == id && !held {
		a.ruleDropped = true
		c.Invalidate()
	}
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

// ruleGap is the space between home rows on the 分类 page.
const ruleGap = 8

// ruleSettle is a drop to settle in the frame after it: the dropped row
// glides from where it was let go into its new place, and the rows that
// made room for it stay where they are drawn.
type ruleSettle struct {
	id       string
	residual float32
}

// ruleGrip is one of a row's two drag handles (side "l" or "r"). Dragging
// either lifts the row and moves it with the pointer while the others
// slide aside. It reports whether the pointer still holds this handle;
// the row drops only when both have been let go. Its menu moves the row
// without dragging.
func (a *App) ruleGrip(c *ui.Context, th *ui.Theme, i int, id, side string) bool {
	grip := ui.Column(c.Key("rule-grip:"+side+":"+id)).Size(26, 30).Radius(6).Center().
		Cursor(ui.CursorGrab).Tooltip("拖动调整顺序，右键可移到最前或最后").Label("拖动调整顺序")
	fg := th.TextMuted
	switch {
	case a.ruleDragID == id:
		fg = th.Accent
		grip.Cursor(ui.CursorGrabbing)
	case grip.Hovered():
		grip.Background(th.SurfaceHover)
		fg = th.Text
	}
	grip.Children(func() {
		ui.Icon(c, iconGrip).Size(16, 16).TextColor(fg)
	})
	grip.ContextMenu(func(menu *ui.Menu) {
		last := len(a.homeRules) - 1
		if menu.Item("移到最前").Disabled(i == 0).Chosen() {
			a.moveRuleTo(i, 0)
		}
		if menu.Item("上移").Disabled(i == 0).Chosen() {
			a.moveRuleTo(i, i-1)
		}
		if menu.Item("下移").Disabled(i == last).Chosen() {
			a.moveRuleTo(i, i+1)
		}
		if menu.Item("移到最后").Disabled(i == last).Chosen() {
			a.moveRuleTo(i, last)
		}
	})
	// Dragged's third value says the pointer still holds the handle; Pressed
	// drops out for the frame the pointer moves in, so it cannot tell a
	// let-go.
	_, dy, held := grip.Dragged()
	if held && dy != 0 {
		if a.ruleDragID == "" {
			a.ruleDragID, a.ruleDragFrom, a.ruleDragDY = id, i, 0
		}
		if a.ruleDragID == id {
			a.ruleDragDY += dy
		}
	}
	return held
}

// ruleDragTarget is the index the dragged row would drop at now.
func (a *App) ruleDragTarget() int {
	p := a.rulePitch
	if p <= 0 {
		p = 96
	}
	to := a.ruleDragFrom + int(math.Round(float64(a.ruleDragDY/p)))
	return max(0, min(len(a.homeRules)-1, to))
}

// applyRuleDrop moves a dropped row into place and leaves the offsets to
// settle in this build (ruleRowOffset).
func (a *App) applyRuleDrop() {
	if !a.ruleDropped {
		return
	}
	from, to := a.ruleDragFrom, a.ruleDragTarget()
	a.ruleSettle = &ruleSettle{id: a.ruleDragID, residual: a.ruleDragDY - float32(to-from)*a.rulePitch}
	a.ruleDragID, a.ruleDragDY, a.ruleDropped = "", 0, false
	a.moveRuleTo(from, to)
}

// ruleRowOffset is how far row i is drawn from its place in the layout: the
// dragged row follows the pointer, the rows between its start and where it
// would drop slide one row over to make room, and every row springs back to
// its place once the drag ends.
func (a *App) ruleRowOffset(row ui.Element, i int, id string) float32 {
	if s := a.ruleSettle; s != nil {
		if id == s.id {
			row.Jump("ry", s.residual)
			return row.AnimateTo("ry", 0, ui.Snappy)
		}
		return row.Jump("ry", 0)
	}
	if a.ruleDragID == "" {
		return row.AnimateTo("ry", 0, ui.Snappy)
	}
	if id == a.ruleDragID {
		return row.Jump("ry", a.ruleDragDY)
	}
	from, to := a.ruleDragFrom, a.ruleDragTarget()
	var shift float32
	switch {
	case from < to && i > from && i <= to:
		shift = -a.rulePitch
	case to < from && i >= to && i < from:
		shift = a.rulePitch
	}
	return row.AnimateTo("ry", shift, ui.Snappy)
}
