package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/source"
)

// fakeUpdater swaps the update plugin's preferences for ones in memory,
// counting the checks asked for, until the returned function puts them back.
func fakeUpdater(checks, downloads *bool, checked *int, last time.Time) func() {
	check, getChecks, setChecks := checkForUpdates, updateChecks, setUpdateChecks
	getDownloads, setDownloads, lastFn := updateDownloads, setUpdateDownloads, lastUpdateCheck
	checkForUpdates = func() { *checked++ }
	updateChecks = func() bool { return *checks }
	setUpdateChecks = func(on bool) { *checks = on }
	updateDownloads = func() bool { return *downloads }
	setUpdateDownloads = func(on bool) { *downloads = on }
	lastUpdateCheck = func() time.Time { return last }
	return func() {
		checkForUpdates, updateChecks, setUpdateChecks = check, getChecks, setChecks
		updateDownloads, setUpdateDownloads, lastUpdateCheck = getDownloads, setDownloads, lastFn
	}
}

func openUpdateSettings(a *App) *ui.Tester {
	a.settingsPath = ""
	a.updates = true
	a.catalog = &source.Catalog{}
	a.screen, a.sideSel, a.lastSide = screenHome, "home", "home"
	a.openSettings()
	tt := ui.NewTester(a.View, 1200, 900)
	tt.Frame()
	return tt
}

// Turning off checking for updates turns it off in the updater, and
// installing updates without asking can't be turned on without it.
func TestTurnOffUpdateChecks(t *testing.T) {
	checks, downloads, checked := true, false, 0
	last := time.Date(2026, 3, 2, 15, 4, 0, 0, time.Local)
	defer fakeUpdater(&checks, &downloads, &checked, last)()

	a := New()
	tt := openUpdateSettings(a)
	if !tt.HasText("更新") || !tt.HasText("每天检查一次，上次检查 "+last.Format("2006-01-02 15:04")) {
		t.Fatalf("settings texts %q", tt.Texts())
	}
	if err := tt.Click("自动检查更新"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if checks {
		t.Fatal("checks for updates still")
	}
	if err := tt.Click("下载后下次启动安装"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if downloads {
		t.Fatal("turned on installing updates without checking for them")
	}

	checks = true
	tt.Frame()
	if err := tt.Click("下载后下次启动安装"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !downloads {
		t.Fatal("didn't turn on installing updates")
	}
}

// 检查更新 in the settings checks for updates as the user asked.
func TestCheckForUpdatesFromSettings(t *testing.T) {
	checks, downloads, checked := true, false, 0
	defer fakeUpdater(&checks, &downloads, &checked, time.Time{})()

	a := New()
	tt := openUpdateSettings(a)
	if !tt.HasText("每天检查一次") {
		t.Fatalf("missing the daily check note: %q", tt.Texts())
	}
	if tt.HasText("上次检查") {
		t.Fatalf("a check that never happened has a time: %q", tt.Texts())
	}
	if err := tt.Click("检查更新"); err != nil {
		t.Fatal(err)
	}
	if checked != 1 {
		t.Fatalf("checked %d times", checked)
	}
}
