package app

import (
	"testing"

	"github.com/harrylyx/QuickBo/internal/source"
)

// A source that failed its probe is listed in the management view (allSites)
// but not in the catalog; removing it must still work.
func TestRemoveSitesDropsAFailedSource(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.allSites = []source.Source{cmsFake{}, otherFake{}}
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}}} // "d" failed
	a.removeSites([]string{"d"})
	if !a.hidden[sourceHideID(otherFake{})] {
		t.Fatal("failed source was not removed")
	}
	for _, s := range a.visibleSources() {
		if s.Meta().Key == "d" {
			t.Fatal("removed source is still listed")
		}
	}
	if len(a.catalog.Sites) != 1 {
		t.Fatalf("working source was touched: %d", len(a.catalog.Sites))
	}
}

// Probe results arriving during the first load must not pull the user back
// home after they opened a title (sideSel stays "home" on the detail page).
func TestLoadingDoesNotLeaveAnOpenedTitle(t *testing.T) {
	a := New()
	a.sideSel = "home"
	for _, sc := range []string{screenHome, screenEmpty, screenIndex, screenWelcome} {
		a.screen = sc
		if !a.homeAwaitsSources() {
			t.Fatalf("%s should switch to home as sources arrive", sc)
		}
	}
	for _, sc := range []string{screenDetail, screenPlay, screenBrowse, screenSources} {
		a.screen = sc
		if a.homeAwaitsSources() {
			t.Fatalf("a probe result would yank %s back home", sc)
		}
	}
}

// Deleting a source from the sidebar keeps the current screen unless it was
// that source's.
func TestDropSiteKeepsTheCurrentScreen(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.catalog = &source.Catalog{Sites: []source.Source{cmsFake{}, otherFake{}}}
	a.screen, a.sideSel = screenSources, "sources"
	a.dropSite("d")
	if a.screen != screenSources || a.sideSel != "sources" {
		t.Fatalf("screen %s / %s after deleting another source", a.screen, a.sideSel)
	}
	a.screen, a.sideSel, a.siteKey = screenBrowse, "site:c", "c"
	a.dropSite("c")
	if a.screen != screenHome {
		t.Fatalf("deleting the open source left screen %s", a.screen)
	}
}
