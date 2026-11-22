package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type model struct {
	samples []float32 // 0 to 1
}

// page declares a chart's frame and the button that feeds it. The page lays the
// chart's element out; what is drawn inside it is the chart's own.
func (m *model) page(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Fill().Pad(24).Gap(16).Children(func() {
		ui.Text(c, "Samples").FontSize(20).Bold()
		ui.Row(c).FillWidth().Height(120).Radius(8).Background(theme.Card).Border(1, theme.Border).
			Hit(ui.ID(c), "Samples chart").
			Draw(m.bars).
			DrawOver(func(d *ui.DrawContext) {
				if d.State.Hovered {
					d.Canvas.Fill(d.Rect, d.Fade(theme.Foreground.WithAlpha(0.06)))
				}
			})
		if ui.Button(c, "Add a sample").Clicked() {
			m.samples = append(m.samples, float32(len(m.samples)%7+1)/8)
		}
	})
}

// bars paints one bar for each sample in d.Rect, the room the page gave the element.
// It only paints: it may run more than once a frame and declares no elements.
func (m *model) bars(d *ui.DrawContext) {
	const gap = 6
	if len(m.samples) == 0 {
		return
	}
	width := (d.Rect.Width - gap*float32(len(m.samples)+1)) / float32(len(m.samples))
	for i, value := range m.samples {
		height := d.Rect.Height * 0.8 * value
		d.Canvas.Fill(ui.Rect{
			X:     d.Rect.X + gap + float32(i)*(width+gap),
			Y:     d.Rect.Y + d.Rect.Height - height - 8,
			Width: width, Height: height,
		}, d.Fade(d.Theme.Primary))
	}
}

func main() {
	m := &model{samples: []float32{0.3, 0.6, 0.45}}
	if err := ui.Run(app.Config{Title: "Samples", Width: 480, Height: 300}, ui.PageConfig{View: m.page}); err != nil {
		log.Fatal(err)
	}
}
