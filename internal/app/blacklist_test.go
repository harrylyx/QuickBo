package app

import (
	"slices"
	"testing"

	"github.com/harrylyx/QuickBo/internal/source"
)

// Several keywords can be pasted at once; duplicates (any case) are skipped.
func TestAddBlacklistAcceptsSeveralKeywords(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.blacklist = []string{"广告"}
	a.addBlacklist("测试，备用 广告\nAD;ad、线路")
	if want := []string{"广告", "测试", "备用", "AD", "线路"}; !slices.Equal(a.blacklist, want) {
		t.Fatalf("blacklist %v, want %v", a.blacklist, want)
	}
	if a.blacklistDraft != "" {
		t.Fatal("the input must clear after adding")
	}
}

// Removing in bulk drops every selected keyword, case-insensitively, and
// clears the selection.
func TestRemoveBlacklistsInBulk(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.blacklist = []string{"广告", "Test", "备用", "线路"}
	a.blSel = map[string]bool{"test": true, "线路": true}
	a.removeBlacklists(a.selectedBlacklist())
	if want := []string{"广告", "备用"}; !slices.Equal(a.blacklist, want) {
		t.Fatalf("blacklist %v, want %v", a.blacklist, want)
	}
	if len(a.blSel) != 0 {
		t.Fatal("selection must clear after removing")
	}
}

// Each keyword shows the sources it hides; the search matches those too.
func TestBlacklistHitsAndFilter(t *testing.T) {
	a := New()
	a.allSites = []source.Source{
		&pagedSrc{key: "测试源一"}, &pagedSrc{key: "正式源"}, &pagedSrc{key: "测试源二"},
	}
	a.blacklist = []string{"测试", "广告"}
	if got := a.blacklistHits("测试"); !slices.Equal(got, []string{"测试源一", "测试源二"}) {
		t.Fatalf("hits %v", got)
	}
	a.blFilter = "源二"
	if got := a.filteredBlacklist(); !slices.Equal(got, []string{"测试"}) {
		t.Fatalf("search by hidden source: %v", got)
	}
}
