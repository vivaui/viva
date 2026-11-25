//go:build windows || linux || darwin

// Command transparent-demo opens one window composited over the desktop, for
// looking at Config.Transparent by hand on each platform. The left band is
// painted at half alpha and the right one at full alpha, and the strip under
// them is left unpainted; the top strip is a custom titlebar. The strip's title
// says whether the capability is there and whether the desktop really shows
// through, as OnTransparentChange reported when the window opened.
package main

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func main() {
	ink := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	supported := app.Supports(app.CapabilityTransparentWindow)
	var active atomic.Bool
	config := app.Config{
		Title: "transparent demo", Width: 640, Height: 360,
		WindowMode:          app.WindowCustomTitlebar,
		TitlebarHeight:      36,
		Transparent:         true,
		OnTransparentChange: func(on bool) { active.Store(on); fmt.Println("transparent active:", on) },
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.MoveRegion(gfx.Rect{Width: size.Width, Height: 36})
			canvas.Fill(gfx.Rect{Width: size.Width, Height: 36}, gfx.Color{Red: 0.14, Green: 0.16, Blue: 0.22, Alpha: 1})
			canvas.Fill(gfx.Rect{Y: 36, Width: size.Width / 2, Height: size.Height - 36 - 120}, gfx.Color{Red: 0.1, Green: 0.2, Blue: 0.6, Alpha: 0.5})
			canvas.Fill(gfx.Rect{X: size.Width / 2, Y: 36, Width: size.Width / 2, Height: size.Height - 36 - 120}, gfx.Color{Red: 0.1, Green: 0.2, Blue: 0.6, Alpha: 1})
			canvas.Label(fmt.Sprintf("supported: %v  active: %v", supported, active.Load()), gfx.Rect{X: 14, Y: 9, Width: 400, Height: 20}, gfx.TextStyle{Size: 13, Color: ink})
			canvas.Label("half alpha band", gfx.Rect{X: 20, Y: 80, Width: 280, Height: 24}, gfx.TextStyle{Size: 16, Color: ink})
			canvas.Label("opaque band", gfx.Rect{X: size.Width/2 + 20, Y: 80, Width: 280, Height: 24}, gfx.TextStyle{Size: 16, Color: ink})
		},
	}
	if err := app.RunCanvas(config, &gfx.Canvas{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
