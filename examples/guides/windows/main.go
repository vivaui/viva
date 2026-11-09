package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree    = gfx.Tree{}
	page    = tree.Feature("notes")
	confirm = tree.Sub(page, tree.Name("confirm"))

	settingsButton = tree.ID(page, "settings")
	confirmScope   = tree.ID(confirm, "scope")
	keep           = tree.ID(confirm, "keep")
	discard        = tree.ID(confirm, "discard")
)

type notes struct {
	unsaved        bool
	confirmClosing bool
	allowClose     func()
	view           ui.View
}

func main() {
	theme := gfx.LightTheme()
	state := &notes{unsaved: true}

	err := app.RunApplication(func(application *app.Application) error {
		var main, settings gfx.Canvas

		config := app.Config{
			Tree: &tree, Title: "Notes", Width: 720, Height: 480,
			// Content runs full height under a transparent title bar; the window
			// moves from the row marked MovesWindow. Keep your own content clear of
			// the window buttons.
			WindowMode: app.WindowCustomTitlebar,
		}
		leading, trailing := app.TitlebarInsets()
		config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
			bounds := gfx.Rect{Width: size.Width, Height: size.Height}
			canvas.Fill(bounds, theme.Background)

			state.view.Draw(canvas, theme, bounds, func(f *ui.Frame) {
				f.Column().Fill().Children(func() {
					// The bar is as tall as the title bar and leaves room for the
					// window buttons on whichever side the platform puts them.
					f.Row().Height(28).PadRight(trailing).PadLeft(leading).Items(ui.Center).MovesWindow().Children(func() {
						f.Text("Notes").Muted()
					})
					f.Column().Grow(1).Pad(24).Items(ui.Start).Children(func() {
						if f.Button(settingsButton, "Settings").Outline().Clicked() {
							// Windows are opened from the start callback or from another
							// window's callbacks, which run on the native event-loop thread.
							_, _ = application.OpenWindow(app.Config{Title: "Settings", Width: 420, Height: 320}, &settings)
						}
					})
					if state.confirmClosing {
						d := f.Dialog(ui.DialogSpec{
							Title: "Discard changes?", Description: "The note has not been saved.", Width: 400,
							Scope: confirmScope, DismissID: keep, NoClose: true,
						})
						d.Footer(func() {
							f.Button(keep, "Keep editing").Outline()
							if f.Button(discard, "Discard").Destructive().Clicked() {
								state.confirmClosing = false
								state.unsaved = false
								state.allowClose()
							}
						})
						if d.Dismissed() {
							state.confirmClosing = false
						}
					}
				})
			})
		}
		// The close button asks. Call allow now, later, or never to keep the window.
		config.OnCloseRequest = func(allow func()) {
			if !state.unsaved {
				allow()
				return
			}
			state.confirmClosing, state.allowClose = true, allow
		}

		_, err := application.OpenWindow(config, &main)
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}
