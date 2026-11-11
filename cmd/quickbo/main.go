// Command quickbo is a desktop TVBox-style client with a native GPU UI.
package main

import (
	"flag"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/harrylyx/QuickBo/internal/app"
)

func main() {
	ref := flag.String("config", "", "配置 URL 或本地 JSON 路径")
	flag.Parse()
	if *ref == "" && flag.NArg() > 0 {
		*ref = flag.Arg(0)
	}
	application := app.New()
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     "QuickBo",
			Width:     1440,
			Height:    900,
			MinWidth:  960,
			MinHeight: 640,
			// No sidebar Vibrancy: the system material follows the system's
			// appearance, not the app's own light/dark switch, so a dark
			// app on a light system got a light sidebar.
			BackgroundColor: "light-dark(#ffffff, #0f0f12)",
			Content:         ui.View(application.View),
		})
		application.Attach(win)
		application.RestoreSettings()
		if *ref != "" {
			application.AddSubscription(*ref)
		} else {
			application.LoadSaved()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
