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
			Width:     1180,
			Height:    760,
			MinWidth:  880,
			MinHeight: 560,
			Content:   ui.View(application.View),
		})
		application.Attach(win)
		if *ref != "" {
			application.Load(*ref)
		} else {
			application.LoadSaved()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
