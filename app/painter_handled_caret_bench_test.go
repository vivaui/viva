package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func BenchmarkPointerCaretOwnership(b *testing.B) {
	for _, events := range []bool{false, true} {
		provider := "editor"
		if events {
			provider = "events"
		}
		b.Run(provider, func(b *testing.B) {
			for _, handled := range []bool{false, true} {
				name := "default"
				if handled {
					name = "handled"
				}
				b.Run(name, func(b *testing.B) {
					fixture := newHandledCaretFixture(b, events)
					if handled {
						fixture.handled = gfx.PointerDown
					}
					gesture := [...]platform.PointerEvent{
						{Kind: platform.PointerDown, X: 10, Y: 10, Button: gfx.PointerButtonPrimary, ClickCount: 1},
						{Kind: platform.PointerMove, X: 30, Y: 10, Button: gfx.PointerButtonPrimary, ClickCount: 1},
						{Kind: platform.PointerUp, X: 30, Y: 10, Button: gfx.PointerButtonPrimary, ClickCount: 1},
					}
					b.ReportAllocs()
					for b.Loop() {
						for _, event := range gesture {
							if _, err := fixture.painter.pointerEvent(event); err != nil {
								b.Fatal(err)
							}
						}
					}
					b.ReportMetric(float64(fixture.placement.calls)/float64(b.N), "placements/op")
					// The baseline overwrites handled selections. Permanent tests
					// assert the repair; the paired workload checks common state.
					if fixture.editor.String() != "ab cd\nef" || fixture.editor.CanUndo() {
						b.Fatal("pointer gesture changed text or history")
					}
				})
			}
		})
	}
}
