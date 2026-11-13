package app

import (
	"context"
	"testing"

	"github.com/harrylyx/QuickBo/internal/model"
)

// A junk genre the user named in a rule keyword must be collected, otherwise
// the row has nothing to show.
func TestCollectSourceKeepsJunkCategoryNamedByRule(t *testing.T) {
	newSrc := func() *yieldSrc {
		return &yieldSrc{
			cats: []model.Category{
				{ID: "1", Name: "电影片"},
				{ID: "2", Name: "伦理片"},
				{ID: "3", Name: "短剧"},
			},
			pages: []model.Page{
				{Items: []model.Item{{Title: "无间道"}}},
				{Items: []model.Item{{Title: "伦理影片"}}},
				{Items: []model.Item{{Title: "重生之路"}}},
			},
		}
	}
	rules := []homeRule{
		{ID: "custom-1", Name: "伦理", Keywords: []string{"伦理"}},
		{ID: "custom-2", Name: "短剧", Keywords: []string{"短剧"}},
	}
	cats := collectSourceWith(context.Background(), newSrc(), rules)
	if len(cats) != 3 {
		t.Fatalf("cats %+v", cats)
	}
	if got := collectSourceWith(context.Background(), newSrc(), defaultHomeRules()); len(got) != 1 {
		t.Fatalf("default rules must still skip junk: %+v", got)
	}
}
