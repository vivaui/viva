// A native watermark stays above embedded views without receiving input.
package main

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"log"
)

func main() {
	config := app.Config{Title: "Native watermark", Width: 800, Height: 500}
	if app.Supports(app.CapabilityWindowWatermark) {
		config.Watermark = app.Watermark{Text: "DRAFT", Angle: -25, FontWeight: gfx.WeightLight, Insets: gfx.All(24)}
	}
	if err := ui.Run(config, ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Fill().Pad(24).Children(func() { ui.Button(c, "The watermark does not intercept input") })
	}}); err != nil {
		log.Fatal(err)
	}
}
