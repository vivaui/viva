//go:build windows || linux || darwin

// Command titlebar-demo opens one custom-titlebar window, for looking at the
// window controls by hand on each platform: Viva draws minimize, maximize and
// close at the end of the strip on Windows and Linux, the system's traffic
// lights are at its start on macOS. Drag the strip, double-click it, drag the
// edges, press the buttons; the application's own button counts clicks, which
// must not include presses on the window buttons.
package main

import (
	"fmt"
	"os"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

const stripHeight = 36

var (
	clicks int
	tree   gfx.Tree
	count  = tree.Feature("demo").ID(tree.Name("count"))
)

func main() {
	ink := gfx.Color{Red: 0.85, Green: 0.88, Blue: 0.95, Alpha: 1}
	config := app.Config{
		Title: "titlebar demo", Width: 700, Height: 400,
		WindowMode: app.WindowCustomTitlebar, TitlebarHeight: stripHeight,
		ClearColor: app.Color{Red: 0.09, Green: 0.1, Blue: 0.13, Alpha: 1},
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			leading, _ := app.TitlebarInsets()
			canvas.MoveRegion(gfx.Rect{Width: size.Width, Height: stripHeight})
			canvas.Fill(gfx.Rect{Width: size.Width, Height: stripHeight}, gfx.Color{Red: 0.14, Green: 0.16, Blue: 0.22, Alpha: 1})
			canvas.Label("drag the strip; double-click maximizes", gfx.Rect{X: leading + 14, Y: 9, Width: 400, Height: 20}, gfx.TextStyle{Size: 13, Color: ink})
			canvas.Fill(gfx.Rect{X: 20, Y: 80, Width: 160, Height: 40}, gfx.Color{Red: 0.2, Green: 0.4, Blue: 0.8, Alpha: 1})
			canvas.HitWithLabel(count, gfx.Rect{X: 20, Y: 80, Width: 160, Height: 40}, "Count")
			if canvas.Clicked(count) {
				clicks++
			}
			canvas.Label("Count", gfx.Rect{X: 62, Y: 91, Width: 80, Height: 20}, gfx.TextStyle{Size: 14, Color: ink})
			canvas.Label(fmt.Sprintf("clicks: %d", clicks), gfx.Rect{X: 20, Y: 140, Width: 300, Height: 20}, gfx.TextStyle{Size: 14, Color: ink})
			canvas.Label(fmt.Sprintf("size: %.0f x %.0f", size.Width, size.Height), gfx.Rect{X: 20, Y: 170, Width: 300, Height: 20}, gfx.TextStyle{Size: 14, Color: ink})
		},
		Tree: &tree,
		OnCloseRequest: func(allow func()) {
			fmt.Println("close requested")
			allow()
		},
	}
	if err := app.RunCanvas(config, &gfx.Canvas{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
