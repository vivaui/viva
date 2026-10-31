// Run with: go run ./examples/windows-materials -material mica-alt
package main

import (
	"flag"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func main() {
	choice := flag.String("material", "mica", "mica, mica-alt, or acrylic")
	flag.Parse()
	kinds := map[string]app.BackdropKind{"mica": app.BackdropMica, "mica-alt": app.BackdropMicaAlt, "acrylic": app.BackdropAcrylic}
	kind, ok := kinds[*choice]
	if !ok {
		log.Fatalf("unknown material %q", *choice)
	}
	active := false
	theme := ui.LightTheme()
	var canvas ui.Canvas
	err := app.RunCanvas(app.Config{
		Title: "Viva " + *choice, Width: 640, Height: 400,
		ClearColor: app.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		WindowMaterials: app.WindowMaterials{
			WindowBackdrop:   func() app.BackdropStyle { return app.BackdropStyle{Kind: kind} },
			OnBackdropChange: func(enabled bool) { active = enabled },
		},
		Paint: func(canvas *ui.Canvas, size ui.Size) {
			background := theme.Background
			label := "Material unavailable or disabled: solid fallback"
			if active {
				background.Alpha = .05
				label = "Requested material: " + *choice
			}
			canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, background)
			canvas.Label(label, ui.Rect{X: 24, Y: 24, Width: size.Width - 48, Height: 40}, ui.TextStyle{Size: 16, Color: theme.Foreground})
		},
	}, &canvas)
	if err != nil {
		log.Fatal(err)
	}
}
