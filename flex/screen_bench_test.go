package flex_test

import (
	"testing"

	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// The fleet page written with Frame and explicit identities, the baseline the
// Screen version is measured against.
var (
	benchTree   = ui.Tree{}
	benchPage   = benchTree.Feature("fleet")
	benchFilter = benchTree.Sub(benchPage, benchTree.Name("filter"))
	benchList   = benchTree.ID(benchPage, "list")
	benchScan   = benchTree.ID(benchPage, "scan")
	benchRows   = benchTree.Sub(benchPage, benchTree.Name("rows"))
)

func (m *fleetModel) frame(f *flex.Frame) {
	f.Column().Fill().Pad(22).Gap(14).Children(func() {
		f.Text("Fleet check").FontSize(26).Bold()
		if i, ok := f.Choice(benchFilter, m.labels, m.filter).ChosenIndex(); ok {
			m.filter = i
		}
		f.ScrollArea(benchList).Grow(1).Gap(8).Children(func() {
			for i := range m.hosts {
				h := &m.hosts[i]
				f.Row().FillWidth().Pad(12).Gap(12).Children(func() {
					f.Text(h.name).Bold()
					f.Text(h.addr).Muted()
					if f.Checkbox(benchRows.At(i), h.handled).Clicked() {
						h.handled = !h.handled
					}
				})
			}
		})
		f.Row().Gap(10).Children(func() {
			f.Text(m.summary).Muted()
			if f.Button(benchScan, "Scan again").Clicked() {
				m.scans++
			}
		})
	})
}

func benchCanvas() *ui.Canvas {
	canvas := &ui.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * size / 2 })
	return canvas
}

func BenchmarkFrameFleet(b *testing.B) {
	canvas := benchCanvas()
	canvas.SetTree(&benchTree)
	m := newFleetModel()
	var view flex.View
	theme := ui.LightTheme()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		canvas.BeginFrame()
		view.Draw(canvas, theme, ui.Rect{Width: 600, Height: 400}, m.frame)
		canvas.EndFrame()
	}
}

func BenchmarkScreenFleet(b *testing.B) {
	screen, canvas := newTestScreen()
	m := newFleetModel()
	b.ReportAllocs()
	for b.Loop() {
		drawScreen(screen, canvas, m.page)
	}
}

func BenchmarkScreenBound1000(b *testing.B) {
	screen, canvas := newTestScreen()
	values := make([]bool, 1000)
	body := func(s flex.Screen) {
		s.Column().Children(func() {
			for i := range values {
				s.Checkbox(&values[i], "")
			}
		})
	}
	b.ReportAllocs()
	for b.Loop() {
		drawScreen(screen, canvas, body)
	}
}

func BenchmarkScreenKeyed1000(b *testing.B) {
	screen, canvas := newTestScreen()
	body := func(s flex.Screen) {
		s.Column().Children(func() {
			for i := range 1000 {
				s.KeyIndex(i).Button("Open")
			}
		})
	}
	b.ReportAllocs()
	for b.Loop() {
		drawScreen(screen, canvas, body)
	}
}

func BenchmarkFrameKeyed1000(b *testing.B) {
	canvas := benchCanvas()
	canvas.SetTree(&benchTree)
	var view flex.View
	theme := ui.LightTheme()
	body := func(f *flex.Frame) {
		f.Column().Children(func() {
			for i := range 1000 {
				f.Button(benchRows.At(i), "Open")
			}
		})
	}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		canvas.BeginFrame()
		view.Draw(canvas, theme, ui.Rect{Width: 600, Height: 400}, body)
		canvas.EndFrame()
	}
}

func BenchmarkScreenNativeView(b *testing.B) {
	screen, canvas := newTestScreen()
	open := true
	body := func(s flex.Screen) {
		s.Column().Fill().Children(func() {
			anchor := s.Button("Menu")
			s.Slot().Grow(1).NativeView(1)
			s.Popover(anchor, &open, func(p flex.Screen) { p.Text("over the page") })
		})
	}
	b.ReportAllocs()
	for b.Loop() {
		drawScreen(screen, canvas, body)
	}
}
