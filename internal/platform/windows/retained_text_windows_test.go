package windows

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

func terminalScrollLabels(count, first int) []platform.TextLabel {
	const columns = 98
	labels := make([]platform.TextLabel, count)
	for index := range labels {
		value := fmt.Sprintf("row %04d ", first+index)
		value += strings.Repeat("the quick brown fox 0123456789 ", (columns-len(value))/30+1)
		value = value[:columns]
		runs := []ui.TextRun{
			{Column: 0, Bytes: 33, Cells: 33, Color: platformTextColor(0.9, 0.4, 0.4)},
			{Column: 33, Bytes: 33, Cells: 33, Color: platformTextColor(0.4, 0.9, 0.4)},
			{Column: 66, Bytes: columns - 66, Cells: columns - 66, Color: platformTextColor(0.4, 0.5, 1)},
		}
		labels[index] = platform.TextLabel{
			Value: value, Y: float32(index) * 20, Width: columns * 8, Height: 20,
			FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			Mono: true, CellWidth: 8, Runs: runs,
			Identity: ui.TextIdentity{Owner: 1, Item: uint64(first + index)},
		}
	}
	return labels
}

func platformTextColor(red, green, blue float32) ui.Color {
	return ui.Color{Red: red, Green: green, Blue: blue, Alpha: 1}
}

func drawTerminalRows(t *testing.T, offscreen *Offscreen, labels []platform.TextLabel) {
	t.Helper()
	if err := offscreen.Set(nil, labels); err != nil {
		t.Fatal(err)
	}
	width, height := offscreen.size()
	if err := offscreen.renderer.draw(width, height, float32(offscreen.scale)); err != nil {
		t.Fatal(err)
	}
}

// One line of scrolling replaces one row. Every row that stays on screen must
// keep its rasterized texture; only the row entering the top or bottom may be
// rasterized again.
func TestScrollingTerminalRowsReusesTheirTextures(t *testing.T) {
	offscreen, err := NewOffscreen(800, 600, 1, platform.Color{Alpha: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer offscreen.Close()
	const rows = 28
	drawTerminalRows(t, offscreen, terminalScrollLabels(rows, 0))
	baseline := offscreen.renderer.labelRebuilds
	if baseline != rows {
		t.Fatalf("first frame rasterized %d rows, want %d", baseline, rows)
	}
	for frame := 1; frame <= 4; frame++ {
		drawTerminalRows(t, offscreen, terminalScrollLabels(rows, frame))
		if got := offscreen.renderer.labelRebuilds - baseline; got != uint64(frame) {
			t.Fatalf("after %d scrolled frames, rasterizations = %d, want %d", frame, got, frame)
		}
	}
}

func benchmarkScrollingTerminalRows(b *testing.B, identities bool) {
	const scale = float64(2)
	offscreen, err := NewOffscreen(800, 600, scale, platform.Color{Alpha: 1})
	if err != nil {
		b.Fatal(err)
	}
	defer offscreen.Close()
	const rows = 28
	if err := offscreen.Set(nil, terminalScrollLabels(rows, 0)); err != nil {
		b.Fatal(err)
	}
	width, height := offscreen.size()
	if err := offscreen.renderer.draw(width, height, float32(scale)); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		labels := terminalScrollLabels(rows, index+1)
		if !identities {
			for label := range labels {
				labels[label].Identity = ui.TextIdentity{}
			}
		}
		if err := offscreen.Set(nil, labels); err != nil {
			b.Fatal(err)
		}
		if err := offscreen.renderer.draw(width, height, float32(scale)); err != nil {
			b.Fatal(err)
		}
	}
}

// Scrolling with stable row identities rasterizes only the entering row. The
// positional form is the old behavior: every row has new content at the same
// list index and is rasterized again.
func BenchmarkScrollingTerminalRows(b *testing.B) {
	b.Run("identity", func(b *testing.B) { benchmarkScrollingTerminalRows(b, true) })
	b.Run("positional", func(b *testing.B) { benchmarkScrollingTerminalRows(b, false) })
}
