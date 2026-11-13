package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// manageTabs are the management page's tabs, in order.
var manageTabs = []string{screenSubs, screenSources, screenBlacklist, screenRules}

func isManage(screen string) bool {
	for _, s := range manageTabs {
		if s == screen {
			return true
		}
	}
	return false
}

// goManage opens the management page on tab (a manage screen); an unknown
// tab opens the subscriptions.
func (a *App) goManage(tab string) {
	if !isManage(tab) {
		tab = screenSubs
	}
	a.query = ""
	a.committed = ""
	a.sideSel, a.lastSide = "manage", "manage"
	a.screen = tab
	a.manageLast = tab
	a.closePlayer()
}

// viewManage is one page for everything that shapes the catalog:
// subscriptions, the sources they bring, the blacklist, and the home rows.
func (a *App) viewManage(c *ui.Context, th *ui.Theme) {
	labels := []string{
		fmt.Sprintf("订阅 %d", len(a.subs)),
		fmt.Sprintf("片源 %d", len(a.allSites)),
		fmt.Sprintf("黑名单 %d", len(a.blacklist)),
		fmt.Sprintf("首页分类 %d", len(a.activeRules())),
	}
	sel := 0
	for i, s := range manageTabs {
		if s == a.screen {
			sel = i
		}
	}
	ui.Tabs(c, &sel, labels...).Label("管理")
	if manageTabs[sel] != a.screen {
		a.goManage(manageTabs[sel])
	}
	switch a.screen {
	case screenSubs:
		ui.Column(c).Grow(1).Gap(14).Children(func() {
			a.subsPanel(c, th, true)
		})
	case screenSources:
		a.viewSources(c, th)
	case screenBlacklist:
		ui.Column(c).Grow(1).Gap(16).Children(func() {
			a.viewBlacklist(c, th)
		})
	case screenRules:
		a.viewRules(c, th)
	}
}
