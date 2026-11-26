package main

import (
	"fmt"
	"log"
	"runtime"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// AppKit must start on the process's main thread.
func init() { runtime.LockOSThread() }

func main() {
	if err := app.RunApplication(run); err != nil {
		log.Fatal(err)
	}
}

func run(application *app.Application) error {
	var main, preview *app.Window
	var tray *app.StatusItem
	var wake, previewWake app.Invalidator
	unread := 1
	show := func() {
		if preview != nil {
			preview.Close()
		}
		main.Show()
	}
	build := func() app.StatusPreview {
		page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
			ui.Column(c).Fill().Pad(12).Gap(8).Children(func() {
				ui.Text(c, fmt.Sprintf("%d unread messages", unread))
				if ui.Button(c, "Open application").Clicked() {
					c.Canvas().AfterPaint(show)
				}
			})
		}})
		config := ui.Configure(page, app.Config{
			Title: "Unread messages", Width: 240, Height: 96, Invalidator: &previewWake,
			OnClosed: func() { preview = nil },
		})
		return app.StatusPreview{Config: config, Opened: func(window *app.Window, err error) {
			if err != nil {
				page.Close()
				log.Printf("open status preview: %v", err)
				return
			}
			preview = window
		}}
	}
	var err error
	main, err = ui.OpenWindow(application, app.Config{Title: "Status preview", Width: 400, Height: 180, Invalidator: &wake}, ui.PageConfig{
		View: func(c ui.C) {
			ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
				ui.Text(c, fmt.Sprintf("%d unread messages", unread))
				if ui.Button(c, "Add a message").Clicked() {
					unread++
					previewWake.Invalidate()
					if tray != nil {
						// Linux hosts cache their menu. Notify them when its text changes.
						tray.RefreshMenu()
					}
				}
			})
		},
	})
	if err != nil {
		return err
	}
	tray, err = application.AddStatusItem(app.StatusItemConfig{
		Tooltip: "Status preview", Icon: app.StatusIcon{Size: 1, RGBA: []byte{70, 130, 210, 255}, Template: true},
		OnClick: func(click app.StatusClick) {
			if click.PanelDismissed {
				return
			}
			// Click remains available on hosts without paired hover events.
			spec := build()
			spec.Config.WindowMode, spec.Config.Anchor = app.WindowPanel, click.Anchor
			window, err := application.OpenWindow(spec.Config, nil)
			spec.Opened(window, err)
		},
		Menu: func() []app.MenuItem {
			return []app.MenuItem{
				{Title: fmt.Sprintf("%d unread messages", unread), ID: "unread", OnSelect: show},
				{Title: "Quit", OnSelect: application.Quit},
			}
		},
	})
	if err != nil {
		log.Printf("status area unavailable; main window remains usable: %v", err)
		return nil
	}
	tray.SetHoverPreview(build)
	return nil
}
