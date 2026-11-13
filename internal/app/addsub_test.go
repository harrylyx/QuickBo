package app

import (
	"context"
	"testing"
	"time"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/source"
)

// Navigating while a subscription loads bumps gen. The finished load must
// still apply, otherwise a new subscription never shows up on the home.
func TestAddSubscriptionSurvivesNavigationDuringLoad(t *testing.T) {
	a := New()
	a.settingsPath = ""
	done := make(chan struct{})
	a.afterLoad = func() { close(done) }
	a.fetchConfig = func(ctx context.Context, ref string) (*config.Config, error) {
		a.bump() // the user clicked elsewhere while the config downloaded
		return &config.Config{Sites: []config.Site{{
			Key: "site1", Name: "甲", Type: config.TypeJSON, API: "https://example.com/api.php",
		}}}, nil
	}
	a.probeSite = func(ctx context.Context, s source.Source) (model.Page, bool) {
		return model.Page{Items: []model.Item{{ID: "1", Title: "热播片", Pic: "http://x/a.jpg"}}}, true
	}

	a.AddSubscription("https://example.com/tvbox.json")

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("load never finished: the finished config was dropped")
	}
	if a.catalog == nil || len(a.catalog.Sites) != 1 {
		t.Fatalf("catalog %+v", a.catalog)
	}
	if len(a.home) == 0 {
		t.Fatalf("home is empty: %v", laneNames(a.home))
	}
}
