package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type notes struct {
	unsaved        bool
	confirmClosing bool
	allowClose     func()
}

func (state *notes) page(c ui.C) {
	// The bar is as tall as the title bar and leaves room for the window buttons
	// on whichever side the platform puts them.
	leading, trailing := app.TitlebarInsets()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(28).PadRight(trailing).PadLeft(leading).Items(ui.Center).MovesWindow().Children(func() {
			ui.Text(c, "Notes").Muted()
		})
		ui.Column(c).Grow(1).Pad(24).Items(ui.Start).Children(func() {
			if ui.Button(c, "Settings").Outline().Clicked() {
				// Windows are opened from the start callback or from another
				// window's callbacks, which run on the native event-loop thread.
				_, _ = ui.OpenWindow(ui.Application(c), app.Config{Title: "Settings", Width: 420, Height: 320},
					ui.PageConfig{View: func(c ui.C) { ui.Text(c, "Settings").FontSize(20).Bold() }})
			}
		})
		ui.ModalWith(c, &state.confirmClosing, "Discard changes?", ui.ModalSpec{
			Description: "The note has not been saved.", Width: 400, NoClose: true,
		}, nil, func() {
			if ui.Button(c, "Keep editing").Outline().Clicked() {
				state.confirmClosing = false
			}
			if ui.Button(c, "Discard").Destructive().Clicked() {
				state.confirmClosing = false
				state.unsaved = false
				state.allowClose()
			}
		})
	})
}

func main() {
	theme := gfx.LightTheme()
	state := &notes{unsaved: true}

	err := app.RunApplication(func(application *app.Application) error {
		_, err := ui.OpenWindow(application, app.Config{
			Title: "Notes", Width: 720, Height: 480,
			// Content runs full height under a transparent title bar; the window
			// moves from the row marked MovesWindow. Keep your own content clear of
			// the window buttons.
			WindowMode: app.WindowCustomTitlebar,
			// The close button asks. Call allow now, later, or never to keep the window.
			OnCloseRequest: func(allow func()) {
				if !state.unsaved {
					allow()
					return
				}
				state.confirmClosing, state.allowClose = true, allow
			},
		}, ui.PageConfig{View: state.page, Theme: &theme})
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}
