package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/config"
)

// A warehouse subscription (storeHouse / urls) is already parsed by QuickBo.
// What was missing: when every child address fails, the links disappeared
// behind a blank page and a generic error. They must stay visible so each
// address can be opened on its own.
func indexFailApp(t *testing.T) (*App, chan struct{}) {
	t.Helper()
	a := New()
	a.settingsPath = ""
	a.fetchConfig = func(_ context.Context, ref string) (*config.Config, error) {
		if ref == "index" {
			return &config.Config{StoreHouse: []config.NamedURL{
				{Name: "仓1", URL: "bad1"},
				{Name: "仓2", URL: "bad2"},
			}}, nil
		}
		return nil, errors.New("打不开 " + ref)
	}
	done := make(chan struct{})
	a.afterLoad = func() { close(done) }
	return a, done
}

func waitLoad(t *testing.T, done chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("load did not finish")
	}
}

func assertIndexLinks(t *testing.T, a *App) {
	t.Helper()
	if a.screen != screenIndex || a.loading {
		t.Fatalf("screen=%s loading=%v", a.screen, a.loading)
	}
	if a.catalog == nil || len(a.catalog.Index) != 2 {
		t.Fatalf("warehouse links lost: %+v", a.catalog)
	}
	if a.loadErr == "" {
		t.Fatal("no explanation for the failed children")
	}
	tt := ui.NewTester(a.View, 960, 640)
	for _, want := range []string{"选一个地址继续加载", "bad1", "bad2"} {
		if !tt.HasText(want) {
			t.Fatalf("index page is missing %q: %q", want, tt.Texts())
		}
	}
}

func TestLoadFailedIndexKeepsLinks(t *testing.T) {
	a, done := indexFailApp(t)
	a.Load("index")
	waitLoad(t, done)
	assertIndexLinks(t, a)
}

func TestLoadSubsFailedIndexKeepsLinks(t *testing.T) {
	a, done := indexFailApp(t)
	a.LoadSubs([]string{"index"})
	waitLoad(t, done)
	assertIndexLinks(t, a)
}

func TestSingleAddressLoadShowsItsOwnError(t *testing.T) {
	a, done := indexFailApp(t)
	a.Load("index")
	waitLoad(t, done)
	// viewIndex opens one address with Load; that path reports the address's
	// own error instead of the generic "no sources" message.
	once := make(chan struct{})
	a.afterLoad = func() { close(once) }
	a.Load("bad1")
	select {
	case <-once:
	case <-time.After(3 * time.Second):
		t.Fatal("single address load did not finish")
	}
	if a.loadErr != "打不开 bad1" {
		t.Fatalf("the address's own error is not shown: %q", a.loadErr)
	}
	if a.screen != screenIndex || a.catalog == nil || len(a.catalog.Index) != 2 {
		t.Fatal("the warehouse list should stay available after one retry")
	}
}
