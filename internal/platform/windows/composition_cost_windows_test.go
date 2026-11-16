package windows

import (
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func BenchmarkWindowsIMEAcknowledgment(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "legacy"
		if enabled {
			name = "tentative"
		}
		b.Run(name, func(b *testing.B) {
			state := &windowState{textEnabled: true, textState: platform.TextState{TentativeComposition: enabled}}
			state.textInput.Terminal = true
			state.textInput.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
				return state.textState, nil
			}
			event := platform.TextEvent{Kind: platform.TextSetMarked, Text: "x"}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				state.handleTextEvent(event)
			}
			b.ReportMetric(float64(unsafe.Sizeof(windowState{})), "window-B")
		})
	}
}

var compositionWindowStateSink *windowState

func BenchmarkWindowsIMEStateAllocation(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		compositionWindowStateSink = new(windowState)
	}
	b.ReportMetric(float64(unsafe.Sizeof(windowState{})), "window-B")
	compositionWindowStateSink = nil
}
