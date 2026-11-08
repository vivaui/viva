//go:build js && wasm

package goldens

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

// Capture paints the same fixed component snapshot on the web backend.
// It must stay byte-identical to the darwin twin.
func Capture() DrawList {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	paintSnapshot(&canvas, theme)
	var list DrawList
	for _, rectangle := range canvas.Rectangles() {
		list.Commands = append(list.Commands, Command{
			Kind: 1,
			X:    rectangle.Bounds.X, Y: rectangle.Bounds.Y,
			Width: rectangle.Bounds.Width, Height: rectangle.Bounds.Height,
			Radius: rectangle.Radius,
			Red:    rectangle.Color.Red, Green: rectangle.Color.Green,
			Blue: rectangle.Color.Blue, Alpha: rectangle.Color.Alpha,
		})
	}
	for _, shadow := range canvas.Shadows() {
		list.Commands = append(list.Commands, Command{
			Kind: 3,
			X:    shadow.Bounds.X, Y: shadow.Bounds.Y,
			Width: shadow.Bounds.Width, Height: shadow.Bounds.Height,
			Radius: shadow.Radius,
			Red:    shadow.Color.Red, Green: shadow.Color.Green,
			Blue: shadow.Color.Blue, Alpha: shadow.Color.Alpha,
			OffsetX: shadow.Style.OffsetX, OffsetY: shadow.Style.OffsetY,
			Blur: shadow.Style.Blur, Spread: shadow.Style.Spread,
		})
	}
	for _, text := range canvas.Texts() {
		list.Commands = append(list.Commands, Command{
			Kind: 2,
			X:    text.Bounds.X, Y: text.Bounds.Y,
			Width: text.Bounds.Width, Height: text.Bounds.Height,
			Red: text.Style.Color.Red, Green: text.Style.Color.Green,
			Blue: text.Style.Color.Blue, Alpha: text.Style.Color.Alpha,
			Value: text.Value,
		})
	}
	return list
}

func paintSnapshot(canvas *ui.Canvas, theme ui.Theme) {
	canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 400, Height: 300}, theme.Background)
	row := ui.BeginRow(ui.Rect{X: 16, Y: 16, Width: 368, Height: 36}, 8)
	components.BeginButton(canvas, theme, ui.Item(7), row.Next(120)).Default("Save")
	components.BeginButton(canvas, theme, ui.Item(8), row.Next(120)).Secondary("Cancel")
	components.PaintBadge(canvas, theme, row.NextFlex(), "Live", components.BadgeDefault)
	components.BeginSwitch(canvas, theme, ui.Item(9), ui.Rect{X: 16, Y: 64}, true).Paint()
	components.BeginCheckbox(canvas, theme, ui.Item(10), ui.Rect{X: 16, Y: 100, Width: 22, Height: 22}, false).Paint()
	components.PaintSeparator(canvas, theme, ui.Rect{X: 16, Y: 140, Width: 368, Height: 9}, components.SeparatorHorizontal)
	components.MetricCard(canvas, theme, ui.Rect{X: 16, Y: 164, Width: 200, Height: 120}, "Revenue", "$1,234", "+5%", true)
}
