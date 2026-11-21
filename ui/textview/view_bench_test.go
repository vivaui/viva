package textview_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/textview"
)

// BenchmarkPaint paints a window of output whose length should not matter: only the
// rows on screen are declared and drawn.
func BenchmarkPaint(b *testing.B) {
	for _, n := range []int{100, 100_000} {
		b.Run(fmt.Sprintf("lines=%d", n), func(b *testing.B) {
			lines := make(textview.Strings, n)
			for i := range lines {
				lines[i] = fmt.Sprintf("%6d  at demo.Main.run(Main.java:%d) and some more output", i, i)
			}
			var view textview.View
			spec := textview.Spec{
				Label:      "Output",
				Source:     lines,
				Font:       gfx.TextStyle{Size: 13, Mono: true, Color: gfx.RGB(0x20, 0x20, 0x20)},
				LineHeight: 18,
				PadX:       4,
				Scroll:     true,
				Follow:     true,
			}
			page := ui.NewPage(ui.PageConfig{View: func(c ui.C) { view.Declare(c, spec).Fill() }})
			driver, err := app.NewDriver(page.Config(app.Config{Width: 800, Height: 600}), nil)
			if err != nil {
				b.Fatal(err)
			}
			defer driver.Close()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := driver.Paint(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
