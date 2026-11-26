//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// Measure the public API and event dispatch with an in-memory host. Window
// creation includes app bookkeeping, not a native graphics driver or compositor.
func BenchmarkStatusPreview(b *testing.B) {
	for _, path := range []string{"configure", "duplicate-enter", "open-close", "refresh-menu"} {
		b.Run(path, func(b *testing.B) {
			host := &previewTestHost{}
			application := &Application{host: host}
			item, err := application.AddStatusItem(StatusItemConfig{Icon: StatusIcon{Size: 1, RGBA: make([]byte, 4)}})
			if err != nil {
				b.Fatal(err)
			}
			build := func() StatusPreview { return StatusPreview{Config: Config{Paint: func(*gfx.Canvas, gfx.Size) {}}} }
			item.SetHoverPreview(build)
			if path == "duplicate-enter" {
				host.hover(StatusAnchor{}, true)
				defer host.windows[0].Close()
			}
			b.ReportAllocs()
			for b.Loop() {
				switch path {
				case "configure":
					item.SetHoverPreview(build)
				case "duplicate-enter":
					host.hover(StatusAnchor{}, true)
				case "open-close":
					host.hover(StatusAnchor{}, true)
					host.windows[0].Close()
					host.windows = host.windows[:0]
					host.hover(StatusAnchor{}, false)
				case "refresh-menu":
					item.RefreshMenu()
				}
			}
		})
	}
}
